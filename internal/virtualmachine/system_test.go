package virtualmachine

import (
	"context"
	"encoding/xml"
	"errors"
	"strings"
	"testing"
)

func TestMachineDeclaredOS(t *testing.T) {
	for _, tc := range []struct{ metadata, want string }{
		{`<metadata><libosinfo:libosinfo xmlns:libosinfo="http://libosinfo.org/xmlns/libvirt/domain/1.0"><libosinfo:os id="http://debian.org/debian/12"/></libosinfo:libosinfo></metadata>`, "http://debian.org/debian/12"},
		{`<metadata><libosinfo xmlns="http://libosinfo.org/xmlns/libvirt/domain/1.0"><os id="http://ubuntu.com/ubuntu/24.04"/></libosinfo></metadata>`, "http://ubuntu.com/ubuntu/24.04"},
		{`<metadata><libosinfo><os id="http://debian.org/debian/12"/></libosinfo></metadata>`, ""},
		{`<metadata><libosinfo xmlns="urn:other"><os id="http://debian.org/debian/12"/></libosinfo></metadata>`, ""},
		{"", ""},
	} {
		r := &fakeRunner{respond: func(a []string) ([]byte, error) {
			switch a[3] {
			case "dumpxml":
				return []byte(`<domain><name>ubuntu-misleading-name</name><uuid>` + testID + `</uuid>` + tc.metadata + `</domain>`), nil
			case "dominfo":
				return []byte("State: shut off\n"), nil
			}
			return nil, errors.New("unexpected command")
		}}
		m, err := New(Config{Runner: r}).Machine(context.Background(), testID)
		if err != nil || m.OSID != tc.want {
			t.Fatalf("metadata=%s: os=%q err=%v", tc.metadata, m.OSID, err)
		}
	}
}

func TestRecordCloudOSPersistsRecognizedMetadata(t *testing.T) {
	for _, image := range []string{"debian-12", "ubuntu-24.04"} {
		for _, state := range []string{"running", "paused", "shut off"} {
			var metadata string
			r := &fakeRunner{respond: func(a []string) ([]byte, error) {
				switch a[3] {
				case "metadata":
					if strings.Contains(a[10], "xmlns") || strings.Contains(a[10], "libosinfo:") {
						return nil, errors.New("libvirt must inject the metadata namespace")
					}
					metadata = strings.Replace(a[10], "<libosinfo>", `<libosinfo xmlns="http://libosinfo.org/xmlns/libvirt/domain/1.0">`, 1)
				case "dumpxml":
					return []byte(`<domain><name>renamed-workload</name><uuid>` + testID + `</uuid><metadata>` + metadata + `</metadata></domain>`), nil
				case "dominfo":
					return []byte("State: " + state + "\n"), nil
				}
				return nil, nil
			}}
			err := New(Config{Runner: r}).recordCloudOS(context.Background(), testID, image, state)
			if err != nil || len(r.calls) != 4 {
				t.Fatalf("%s %s: calls=%v err=%v", image, state, r.calls, err)
			}
			args := r.calls[0]
			if strings.Join(args[:5], " ") != "virsh --connect qemu:///system metadata "+testID || args[11] != "--config" {
				t.Fatalf("wrong metadata target: %v", args)
			}
			var d domainXML
			if err := xml.Unmarshal([]byte("<domain><metadata>"+metadata+"</metadata></domain>"), &d); err != nil || d.Metadata.LibOSInfo.OS.ID == "" {
				t.Fatalf("unreadable metadata: %v %v", args, err)
			}
			want := "http://debian.org/debian/12"
			if image == "ubuntu-24.04" {
				want = "http://ubuntu.com/ubuntu/24.04"
			}
			if d.Metadata.LibOSInfo.OS.ID != want || (len(args) == 13 && args[12] == "--live") != (state != "shut off") {
				t.Fatalf("incorrect OS or persistence flags: %v", args)
			}
		}
	}
}

func TestRecordCloudOSRejectsUnknownInputsAndReportsWriteFailure(t *testing.T) {
	r := &fakeRunner{respond: func([]string) ([]byte, error) { return nil, errors.New("metadata write failed") }}
	s := New(Config{Runner: r})
	if err := s.recordCloudOS(context.Background(), testID, "custom", "running"); !errors.Is(err, ErrInvalid) || len(r.calls) != 0 {
		t.Fatalf("unknown image reached runner: %v", err)
	}
	if err := s.recordCloudOS(context.Background(), "--connect=evil", "debian-12", "running"); !errors.Is(err, ErrInvalid) || len(r.calls) != 0 {
		t.Fatalf("invalid domain reached runner: %v", err)
	}
	if err := s.recordCloudOS(context.Background(), testID, "debian-12", "running"); err == nil || !strings.Contains(err.Error(), "metadata write failed") {
		t.Fatalf("write failure lost: %v", err)
	}
}

func TestRecordCloudOSRequiresMatchingReadback(t *testing.T) {
	r := &fakeRunner{respond: func(a []string) ([]byte, error) {
		if a[3] == "dumpxml" {
			return []byte(`<domain><uuid>` + testID + `</uuid></domain>`), nil
		}
		return nil, nil
	}}
	if err := New(Config{Runner: r}).recordCloudOS(context.Background(), testID, "debian-12", "shut off"); !errors.Is(err, ErrConflict) {
		t.Fatalf("missing metadata readback was accepted: %v", err)
	}
}
