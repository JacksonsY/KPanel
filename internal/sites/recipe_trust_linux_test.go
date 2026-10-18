//go:build linux

package sites

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecipeScriptParentsRejectWritableDirectories(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "home", "docker", "kpanel", "bin")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "home", "docker"), 0o777); err != nil {
		t.Fatal(err)
	}
	if recipeScriptParentsTrusted(installDir) {
		t.Fatal("script under a writable parent directory was trusted")
	}
}
