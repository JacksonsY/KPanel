//go:build linux

package filemanager

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func fileExtendedAttributes(file *os.File) (map[string][]byte, error) {
	size, err := unix.Flistxattr(int(file.Fd()), nil)
	if errors.Is(err, unix.ENOTSUP) {
		return map[string][]byte{}, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]byte, size)
	count, err := unix.Flistxattr(int(file.Fd()), names)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, name := range strings.Split(string(names[:count]), "\x00") {
		if name == "" {
			continue
		}
		size, err := unix.Fgetxattr(int(file.Fd()), name, nil)
		if err != nil {
			return nil, err
		}
		value := make([]byte, size)
		count, err := unix.Fgetxattr(int(file.Fd()), name, value)
		if err != nil {
			return nil, err
		}
		out[name] = value[:count]
	}
	return out, nil
}

// Exact replacement removes inherited ACLs/default ACLs absent on the source.
// Unsupported destinations must reject nonempty source metadata, never publish
// a weaker permission set. Capabilities are restored only for unchanged bytes.
func preserveFileExtendedAttributes(target, source *os.File, sameContent bool) error {
	attrs, err := fileExtendedAttributes(source)
	if err != nil {
		return err
	}
	if !sameContent {
		delete(attrs, "security.capability")
	}
	inherited, err := fileExtendedAttributes(target)
	if err != nil {
		return err
	}
	for name := range inherited {
		if _, ok := attrs[name]; !ok {
			if err := unix.Fremovexattr(int(target.Fd()), name); err != nil && !errors.Is(err, unix.ENODATA) {
				return err
			}
		}
	}
	for name, value := range attrs {
		if name == "security.capability" {
			continue
		}
		if err := unix.Fsetxattr(int(target.Fd()), name, value, 0); err != nil {
			return err
		}
	}
	if capability, ok := attrs["security.capability"]; ok {
		return unix.Fsetxattr(int(target.Fd()), "security.capability", capability, 0)
	}
	return nil
}
