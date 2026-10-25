package virtualmachine

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"encoding/json"
	"github.com/kejilion/kejilion-panel/internal/contract"
	"golang.org/x/crypto/bcrypt"
)

type fakeRunner struct {
	calls   [][]string
	respond func([]string) ([]byte, error)
}

func (f *fakeRunner) LookPath(s string) (string, error) { return "/usr/bin/" + s, nil }
func (f *fakeRunner) Run(_ context.Context, _ io.Reader, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)
	if f.respond != nil {
		return f.respond(call)
	}
	return nil, nil
}

const testID = "11111111-1111-1111-1111-111111111111"

func TestVMSnapshotReadsSupportedVirshVolumeTable(t *testing.T) {
	r := &fakeRunner{respond: func(a []string) ([]byte, error) {
		switch a[3] {
		case "domcapabilities":
			return []byte("<domain>kvm</domain>"), nil
		case "pool-list", "net-list":
			return []byte("default\n"), nil
		case "pool-dumpxml":
			return []byte(`<pool type="dir"><target><path>/var/lib/libvirt/images</path></target></pool>`), nil
		case "vol-list":
			for _, arg := range a[4:] {
				if arg == "--name" {
					return nil, errors.New("vol-list does not support --name")
				}
			}
			return []byte(" Name         Path\n-----------------------------------\n debian.iso   /var/lib/libvirt/images/debian.iso\n disk.qcow2   /var/lib/libvirt/images/disk.qcow2\n ubuntu.ISO   /var/lib/libvirt/images/ubuntu.ISO\n"), nil
		}
		return nil, nil
	}}
	s := New(Config{Runner: r, StateDir: filepath.Join(t.TempDir(), "absent")})
	got, err := s.Snapshot(context.Background())
	if err != nil || len(got.Images) != 2 || got.Images[0].Name != "debian.iso" || got.Images[1].Name != "ubuntu.ISO" {
		t.Fatalf("invalid volume inventory: %#v %v", got.Images, err)
	}
}

func TestVMImageRedirectRequiresExactApprovedDebianFile(t *testing.T) {
	base, file, _, _ := imageSource("debian-12", "amd64")
	original, _ := http.NewRequest(http.MethodGet, base+file, nil)
	target := "https://saimei.ftp.acc.umu.se/images/cloud/bookworm/latest/" + file
	check := imageRedirect("debian-12", base, file)
	for _, test := range []struct {
		url     string
		allowed bool
	}{
		{target, true},
		{strings.Replace(target, "https:", "http:", 1), false},
		{strings.Replace(target, "saimei.", "evil.", 1), false},
		{strings.Replace(target, "saimei.ftp.acc.umu.se", "saimei.ftp.acc.umu.se.evil.example", 1), false},
		{strings.Replace(target, "https://", "https://user@", 1), false},
		{target + "?redirect=elsewhere", false},
		{target + ".other", false},
	} {
		next, _ := http.NewRequest(http.MethodGet, test.url, nil)
		if (check(next, []*http.Request{original}) == nil) != test.allowed {
			t.Fatalf("unexpected redirect decision for %s", test.url)
		}
		if check(next, []*http.Request{original, next}) == nil {
			t.Fatal("accepted second redirect")
		}
	}
	next, _ := http.NewRequest(http.MethodGet, target, nil)
	manifest, _ := http.NewRequest(http.MethodGet, base+"SHA512SUMS", nil)
	if check(next, []*http.Request{manifest}) == nil || imageRedirect("ubuntu-24.04", base, file)(next, []*http.Request{original}) == nil {
		t.Fatal("mirror rule escaped the Debian image request")
	}
}

func TestVMPrepareRejectsInactiveAutostartNetwork(t *testing.T) {
	for _, active := range []bool{false, true} {
		r := &fakeRunner{respond: func(a []string) ([]byte, error) {
			switch a[3] {
			case "net-start":
				return nil, errors.New("network start failed")
			case "net-info":
				if active {
					return []byte("Active: yes\nAutostart: no\n"), nil
				}
				return []byte("Active: no\nAutostart: yes\n"), nil
			}
			return nil, nil
		}}
		err := New(Config{Runner: r}).prepare(context.Background(), func(string) error { return nil })
		if (err == nil) != active {
			t.Fatalf("active=%t: %v", active, err)
		}
	}
}

