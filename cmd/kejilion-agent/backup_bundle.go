package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/kejilion/kejilion-panel/internal/backup"
	"github.com/kejilion/kejilion-panel/internal/hostbackup"
	"github.com/kejilion/kejilion-panel/internal/version"
	"golang.org/x/term"
)

type backupCommand func([]string, io.Reader, io.Writer) error
type backupFileRequest struct {
	Path     string   `json:"path"`
	Password string   `json:"password"`
	Modules  []string `json:"modules"`
}

func backupRPC(call backupCommand, args []string, input any, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := call(args, bytes.NewReader(body), &out); err != nil {
		return err
	}
	if output == nil {
		return nil
	}
	return backup.Decode(out.Bytes(), output)
}
func waitBackupCLI(call backupCommand, id string) (backup.Record, error) {
	deadline := time.NewTimer(2 * time.Hour)
	defer deadline.Stop()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		var r backup.Record
		if err := backupRPC(call, []string{"status", id}, nil, &r); err != nil {
			return r, err
		}
		switch r.Status {
		case "completed", "ready":
			return r, nil
		case "failed", "expired":
			return r, fmt.Errorf("备份任务未完成：%s；任务 %s", r.ErrorCode, id)
		}
		select {
		case <-deadline.C:
			return r, errors.New("备份任务等待超时，请查看任务记录")
		case <-tick.C:
		}
	}
}
func cliBackupWorkspace() (string, error) {
	stateDir, err := backup.CanonicalRoot(env("KEJILION_AGENT_STATE_DIR", "/var/lib/kejilion-panel"))
	if err != nil {
		return "", err
	}
	root := filepath.Join(stateDir, "backup-cli")
	if err := backup.PrivateDir(root); err != nil {
		return "", err
	}
	if err := hostbackup.SweepCLI(root, time.Now()); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	if len(entries) >= 50 {
		return "", errors.New("备份临时任务达到上限，请清理过期任务")
	}
	path := filepath.Join(root, backup.NewID())
	return path, os.Mkdir(path, 0700)
}
func exportBackupFile(call backupCommand, input backupFileRequest) error {
	modules, err := backup.Selection(input.Modules)
	if err != nil || slices.Contains(modules, "panel") {
		return backup.ErrInvalid
	}
	if err := backup.ValidatePassword(input.Password); err != nil {
		return err
	}
	if !filepath.IsAbs(input.Path) || !strings.HasSuffix(input.Path, ".kpb") {
		return errors.New("请指定绝对路径的 .kpb 文件")
	}
	if err := backup.NoLinkParents(input.Path); err != nil {
		return err
	}
	if _, err := os.Lstat(input.Path); !errors.Is(err, os.ErrNotExist) {
		return errors.New("导出文件已存在，请使用新文件名")
	}
	dir, err := cliBackupWorkspace()
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	var inventory hostbackup.Preview
	if err := backupRPC(call, []string{"inventory"}, nil, &inventory); err != nil {
		return err
	}
	var expected int64
	for _, m := range inventory.Modules {
		if slices.Contains(modules, m.ID) {
			expected += m.Bytes
		}
	}
	if err := backup.RequireSpace(dir, expected); err != nil {
		return err
	}
	if err := backup.RequireSpace(filepath.Dir(input.Path), expected+(32<<20)); err != nil {
		return err
	}
	var record backup.Record
	if err := backupRPC(call, []string{"export"}, hostbackup.Request{Action: "export", Modules: modules, Revision: inventory.Revision}, &record); err != nil {
		return err
	}
	if _, err := waitBackupCLI(call, record.ID); err != nil {
		return err
	}
	defer backupRPC(call, []string{"delete", record.ID}, nil, nil)
	sources := []backup.Source{}
	for _, module := range modules {
		path := filepath.Join(dir, module+".payload")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		err = call([]string{"download", record.ID, module}, nil, f)
		err = errors.Join(err, f.Sync(), f.Close())
		if err != nil {
			return err
		}
		sources = append(sources, backup.Source{Module: module, Path: path})
	}
	out, err := os.OpenFile(input.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = backup.Write(out, input.Password, version.Version, sources)
	err = errors.Join(err, out.Sync(), out.Close())
	if err != nil {
		_ = os.Remove(input.Path)
		return err
	}
	return backup.SyncDir(filepath.Dir(input.Path))
}
func inspectBackupFile(call backupCommand, input backupFileRequest) (backup.Record, error) {
	var record backup.Record
	if !filepath.IsAbs(input.Path) {
		return record, backup.ErrInvalid
	}
	dir, err := cliBackupWorkspace()
	if err != nil {
		return record, err
	}
	defer os.RemoveAll(dir)
	in, err := backup.OpenRegular(input.Path, backup.MaxEncryptedBytes)
	if err != nil {
		return record, err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return record, err
	}
	if err := backup.RequireSpace(dir, info.Size()*2); err != nil {
		return record, err
	}
	manifest, err := backup.Read(in, input.Password, dir)
	if err != nil {
		return record, err
	}
	modules := []string{}
	for _, p := range manifest.Parts {
		if p.Module != "panel" {
			modules = append(modules, p.Module)
		}
	}
	if len(input.Modules) > 0 {
		for _, m := range input.Modules {
			if !slices.Contains(modules, m) {
				return record, backup.ErrInvalid
			}
		}
		modules = input.Modules
	}
	modules, err = backup.Selection(modules)
	if err != nil {
		return record, errors.New("所选备份不含应用、网站或 Docker 数据；面板数据请在 KPanel 中恢复")
	}
	if err := backupRPC(call, []string{"import"}, hostbackup.Request{Action: "import", Modules: modules}, &record); err != nil {
		return record, err
	}
	uploaded := false
	defer func() {
		if !uploaded {
			_ = backupRPC(call, []string{"abort", record.ID}, nil, nil)
		}
	}()
	for _, module := range modules {
		f, err := backup.OpenRegular(filepath.Join(dir, module+".payload"), backup.MaxBytes)
		if err != nil {
			return record, err
		}
		err = call([]string{"upload", record.ID, module}, f, io.Discard)
		f.Close()
		if err != nil {
			return record, err
		}
	}
	if err := backupRPC(call, []string{"inspect", record.ID}, nil, &record); err != nil {
		return record, err
	}
	uploaded = true
	return waitBackupCLI(call, record.ID)
}
func runBackupFileCommand(args []string, input io.Reader, output io.Writer) error {
	if len(args) != 1 || os.Geteuid() != 0 {
		return backup.ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(input, 16385))
	var request backupFileRequest
	if err != nil || len(data) > 16384 || backup.Decode(data, &request) != nil {
		return backup.ErrInvalid
	}
	if args[0] == "file-export" {
		if err := exportBackupFile(runBackupCLI, request); err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(map[string]string{"path": request.Path, "status": "completed"})
	}
	record, err := inspectBackupFile(runBackupCLI, request)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(record)
}
func runBackupMenu(args []string, input io.Reader, output io.Writer) error {
	tty, ok := input.(*os.File)
	if !ok || !term.IsTerminal(int(tty.Fd())) || os.Geteuid() != 0 {
		return errors.New("交互备份需要 root 终端；自动化请使用 file-export/file-import 的 JSON stdin")
	}
	if len(args) > 1 {
		return backup.ErrInvalid
	}
	preset := ""
	if len(args) == 1 {
		preset = args[0]
		if preset != "apps" && preset != "web" && preset != "docker" {
			return backup.ErrInvalid
		}
	}
	reader := bufio.NewReader(tty)
	ask := func(prompt string) (string, error) {
		fmt.Fprint(output, prompt)
		value, err := reader.ReadString('\n')
		return strings.TrimSpace(value), err
	}
	password := func() (string, error) {
		fmt.Fprint(output, "备份密码（不回显，至少 10 字节）: ")
		value, err := term.ReadPassword(int(tty.Fd()))
		fmt.Fprintln(output)
		return string(value), err
	}
	fmt.Fprintln(output, "KPanel 通用备份与恢复 (.kpb)\n1. 导出备份  2. 导入检查与恢复  3. 任务记录  0. 返回")
	choice, err := ask("选择: ")
	if err != nil {
		return err
	}
	if choice == "0" {
		return nil
	}
	if choice == "3" {
		return runBackupCLI([]string{"jobs"}, nil, output)
	}
	if choice != "1" && choice != "2" {
		return backup.ErrInvalid
	}
	selection, err := ask("选择类别 apps web docker（空格分隔，回车使用 " + map[bool]string{true: preset, false: "全部类别"}[preset != ""] + "）: ")
	if err != nil {
		return err
	}
	modules := strings.Fields(selection)
	if len(modules) == 0 && preset != "" {
		modules = []string{preset}
	}
	if len(modules) == 0 && choice == "1" {
		modules = []string{"apps", "web", "docker"}
	}
	path, err := ask("备份文件绝对路径 (.kpb): ")
	if err != nil {
		return err
	}
	secret, err := password()
	if err != nil {
		return err
	}
	request := backupFileRequest{Path: path, Password: secret, Modules: modules}
	if choice == "1" {
		fmt.Fprintln(output, "请再次输入同一密码。")
		again, err := password()
		if err != nil {
			return err
		}
		if secret != again {
			return errors.New("两次密码不一致")
		}
		fmt.Fprintln(output, "备份会短暂停止相关容器并恢复原状态。请勿同时通过 SSH 修改数据。")
		yes, err := ask("开始备份？[y/N]: ")
		if err != nil {
			return err
		}
		if !strings.EqualFold(yes, "y") {
			return nil
		}
		if err := exportBackupFile(runBackupCLI, request); err != nil {
			return err
		}
		fmt.Fprintln(output, "备份已导出：", path)
		return nil
	}
	record, err := inspectBackupFile(runBackupCLI, request)
	if err != nil {
		return err
	}
	fmt.Fprintln(output, "检查通过，所含类别：", strings.Join(record.Modules, ", "), "；确认后覆盖所选数据。面板数据请在 KPanel 中恢复。")
	if len(record.Roots) > 0 {
		fmt.Fprintln(output, "将被替换的主机目录：")
		for _, root := range record.Roots {
			fmt.Fprintln(output, "  ", root.Path)
		}
	}
	// Refresh first so the review describes the revision being confirmed.
	if err := backupRPC(runBackupCLI, []string{"preview", record.ID}, nil, &record); err != nil {
		return err
	}
	var review hostbackup.ImportReview
	if err := backupRPC(runBackupCLI, []string{"review", record.ID}, nil, &review); err != nil {
		return err
	}
	foreign := review.Foreign(record.Modules)
	prompt := "确认恢复？[y/N]: "
	if foreign {
		printForeignReview(output, review)
		prompt = "已了解以上宿主机配置，确认恢复？[y/N]: "
	}
	yes, err := ask(prompt)
	if err != nil {
		return err
	}
	if !strings.EqualFold(yes, "y") {
		fmt.Fprintln(output, "尚未恢复，导入记录：", record.ID)
		return nil
	}
	var restored backup.Record
	if err := backupRPC(runBackupCLI, []string{"restore", record.ID}, hostbackup.Request{Modules: record.Modules, Revision: record.TargetRevision, ConfirmForeign: foreign}, &restored); err != nil {
		return err
	}
	if _, err := waitBackupCLI(runBackupCLI, restored.ID); err != nil {
		return err
	}
	fmt.Fprintln(output, "所选数据已恢复。")
	return nil
}

// printForeignReview lists what restoring a package this host did not export
// would run, so the single confirmation covers the host configuration.
func printForeignReview(output io.Writer, review hostbackup.ImportReview) {
	// Names, paths and options come from the package: drop terminal controls.
	say := func(parts ...any) {
		fmt.Fprint(output, strings.Map(func(r rune) rune {
			if r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) {
				return '?'
			}
			return r
		}, fmt.Sprintln(parts...)))
	}
	say("注意：该备份不是本机导出，恢复会按包内配置重建以下宿主机资源：")
	for _, m := range review.Modules {
		if m.Origin != hostbackup.OriginForeign {
			continue
		}
		say("类别", m.Module+"：")
		if m.Unreviewed {
			say("  旧版本检查的导入无法列出明细；如需查看请重新导入。")
			continue
		}
		for _, r := range m.Roots {
			line := "  替换目录 " + r.Path
			if r.SystemPath {
				line += "（系统路径）"
			}
			for _, user := range r.UsedBy {
				line += "；现被容器 " + user.Container + map[bool]string{true: " 只读挂载", false: " 挂载"}[user.ReadOnly]
			}
			say(line)
		}
		for _, c := range m.Containers {
			line := "  容器 " + c.Name + " 镜像 " + c.Image
			line += " ImageID=" + c.ImageID + " ImageRef=" + c.ImageRef
			if c.Replaces {
				line += "（替换同名容器）"
			}
			say(line)
			say("    HostConfig:", string(c.HostConfig))
			if c.Privileged {
				say("    特权模式 Privileged")
			}
			if len(c.HostNamespaces) > 0 {
				say("    共享宿主机命名空间:", strings.Join(c.HostNamespaces, ", "))
			}
			if len(c.CapAdd) > 0 {
				say("    CapAdd:", strings.Join(c.CapAdd, ", "))
			}
			if len(c.Devices) > 0 {
				say("    Devices:", strings.Join(c.Devices, ", "))
			}
			if len(c.SecurityOpt) > 0 {
				say("    SecurityOpt:", strings.Join(c.SecurityOpt, ", "))
			}
			for _, b := range c.Binds {
				line := "    宿主机目录 " + b.Source + " -> " + b.Destination + map[bool]string{true: " 只读", false: " 读写"}[b.ReadOnly]
				if b.Live {
					line += "（宿主机现有目录，不随备份恢复）"
				}
				say(line)
			}
			if c.MoreItems > 0 {
				say("    另有", c.MoreItems, "项未列出")
			}
		}
		for _, n := range m.Networks {
			say("  网络", n.Name, "Driver", n.Driver, strings.Join(n.Subnets, ", "), strings.Join(n.Options, ", "))
			say("    NetworkConfig:", string(n.Config))
		}
		if m.SetIDFiles > 0 {
			say("  包含", m.SetIDFiles, "个 setuid/setgid 文件，将按包内属主与权限恢复")
		}
	}
}
