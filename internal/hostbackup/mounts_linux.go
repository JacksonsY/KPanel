package hostbackup

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Resolve only the pinned parent descriptor for the mount-table check. Actual
// deletion still goes through Root, so a later pathname swap cannot redirect it.
func restoreNoNestedMounts(parent *os.Root, name string) error {
	f, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer f.Close()
	path, err := os.Readlink("/proc/self/fd/" + strconv.FormatUint(uint64(f.Fd()), 10))
	if err != nil {
		return err
	}
	return noNestedMounts(filepath.Join(path, name))
}

// Directory swaps cannot safely cross a mounted filesystem. A separate
// filesystem needs its own snapshot adapter before it can participate.
func noNestedMounts(target string) error {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	target = filepath.Clean(target)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 {
			return errors.New("invalid mount table")
		}
		point := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(fields[4])
		if within(filepath.ToSlash(target), point) {
			return errors.New("data root contains a mounted filesystem")
		}
	}
	return scanner.Err()
}
