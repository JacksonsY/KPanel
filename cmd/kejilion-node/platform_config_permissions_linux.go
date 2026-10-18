//go:build linux

package main

import (
	"errors"
	"os"
)

func validatePlatformConfigAccess(file *os.File, secret bool) error {
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || (!rootOwned(info) && !processOwned(info)) {
		return errors.New("configuration file ownership is unsafe")
	}
	mode := info.Mode().Perm()
	if secret {
		if mode&0o077 != 0 || mode&0o400 == 0 {
			return errors.New("secret configuration permissions are unsafe")
		}
	} else if mode&0o137 != 0 {
		return errors.New("configuration permissions are unsafe")
	}
	return nil
}
