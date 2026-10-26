//go:build windows

package filemanager

import (
	"golang.org/x/sys/windows"
	"testing"
)

func TestWindowsPrivateStagingDirectory(t *testing.T) {
	m, _, _ := windowsVolumeManager(t)
	root, err := m.rootFS.OpenRoot("C")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	info, err := root.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	file, err := root.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	sd, err := windows.GetSecurityInfo(windows.Handle(file.Fd()), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if sd == nil || !archiveWorkspaceProtected(info, root) {
		t.Fatal("trusted fixture workspace rejected")
	}
	if err = root.Mkdir("private", 0700); err != nil {
		t.Fatal(err)
	}
	stage, err := root.OpenRoot("private")
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	private, err := stage.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	if !archiveStagePrivate(private, stage) {
		t.Fatal("private staging DACL is not enforced")
	}
	setTestDACL(t, stage.Name(), testAccessSDDL(t, true, true))
	if archiveStagePrivate(private, stage) {
		t.Fatal("stage with public read access accepted as private")
	}
}
