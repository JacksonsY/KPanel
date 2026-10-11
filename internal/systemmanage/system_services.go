package systemmanage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
	"github.com/kejilion/kejilion-panel/internal/redact"
)

var ErrSystemServicesBusy = errors.New("another service operation is in progress")

const serviceDetailProperties = "Id,Description,LoadState,ActiveState,SubState,UnitFileState,FragmentPath,MainPID,CanReload,CanStart,CanStop,NeedDaemonReload"

// Fileless/transient units can carry an execution definition only in the
// manager. Read fixed properties, never expose them (they may contain secrets).
const serviceRuntimeConfigProperties = "Type,ExecCondition,ExecStart,ExecStartPre,ExecStartPost,ExecReload,ExecStop,ExecStopPost,Environment,EnvironmentFiles,WorkingDirectory,RootDirectory,RootImage,User,Group,DynamicUser,SupplementaryGroups,Restart,KillMode,KillSignal,PIDFile,RemainAfterExit,BusName,StandardInput,StandardOutput,StandardError,UMask,CapabilityBoundingSet,AmbientCapabilities,NoNewPrivileges,ProtectSystem,ProtectHome,PrivateTmp,PrivateDevices,ReadWritePaths,ReadOnlyPaths,InaccessiblePaths,BindPaths,BindReadOnlyPaths,DevicePolicy,DeviceAllow,SystemCallFilter,RestrictAddressFamilies,TimeoutStartUSec,TimeoutStopUSec,MemoryMax,TasksMax,LimitNOFILE,LimitNPROC"

var serviceExecRuntimeMetadata = regexp.MustCompile(` ; start_time=\[[^\]\n]*\] ; stop_time=\[[^\]\n]*\] ; pid=[0-9]+ ; code=[^;{}\n]* ; status=[^;{}\n]* (})`)

func stableServiceRuntimeConfiguration(output []byte) []byte {
	lines := strings.Split(string(output), "\n")
	for index, line := range lines {
		key, _, _ := strings.Cut(line, "=")
		switch key {
		case "ExecCondition", "ExecStart", "ExecStartPre", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost":
			lines[index] = serviceExecRuntimeMetadata.ReplaceAllString(line, " $1")
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

func (m *Manager) runSystemServiceCommand(ctx context.Context, limit int, name string, arguments ...string) ([]byte, []byte, error) {
	output, stderr, err := m.runResourceCommand(ctx, limit, name, arguments...)
	if ctx.Err() != nil {
		return output, stderr, ctx.Err()
	}
	return output, stderr, err
}

func (m *Manager) systemServicesAvailable() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("%w: systemd 服务管理仅支持 Linux", ErrUnsupported)
	}
	if m.effectiveUID() != 0 {
		return fmt.Errorf("%w: Agent 必须以受限 root 服务运行", ErrUnsupported)
	}
	if m.openRCRuntimeActive() {
		return fmt.Errorf("%w: 当前主机运行 OpenRC，不支持 systemd 服务管理", ErrUnsupported)
	}
	info, err := os.Stat(filepath.Join(m.runRoot, "systemd", "system"))
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%w: 当前主机没有运行 systemd", ErrUnsupported)
	}
	if _, err := m.runner.LookPath("systemctl"); err != nil {
		return fmt.Errorf("%w: systemctl 不可用", ErrUnsupported)
	}
	return nil
}

func (m *Manager) SystemServiceCapabilities() []contract.Capability {
	read := contract.Capability{ID: "system.services.read", Methods: []string{"GET"}}
	write := contract.Capability{ID: "system.services.write", Methods: []string{"POST"}}
	if err := m.systemServicesAvailable(); err != nil {
		read.Reason, write.Reason = err.Error(), err.Error()
	} else {
		read.Enabled = true
		write.Enabled = m.enabled
		if !write.Enabled {
			write.Reason = "宿主机系统写入开关未启用"
		}
	}
	return []contract.Capability{read, write}
}

