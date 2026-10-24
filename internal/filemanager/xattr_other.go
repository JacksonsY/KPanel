//go:build !linux

package filemanager

import "os"

func preserveFileExtendedAttributes(_, _ *os.File, _ bool) error {
	return nil
}
