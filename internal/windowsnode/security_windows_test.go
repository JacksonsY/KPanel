//go:build windows

package windowsnode

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

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
