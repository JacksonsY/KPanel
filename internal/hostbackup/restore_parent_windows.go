package hostbackup

import (
	"fmt"
	"os"
	"syscall"
)

// Windows is used by isolated fixtures; bind the same persistent volume/file
// identity that Windows SameFile uses rather than a pathname or timestamp.
func restoreParentIdentity(root *os.Root) (string, error) {
	f, err := root.Open(".")
	if err != nil {
		return "", err
	}
	defer f.Close()
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &info); err != nil {
		return "", err
	}
	return fmt.Sprintf("windows:%d:%d:%d", info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow), nil
}
