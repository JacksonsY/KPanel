//go:build !windows

package hostbackup

import "syscall"

const exportNonblock = syscall.O_NONBLOCK