func TestMachineIdentityAndStableConfigurationVersion(t *testing.T) {
	r := &fakeRunner{respond: func(a []string) ([]byte, error) {
		switch a[3] {
		case "dumpxml":
			return []byte(`<domain><name>external-vm</name><uuid>` + testID + `</uuid><devices><disk device="disk"><source file="/images/disk.qcow2"/><target dev="vda"/></disk><graphics type="vnc" port="5901" listen="127.0.0.1"/></devices></domain>`), nil
		case "dominfo":
			return []byte("State: running\nCPU(s): 2\nMax memory: 1048576 KiB\nAutostart: enable\n"), nil
		}
		return nil, errors.New("unexpected call")
	}}
	s := New(Config{Runner: r})
	m, err := s.Machine(context.Background(), testID)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "external-vm" || m.CPUs != 2 || m.MemoryMiB != 1024 || !m.Autostart || len(m.Disks) != 1 || len(m.ResourceVersion) != 64 {
		t.Fatalf("invalid projection: %#v", m)
	}
	before := len(r.calls)
	if _, err = s.Machine(context.Background(), "--connect=evil"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if len(r.calls) != before {
		t.Fatalf("invalid identity reached runner: %#v", r.calls)
	}
}

func TestChecksumsMatchExactFilename(t *testing.T) {
	digest := strings.Repeat("a", 64)
	for _, name := range []string{"wrong.img", "prefix-noble.img", "noble.img.extra"} {
		if _, err := checksumFor([]byte(digest+" *"+name), "noble.img", 32); err == nil {
			t.Fatal("accepted different image")
		}
	}
	if got, err := checksumFor([]byte(digest+" *noble.img\n"), "noble.img", 32); err != nil || got != digest {
		t.Fatalf("checksum: %s %v", got, err)
	}
	if _, err := checksumFor([]byte("deadbeef *noble.img"), "noble.img", 32); err == nil {
		t.Fatal("accepted short digest")
	}
}

func TestDeletePreservesStorageUnlessExplicitlyRequested(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "retain", true: "remove"}[remove], func(t *testing.T) {
			r := &fakeRunner{respond: func(a []string) ([]byte, error) {
				switch a[3] {
				case "dumpxml":
					return []byte(`<domain><name>external-vm</name><uuid>` + testID + `</uuid><devices/></domain>`), nil
				case "dominfo":
					return []byte("State: shut off\n"), nil
				}
				return nil, nil
			}}
			s := New(Config{Runner: r})
			m, err := s.Machine(context.Background(), testID)
			if err != nil {
				t.Fatal(err)
			}
			r.calls = nil
			j := jobRecord{Request: contract.VMRequest{Action: "delete", ID: testID, ExpectedResourceVersion: m.ResourceVersion, DeleteDisks: remove}}
			if err = s.action(context.Background(), &j, func(string) error { return nil }); err != nil {
				t.Fatal(err)
			}
			var call []string
			for _, a := range r.calls {
				if a[3] == "undefine" {
					call = a
				}
			}
			if (strings.Contains(strings.Join(call, " "), "--remove-all-storage")) != remove {
				t.Fatalf("unexpected deletion: %#v", call)
			}
		})
	}
}

func TestWorkerRunsOnceAndPersistsFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("durable worker uses Linux file locks")
	}
	if os.Geteuid() != 0 {
		t.Skip("root-owned worker fixture runs in the native hosted root qualification")
	}
	s := New(Config{StateDir: privateTestDirectory(t), Runner: &fakeRunner{}})
	j := jobRecord{VMJob: contract.VMJob{ID: testID, Action: "create", Status: "queued"}, Request: contract.VMRequest{Action: "create", RequestID: testID}}
	if err := s.save(&j); err != nil {
		t.Fatal(err)
	}
	// An invalid persisted request cannot launch any tool.
	if err := s.RunJob(context.Background(), testID); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	j.Status = "succeeded"
	if err := s.save(&j); err != nil {
		t.Fatal(err)
	}
	if err := s.RunJob(context.Background(), testID); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	got, err := s.readJob(testID)
	if err != nil || got.Status != "succeeded" {
		t.Fatalf("lost receipt: %#v %v", got, err)
	}
}

func TestWorkerRejectsNonRootOwnedState(t *testing.T) {
	if runtime.GOOS != "linux" || os.Geteuid() == 0 {
		t.Skip("requires an unprivileged Linux user")
	}
	runner := &fakeRunner{}
	s := New(Config{StateDir: privateTestDirectory(t), Runner: runner, Enabled: true})
	j := jobRecord{VMJob: contract.VMJob{ID: testID, Action: "prepare", Status: "queued"}, Request: contract.VMRequest{Action: "prepare", RequestID: testID}}
	if err := s.save(&j); err != nil {
		t.Fatal(err)
	}
	err := s.RunJob(context.Background(), testID)
	if err == nil || !strings.Contains(err.Error(), "private real directory") || len(runner.calls) != 0 {
		t.Fatalf("unprivileged state reached worker execution: error=%v calls=%v", err, runner.calls)
	}
}

