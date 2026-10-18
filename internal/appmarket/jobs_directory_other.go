//go:build !linux

package appmarket

import "os"

func trustedJobDirectoryOwner(os.FileInfo) bool {
	return true
}

func trustedJobDirectoryMode(os.FileInfo) bool {
	return true
}
