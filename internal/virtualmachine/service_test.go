package virtualmachine

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/contract"
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
	s := New(Config{StateDir: t.TempDir(), Runner: &fakeRunner{}})
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
