package cluster

import (
	"context"
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestWindowsCommandNodeEnrollmentReportAndRestart(t *testing.T) {
	now := time.Now().UTC()
	s := newLightServiceForTest(t, &serviceTestClock{now: now})
	enrollment, err := s.CreateLightEnrollment()
	if err != nil {
		t.Fatal(err)
	}
	if enrollment.WindowsCommand == "" || strings.Contains(enrollment.WindowsCommand, "KPANEL_NODE_TOKEN") || strings.Contains(enrollment.WindowsCommand, "EnableDesktop") {
		t.Fatal("missing Windows command or credential/desktop regression")
	}
	key, err := GenerateFederationV2Keypair()
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(enrollment.Command)
	enrolled, err := s.EnrollLightNode("198.51.100.1", LightEnrollRequest{Platform: "windows", Token: strings.Trim(fields[len(fields)-1], "'"), TerminalPublicKey: base64.RawURLEncoding.EncodeToString(key.Public)})
	if err != nil {
		t.Fatal(err)
	}
	host, err := s.Host(context.Background(), enrolled.NodeID)
	if err != nil || host.Platform != "windows" || host.TerminalShell != "powershell" {
		t.Fatalf("Windows host = %#v, %v", host, err)
	}
	input := LightReportRequest{Platform: "windows", Telemetry: serviceTelemetry(now, "windows-node")}
	input.Telemetry.OS, input.Telemetry.OSID = "Windows Server 2025", "windows"
	input.Telemetry.UnavailableMetrics = []string{"load", "swap", "diskIO", "networkConnections"}
	body, auth := signedLightReportForTest(t, now, enrolled, input, strings.Repeat("a", 32))
	if _, err := s.AcceptLightReport(auth, body, input); err != nil {
		t.Fatal(err)
	}
	reopened, err := openLightStore(s.light.path)
	if err != nil {
		t.Fatal(err)
	}
	record, err := reopened.Host(enrolled.NodeID)
	if err != nil || !record.WindowsCommand || !lightNodePlatformSupported(record) || record.LastSnapshot == nil {
		t.Fatalf("Windows enrollment lost on restart: %#v, %v", record, err)
	}
	if !slices.Equal(record.LastSnapshot.Telemetry.UnavailableMetrics, input.Telemetry.UnavailableMetrics) {
		t.Fatal("missing metric availability")
	}
	// Low privilege reporting cannot change an enrolled Windows identity to Linux.
	input.Platform = "linux"
	input.Telemetry = serviceTelemetry(now, "pretend-linux")
	body, auth = signedLightReportForTest(t, now, enrolled, input, strings.Repeat("b", 32))
	if _, err := s.AcceptLightReport(auth, body, input); !errors.Is(err, ErrLightPlatformUnsupported) {
		t.Fatalf("platform switch = %v", err)
	}
	if !s.lightControlAllowed(enrolled.NodeID, "files") || !s.lightControlAllowed(enrolled.NodeID, "terminal") {
		t.Fatal("new Windows identity blocked")
	}
}

func TestWindowsBatchRetryCannotChangePlatform(t *testing.T) {
	s := newLightServiceForTest(t, &serviceTestClock{now: time.Now().UTC()})
	batch, err := s.CreateLightBatchEnrollment(CreateLightBatchEnrollmentInput{MaxUses: 2})
	if err != nil {
		t.Fatal(err)
	}
	input := LightEnrollRequest{Platform: "windows", Token: lightBatchTokenForTest(t, batch), AttemptID: strings.Repeat("c", 32)}
	first, _, err := s.EnrollLightNodeBatch("198.51.100.1", s.publicURL, input)
	if err != nil {
		t.Fatal(err)
	}
	retry, _, err := s.EnrollLightNodeBatch("198.51.100.1", s.publicURL, input)
	if err != nil || retry.NodeID != first.NodeID {
		t.Fatalf("idempotent retry = %#v %v", retry, err)
	}
	input.Platform = "linux"
	if _, _, err = s.EnrollLightNodeBatch("198.51.100.1", s.publicURL, input); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("cross-platform retry = %v", err)
	}
	if got := s.LightBatchEnrollments().Items[0].UsedCount; got != 1 {
		t.Fatalf("retry consumed %d uses", got)
	}
}

func TestWindowsCommandQuotesArbitraryDisplayTextAndPinsBootstrap(t *testing.T) {
	s := newLightServiceForTest(t, &serviceTestClock{now: time.Now().UTC()})
	name := "Windows ‘测试’ $(Write-Output injected)"
	command, err := s.windowsEnrollmentCommand("kpl1.token", name)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(command, name) || !strings.Contains(command, powershellSingleQuote(name)) || !strings.Contains(command, windowsBootstrapScriptSHA256()) {
		t.Fatal("unquoted display text or unpinned bootstrap")
	}
	for _, forbidden := range []string{"-EnableDesktop", "RDP", "KPANEL_NODE_TOKEN", "EncodedCommand"} {
		if strings.Contains(command, forbidden) {
			t.Fatalf("unexpected launcher behavior: %s", forbidden)
		}
	}
}