func TestPrivateStateAndBoundedCommands(t *testing.T) {
	dir := t.TempDir()
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := privateDirectory(dir); err == nil {
			t.Fatal("accepted public task directory")
		}
	}
	b := &boundedBuffer{limit: 4}
	if _, err := b.Write([]byte("12345")); err == nil {
		t.Fatal("unbounded output")
	}
	if _, _, _, err := imageSource("http://127.0.0.1/secret", "amd64"); !errors.Is(err, ErrInvalid) {
		t.Fatal("accepted arbitrary image source")
	}
	s := New(Config{StateDir: filepath.Join(dir, "jobs"), Enabled: false})
	if _, err := s.Start(context.Background(), contract.VMRequest{}); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.spec(testID).Arguments, []string{"vm-run", "--state-dir", s.dir, "--job", testID}) {
		t.Fatal("worker arguments drifted")
	}
}

func TestCloudImageRejectsExternalFilesBeforeImageTool(t *testing.T) {
	base := make([]byte, 104)
	copy(base, []byte("QFI\xfb"))
	binary.BigEndian.PutUint32(base[4:8], 3)
	binary.BigEndian.PutUint64(base[24:32], 4<<30)
	for _, test := range []struct {
		name   string
		change func([]byte)
	}{
		{"backing-offset", func(b []byte) { binary.BigEndian.PutUint64(b[8:16], 104) }},
		{"backing-name", func(b []byte) { binary.BigEndian.PutUint32(b[16:20], 11) }},
		{"external-data", func(b []byte) { binary.BigEndian.PutUint64(b[72:80], 4) }},
		{"encrypted", func(b []byte) { binary.BigEndian.PutUint32(b[32:36], 1) }},
		{"oversized-virtual-disk", func(b []byte) { binary.BigEndian.PutUint64(b[24:32], 64<<30) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := append([]byte(nil), base...)
			test.change(b)
			path := filepath.Join(t.TempDir(), "image.qcow2")
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			if err := validateCloudDisk(path, 20<<30); err == nil {
				t.Fatal("accepted external or incompatible image")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "valid.qcow2")
	if err := os.WriteFile(path, base, 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateCloudDisk(path, 20<<30); err != nil {
		t.Fatal(err)
	}
}

func TestPoolRejectsUntrustedOwnerAncestorAndSymlink(t *testing.T) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		t.Skip("requires root Linux inode ownership")
	}
	dir := t.TempDir()
	pool := filepath.Join(dir, "pool")
	if err := os.Mkdir(pool, 0755); err != nil {
		t.Fatal(err)
	}
	if err := trustedPoolDirectory(pool); err != nil {
		t.Fatalf("root private pool rejected: %v", err)
	}
	if err := os.Chown(pool, 65534, 65534); err != nil {
		t.Fatal(err)
	}
	if err := trustedPoolDirectory(pool); err == nil {
		t.Fatal("accepted user-owned 0755 pool")
	}
	if err := os.Chown(pool, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if err := trustedPoolDirectory(pool); err == nil {
		t.Fatal("accepted replaceable ancestor")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked")
	if err := os.Symlink(pool, link); err != nil {
		t.Fatal(err)
	}
	if err := trustedPoolDirectory(link); err == nil {
		t.Fatal("accepted linked pool")
	}
}

func TestPasswordIsHashedBeforeDurableSubmissionAndNeverProjected(t *testing.T) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		t.Skip("requires private root Linux task state")
	}
	runner := &fakeRunner{respond: func(args []string) ([]byte, error) {
		if args[0] == "virsh" {
			switch args[3] {
			case "uri":
				return []byte("qemu:///system"), nil
			case "domcapabilities":
				return []byte("<domain>kvm</domain>"), nil
			}
		}
		return nil, nil
	}}
	s := New(Config{Enabled: true, StateDir: privateTestDirectory(t), Runner: runner})
	r := contract.VMRequest{Action: "create", RequestID: testID, Name: "password-test", Source: "cloud", Image: "debian-12", Pool: "default", Network: "default", CPUs: 1, MemoryMiB: 1024, DiskGiB: 20, Username: "kpanel", Password: "example-secret-password"}
	job, err := s.Start(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(s.dir, testID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), r.Password) {
		t.Fatal("plaintext password persisted")
	}
	stored, err := s.readJob(testID)
	if err != nil {
		t.Fatal(err)
	}
	if err = bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(r.Password)); err != nil {
		t.Fatal("stored hash cannot authenticate chosen password")
	}
	public, _ := json.Marshal(job)
	if strings.Contains(string(public), r.Password) || strings.Contains(string(public), stored.PasswordHash) || strings.Contains(string(public), "passwordHash") {
		t.Fatal("credential hash leaked into job projection")
	}
	before := len(runner.calls)
	again, err := s.Start(context.Background(), r)
	if err != nil || again.ID != job.ID || len(runner.calls) != before {
		t.Fatalf("duplicate launched another worker: %v", err)
	}
	r.Password = "different-password"
	if _, err = s.Start(context.Background(), r); !errors.Is(err, ErrConflict) {
		t.Fatal("different password reused request identity")
	}
}

func privateTestDirectory(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
