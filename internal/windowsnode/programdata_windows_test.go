//go:build windows

package windowsnode

import (
	"golang.org/x/sys/windows"
	"testing"
)

func TestStandardProgramDataMetadataIsNarrowlyScoped(t *testing.T) {
	for _, test := range []struct {
		mask, sid    string
		known, valid bool
	}{
		{"0x1201bf", "BU", true, true},
		{"0x1201bf", "BU", false, false},
		{"0x1301bf", "BU", true, false},
		{"0x1201ff", "BU", true, false},
		{"0x1201bf", "WD", true, false},
	} {
		sd, err := windows.SecurityDescriptorFromString("O:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;OICIIO;FA;;;CO)(A;;" + test.mask + ";;;" + test.sid + ")")
		if err != nil {
			t.Fatal(err)
		}
		err = validateDescriptor(sd, StateDirectory, true, test.known)
		if (err == nil) != test.valid {
			t.Fatalf("%+v err=%v", test, err)
		}
	}
}
