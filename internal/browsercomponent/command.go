package browsercomponent

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"
)

type commandRunner struct{}

func (commandRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }

type boundedOutput struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.b.Len()+len(p) > 64<<10 {
		return 0, errors.New("browser command output limit")
	}
	return b.b.Write(p)
}
func (commandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, name, args...)
	c.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "HOME=/root", "LANG=C", "LC_ALL=C"}
	c.WaitDelay = 2 * time.Second
	out := &boundedOutput{}
	c.Stdout = out
	c.Stderr = out
	if err := c.Run(); err != nil {
		return out.b.Bytes(), errors.New("browser component command failed: " + out.b.String())
	}
	return out.b.Bytes(), nil
}
