package browsercomponent

import (
	"os"
	"os/exec"
	"testing"
)

// This journey changes Docker resources and is restricted to disposable GitHub-hosted runners.
func TestHostedBrowserReinstallAndNavigation(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Skip("requires a disposable GitHub-hosted runner")
	}
	// Bind Go's test cache to this runner execution; a previous run is not proof
	// that the current candidate completed the real Docker journey.
	if os.Getenv("GITHUB_RUN_ID") == "" || os.Getenv("GITHUB_RUN_ATTEMPT") == "" {
		t.Fatal("missing hosted run identity")
	}
	cmd := exec.Command("sudo", "-n", "--preserve-env=PATH,GITHUB_ACTIONS,RUNNER_ENVIRONMENT,RUNNER_TEMP,GITHUB_RUN_ID,GITHUB_RUN_ATTEMPT", "bash", "testdata/hosted-lifecycle.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("browser lifecycle journey: %v", err)
	}
}