func (m *Manager) SystemServices(ctx context.Context) (contract.SystemServicesSnapshot, error) {
	result := contract.SystemServicesSnapshot{Services: []contract.SystemService{}, ObservedAt: m.now().UTC()}
	if err := m.systemServicesAvailable(); err != nil {
		result.Reason = err.Error()
		return result, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	files, _, filesErr := m.runSystemServiceCommand(ctx, contract.SystemServiceOutputLimit, "systemctl", "--no-pager", "--no-legend", "--plain", "--full", "list-unit-files", "--type=service")
	if filesErr != nil && !errors.Is(filesErr, errResourceOutputTooLarge) {
		return result, filesErr
	}
	units, _, unitsErr := m.runSystemServiceCommand(ctx, contract.SystemServiceOutputLimit, "systemctl", "--no-pager", "--no-legend", "--plain", "--full", "list-units", "--all", "--type=service")
	if unitsErr != nil && !errors.Is(unitsErr, errResourceOutputTooLarge) {
		return result, unitsErr
	}
	result.Services, result.Truncated = parseSystemServices(files, units)
	if unitsErr != nil {
		for index := range result.Services {
			if result.Services[index].LoadState == "unloaded" {
				result.Services[index].LoadState, result.Services[index].ActiveState, result.Services[index].SubState = "unknown", "unknown", "unknown"
			}
		}
	}
	result.Truncated = result.Truncated || filesErr != nil || unitsErr != nil
	result.Supported = true
	return result, nil
}

func parseSystemServices(files, units []byte) ([]contract.SystemService, bool) {
	services := make(map[string]contract.SystemService)
	truncated := false
	// Ignore an incomplete final line from a bounded command prefix.
	for _, line := range completeServiceLines(files) {
		fields := strings.Fields(line)
		if len(fields) < 2 || !contract.ValidSystemServiceName(fields[0]) {
			continue
		}
		if len(services) >= contract.SystemServiceMaxEntries {
			truncated = true
			break
		}
		// An absent loaded-unit row may be an alias of another running unit.
		// Preserve the installed file without inventing an inactive state.
		services[fields[0]] = contract.SystemService{Name: fields[0], LoadState: "unloaded", ActiveState: "unknown", SubState: "unknown", UnitFileState: sanitizeLogText(fields[1], 64)}
	}
	for _, line := range completeServiceLines(units) {
		fields := strings.Fields(line)
		if len(fields) < 4 || !contract.ValidSystemServiceName(fields[0]) {
			continue
		}
		service, exists := services[fields[0]]
		if !exists && len(services) >= contract.SystemServiceMaxEntries {
			truncated = true
			continue
		}
		service.Name = fields[0]
		service.LoadState, service.ActiveState, service.SubState = sanitizeLogText(fields[1], 64), sanitizeLogText(fields[2], 64), sanitizeLogText(fields[3], 64)
		service.Description = sanitizeLogText(redact.Text(strings.Join(fields[4:], " ")), 1024)
		services[service.Name] = service
	}
	result := make([]contract.SystemService, 0, len(services))
	for _, service := range services {
		result = append(result, service)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, truncated
}

func completeServiceLines(output []byte) []string {
	text := string(output)
	if index := strings.LastIndexByte(text, '\n'); index >= 0 {
		return strings.Split(text[:index], "\n")
	}
	return nil
}

func (m *Manager) SystemService(ctx context.Context, name string) (contract.SystemServiceDetail, error) {
	if !contract.ValidSystemServiceName(name) {
		return contract.SystemServiceDetail{}, ErrInvalidInput
	}
	if err := m.systemServicesAvailable(); err != nil {
		return contract.SystemServiceDetail{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, _, err := m.runSystemServiceCommand(ctx, 16<<10, "systemctl", "--no-pager", "--all", "--property="+serviceDetailProperties, "show", "--", name)
	if err != nil {
		return contract.SystemServiceDetail{}, err
	}
	detail, err := parseSystemServiceDetail(name, output)
	if err != nil {
		return detail, err
	}
	var configuration []byte
	if detail.FragmentPath != "" && detail.LoadState != "masked" {
		configuration, _, err = m.runSystemServiceCommand(ctx, 256<<10, "systemctl", "--no-pager", "cat", "--", name)
	} else {
		configuration, _, err = m.runSystemServiceCommand(ctx, 256<<10, "systemctl", "--no-pager", "--all", "--property="+serviceRuntimeConfigProperties, "show", "--", name)
		// Exec* exposes both command definitions and per-execution metadata.
		// Strip only the complete systemd-generated metadata tuple so ordinary
		// PID/timestamp changes do not invalidate a fileless unit's version.
		configuration = stableServiceRuntimeConfiguration(configuration)
	}
	if err != nil {
		return detail, fmt.Errorf("%w: 无法读取服务配置版本: %w", ErrUnsupported, err)
	}
	bindSystemServiceConfiguration(&detail, configuration)
	return detail, nil
}

func bindSystemServiceConfiguration(detail *contract.SystemServiceDetail, configuration []byte) {
	digest := sha256.New()
	digest.Write([]byte(detail.ResourceVersion + "\x00"))
	digest.Write(configuration)
	detail.ResourceVersion = hex.EncodeToString(digest.Sum(nil))
}

func parseSystemServiceDetail(name string, output []byte) (contract.SystemServiceDetail, error) {
	result := contract.SystemServiceDetail{SystemService: contract.SystemService{Name: name}}
	properties := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return result, fmt.Errorf("%w: invalid service state", ErrUnsupported)
		}
		if _, duplicate := properties[key]; duplicate {
			return result, fmt.Errorf("%w: duplicate service state", ErrUnsupported)
		}
		properties[key] = value
	}
	for _, key := range strings.Split(serviceDetailProperties, ",") {
		if _, exists := properties[key]; !exists {
			return result, fmt.Errorf("%w: missing service property %s", ErrUnsupported, key)
		}
	}
	if !contract.ValidSystemServiceName(properties["Id"]) || properties["LoadState"] == "not-found" {
		return result, fmt.Errorf("%w: service not found", ErrInvalidInput)
	}
	pid, err := strconv.Atoi(properties["MainPID"])
	if err != nil || pid < 0 {
		return result, fmt.Errorf("%w: invalid service PID", ErrUnsupported)
	}
	result.Description = sanitizeLogText(redact.Text(properties["Description"]), 1024)
	result.LoadState, result.ActiveState, result.SubState = properties["LoadState"], properties["ActiveState"], properties["SubState"]
	result.UnitFileState = properties["UnitFileState"]
	result.FragmentPath, result.MainPID = sanitizeLogText(properties["FragmentPath"], 4096), pid
	for _, key := range []string{"CanReload", "CanStart", "CanStop", "NeedDaemonReload"} {
		if properties[key] != "yes" && properties[key] != "no" {
			return result, fmt.Errorf("%w: invalid service boolean", ErrUnsupported)
		}
	}
	result.CanReload, result.CanStart, result.CanStop = properties["CanReload"] == "yes", properties["CanStart"] == "yes", properties["CanStop"] == "yes"
	// Version the selected unit's identity, configuration and action-relevant
	// state; PID and timestamps must not turn routine process churn into conflict.
	var version strings.Builder
	for _, key := range strings.Split(serviceDetailProperties, ",") {
		if key != "MainPID" {
			version.WriteString(key + "=" + properties[key] + "\x00")
		}
	}
	digest := sha256.Sum256([]byte(version.String()))
	result.ResourceVersion = hex.EncodeToString(digest[:])
	return result, nil
}

func (m *Manager) SystemServiceLogs(ctx context.Context, name string) (contract.SystemServiceLogs, error) {
	result := contract.SystemServiceLogs{Name: name, ObservedAt: m.now().UTC()}
	if !contract.ValidSystemServiceName(name) {
		return result, ErrInvalidInput
	}
	if err := m.systemServicesAvailable(); err != nil {
		return result, err
	}
	if _, err := m.runner.LookPath("journalctl"); err != nil {
		return result, fmt.Errorf("%w: journalctl 不可用", ErrUnsupported)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, _, err := m.runSystemServiceCommand(ctx, contract.SystemServiceLogOutputLimit, "journalctl", "--no-pager", "--quiet", "--output=short-iso", "--lines="+strconv.Itoa(contract.SystemServiceLogLines+1), "--unit="+name)
	if err != nil && !errors.Is(err, errResourceOutputTooLarge) {
		return result, err
	}
	result.Truncated = err != nil || len(strings.Split(strings.TrimRight(string(output), "\n"), "\n")) > contract.SystemServiceLogLines
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) > 2048 {
			result.Truncated = true
		}
	}
	lines := redact.Lines(output, contract.SystemServiceLogLines, 2048)
	for index := range lines {
		lines[index] = normalizeLogText(lines[index], 2051)
	}
	result.Content = strings.Join(lines, "\n")
	return result, nil
}

func (m *Manager) ExecuteSystemServiceAction(ctx context.Context, input contract.SystemServiceActionRequest) (contract.SystemServiceActionResult, error) {
	result := contract.SystemServiceActionResult{Name: input.Name, Action: input.Action}
	if field, detail := contract.ValidateSystemServiceAction(&input); field != "" {
		return result, fmt.Errorf("%w: %s: %s", ErrInvalidInput, field, detail)
	}
	if !m.enabled {
		return result, ErrDisabled
	}
	if err := m.systemServicesAvailable(); err != nil {
		return result, err
	}
	if !m.servicesMu.TryLock() {
		return result, ErrSystemServicesBusy
	}
	defer m.servicesMu.Unlock()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	before, err := m.SystemService(ctx, input.Name)
	if err != nil {
		return result, err
	}
	if before.ResourceVersion != input.ExpectedResourceVersion {
		return result, ErrConflict
	}
	arguments := []string{"--no-pager", "--no-ask-password"}
	lifecycle := input.Action != "enable" && input.Action != "disable"
	if lifecycle {
		arguments = append(arguments, "--no-block")
	}
	arguments = append(arguments, input.Action, "--", input.Name)
	if _, _, err := m.runSystemServiceCommand(ctx, 16<<10, "systemctl", arguments...); err != nil {
		return result, err
	}
	result.Accepted = lifecycle
	if lifecycle {
		result.Message = "操作请求已提交给 systemd；请刷新服务状态确认执行结果"
	} else {
		result.Message = "开机启动设置已更新；服务当前运行状态保持不变"
	}
	after, err := m.SystemService(ctx, input.Name)
	if err != nil {
		if lifecycle {
			result.Message = "操作请求已提交；暂时无法回读服务状态，请稍后刷新确认"
			return result, nil
		}
		return result, fmt.Errorf("%w: 开机启动操作已执行，但状态回读失败: %v", ErrNeedsAttention, err)
	}
	result.Service = &after
	if !lifecycle {
		if (input.Action == "enable" && after.UnitFileState != "enabled") ||
			(input.Action == "disable" && (after.UnitFileState == "enabled" || after.UnitFileState == "enabled-runtime")) {
			return result, fmt.Errorf("%w: 开机启动状态未确认，请刷新后核对 systemd 返回状态", ErrNeedsAttention)
		}
	}
	return result, nil
}
