//go:build !linux

package main

import "os"

func validatePlatformConfigAccess(*os.File, bool) error { return nil }
