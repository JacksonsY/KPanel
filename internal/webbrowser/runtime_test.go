package webbrowser

import (
	"os"
	"testing"
)

func TestSandboxStatusFailsClosed(t *testing.T) {
	for _, status := range []string{"", "PID namespaces Yes\nSeccomp-BPF sandbox No", "PID namespaces No\nSeccomp-BPF sandbox Yes"} {
		if sandboxEnabled(status) {
			t.Fatalf("accepted an unavailable sandbox: %q", status)
		}
	}
	if !sandboxEnabled("PID namespaces\tYes\nSeccomp-BPF sandbox\tYes") {
		t.Fatal("rejected enabled sandbox")
	}
}

func TestRealBrowserSandbox(t *testing.T) {
	if os.Getenv("KPANEL_TEST_SANDBOX") != "1" {
		t.Skip("set KPANEL_TEST_SANDBOX=1 for Linux container qualification")
	}
	if err := CheckRuntime(os.Getenv("KPANEL_TEST_CHROME")); err != nil {
		t.Fatal(err)
	}
}
