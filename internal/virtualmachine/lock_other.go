//go:build !linux

package virtualmachine

func lockDirectory(string, string) (func(), error) { return nil, ErrUnsupported }
