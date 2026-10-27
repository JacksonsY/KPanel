//go:build !linux

package browsercomponent

import "os"

func rootOwned(os.FileInfo) bool { return true }

func lockDirectory(string, string) (func(), error) { return nil, ErrUnsupported }
