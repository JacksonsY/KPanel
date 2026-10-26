//go:build windows

package windowsnode

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

func TestSystemOnlyAtomicRenameRetainsSealedDescriptor(t *testing.T) {
	if !windows.GetCurrentProcessToken().IsElevated() || IsSystem() {
		t.Skip("elevated non-SYSTEM storage fixture required; no services")
	}
	root := t.TempDir()
	parentName, _ := windows.UTF16PtrFromString(root)
	parent, err := windows.CreateFile(parentName, windows.READ_CONTROL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(parent)
	target := filepath.Join(root, "密钥.json")
	t.Cleanup(func() {
		// Only this test's non-secret fixture is made readable for TempDir cleanup.
		sd, err := descriptor(ProgramRead, false)
		if err != nil {
			t.Error(err)
			return
		}
		acl, _, _ := sd.DACL()
		if err := windows.SetNamedSecurityInfo(target, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
			t.Error(err)
		}
	})
	for iteration := 0; iteration < 2; iteration++ {
		tmp := filepath.Join(root, "fixture.tmp")
		name, _ := windows.UTF16PtrFromString(tmp)
		sd, err := descriptor(SystemOnly, false)
		if err != nil {
			t.Fatal(err)
		}
		sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
		h, err := windows.CreateFile(name, windows.GENERIC_WRITE|windows.DELETE, 0, &sa, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err != nil {
			t.Fatal(err)
		}
		f := os.NewFile(uintptr(h), tmp)
		if _, err := f.Write([]byte("non-secret storage fixture")); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Sync(); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := renameAtomicHandle(h, parent, filepath.Base(target)); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(tmp); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("temporary source name survived atomic commit")
		}
		if file, err := os.Open(target); err == nil {
			file.Close()
			t.Fatal("administrator can read committed SYSTEM-only data")
		} else if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Fatal(err)
		}
		actual, err := windows.GetNamedSecurityInfo(target, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateDescriptor(actual, SystemOnly, false); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSystemOnlyHandoffIsSealedAtCreation(t *testing.T) {
	if !windows.GetCurrentProcessToken().IsElevated() || IsSystem() {
		t.Skip("requires an elevated non-SYSTEM test process")
	}
	path := filepath.Join(t.TempDir(), "handoff.json")
	sd, err := descriptor(SystemOnly, false)
	if err != nil {
		t.Fatal(err)
	}
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	name, _ := windows.UTF16PtrFromString(path)
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, &sa, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	file := os.NewFile(uintptr(h), path)
	if _, err := file.Write([]byte("test enrollment handoff")); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if file, err := os.Open(path); err == nil {
		file.Close()
		t.Fatal("administrator could directly reopen SYSTEM-only handoff")
	}
}

func TestServiceSIDMatchesSCMIdentity(t *testing.T) {
	if got := ServiceSID("TrustedInstaller"); got != "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464" {
		t.Fatalf("unexpected service SID %s", got)
	}
	if ServiceSID(TelemetryService) != ServiceSID("kejilionnode") {
		t.Fatal("SID must be case insensitive")
	}
}
func TestProtectedDescriptorBoundaries(t *testing.T) {
	tests := []struct {
		name, sddl      string
		access          Access
		ancestor, valid bool
	}{
		{"system secret", "O:SYD:P(A;;FA;;;SY)", SystemOnly, false, true},
		{"administrator owns sealed secret", "O:BAD:P(A;;FA;;;SY)", SystemOnly, false, true},
		{"administrator cannot read key", "O:SYD:P(A;;FA;;;SY)(A;;FR;;;BA)", SystemOnly, false, false},
		{"users cannot read key", "O:SYD:P(A;;FA;;;SY)(A;;FR;;;BU)", SystemOnly, false, false},
		{"users cannot write program", "O:BAD:P(A;;FA;;;BA)(A;;FW;;;BU)", ProgramRead, false, false},
		{"users may execute program", "O:BAD:P(A;;FA;;;BA)(A;;FRFX;;;BU)", ProgramRead, false, true},
		{"node reads telemetry", "O:SYD:P(A;;FA;;;SY)(A;;FR;;;" + ServiceSID(TelemetryService) + ")", TelemetryRead, false, true},
		{"node cannot write telemetry", "O:SYD:P(A;;FA;;;SY)(A;;FW;;;" + ServiceSID(TelemetryService) + ")", TelemetryRead, false, false},
		{"unsafe parent delete child", "O:SYD:P(A;;FA;;;SY)(A;;0x40;;;BU)", StateDirectory, true, false},
		{"null DACL", "O:SYD:NO_ACCESS_CONTROL", SystemOnly, false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sd, err := windows.SecurityDescriptorFromString(test.sddl)
			if err != nil {
				t.Fatal(err)
			}
			err = validateDescriptor(sd, test.access, test.ancestor)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v err=%v", test.valid, err)
			}
		})
	}
}
func TestNativeReadOnlyPlatformFacts(t *testing.T) {
	if DataDir() == "" || InstallDir() == "" {
		t.Fatal("known folder unavailable")
	}
	if _, err := DomainJoined(); err != nil {
		t.Fatal(err)
	}
}
