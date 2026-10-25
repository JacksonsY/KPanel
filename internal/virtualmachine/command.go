// Package virtualmachine adapts the host's libvirt state. It owns no shadow
// inventory and opens no listener. Only the Agent may call its write methods.
package virtualmachine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalid     = errors.New("invalid virtual machine request")
	ErrConflict    = errors.New("virtual machine state changed")
	ErrUnsupported = errors.New("virtual machine runtime unavailable")
	ErrBusy        = errors.New("virtual machine task in progress")
	ErrDisabled    = errors.New("host writes disabled")
)

type Runner interface {
	Run(context.Context, io.Reader, string, ...string) ([]byte, error)
	LookPath(string) (string, error)
}

type CommandRunner struct{}

func (CommandRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }

type boundedBuffer struct {
	mu    sync.Mutex
	data  bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(p) > b.limit-b.data.Len() {
		return 0, errors.New("command output exceeds limit")
	}
	return b.data.Write(p)
}

func (CommandRunner) Run(ctx context.Context, input io.Reader, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, name, args...)
	c.Stdin = input
	// Do not inherit libvirt URIs, credential helpers, Python injection paths or
	// dynamic-linker variables from the service's environment.
	c.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "HOME=/root", "LC_ALL=C", "LANG=C", "DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a", "VIRTINST_OSINFO_DISABLE_REQUIRE=1"}
	c.WaitDelay = 2 * time.Second
	out := &boundedBuffer{limit: 2 << 20}
	errout := &boundedBuffer{limit: 16 << 10}
	c.Stdout, c.Stderr = out, errout
	if err := c.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		detail := strings.TrimSpace(errout.data.String())
		if len(detail) > 1024 {
			detail = detail[:1024]
		}
		return nil, fmt.Errorf("%s: %w", detail, err)
	}
	return out.data.Bytes(), nil
}

func (s *Service) virsh(ctx context.Context, args ...string) ([]byte, error) {
	return s.runner.Run(ctx, nil, "virsh", append([]string{"--connect", "qemu:///system"}, args...)...)
}

func (s *Service) virshXML(ctx context.Context, xml string, args ...string) ([]byte, error) {
	return s.runner.Run(ctx, strings.NewReader(xml), "virsh", append([]string{"--connect", "qemu:///system"}, args...)...)
}

func privateDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 || !rootOwned(info) {
		return errors.New("task directory must be a private real directory")
	}
	return nil
}
