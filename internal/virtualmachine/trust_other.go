//go:build !linux

package virtualmachine

import "os"

func rootOwned(os.FileInfo) bool { return false }
