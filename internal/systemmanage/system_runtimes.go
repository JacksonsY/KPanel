package systemmanage

import (
	"context"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

var systemRuntimeCatalog = []struct {
	id       string
	commands []string
	args     []string
	version  *regexp.Regexp
}{
	{"python", []string{"python3", "python"}, []string{"--version"}, regexp.MustCompile(`(?m)^Python ([0-9][0-9A-Za-z.+_-]{0,63})\b`)},
	{"node", []string{"node", "nodejs"}, []string{"--version"}, regexp.MustCompile(`(?m)^v([0-9][0-9A-Za-z.+_-]{0,63})\b`)},
	{"java", []string{"java"}, []string{"-version"}, regexp.MustCompile(`(?m)^(?:openjdk|java) version "([0-9][0-9A-Za-z.+_-]{0,63})"`)},
	{"go", []string{"go"}, []string{"version"}, regexp.MustCompile(`(?m)^go version go([0-9][0-9A-Za-z.+_-]{0,63})\b`)},
}

type runtimeVersionRunner interface {
	RunRuntimeVersion(context.Context, string, ...string) ([]byte, []byte, error)
}

// A version shim may leave inherited pipes open after its parent exits. Bound
// that drain separately without changing background resource action semantics.
func (runner commandRunner) RunRuntimeVersion(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	return runner.runResource(ctx, 4096, nil, 250*time.Millisecond, name, args...)
}

func (m *Manager) runRuntimeVersion(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	if runner, ok := m.runner.(runtimeVersionRunner); ok {
		return runner.RunRuntimeVersion(ctx, name, args...)
	}
	return m.runResourceCommand(ctx, 4096, name, args...)
}

// Only fixed version commands run, with bounded output, deadlines and no inherited
// startup options or automatic Go toolchain downloads. Package write hashes stay unchanged.
func (m *Manager) readSystemRuntimes(ctx context.Context) []contract.SystemRuntime {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	items := make([]contract.SystemRuntime, 0, len(systemRuntimeCatalog))
	for _, definition := range systemRuntimeCatalog {
		item := contract.SystemRuntime{ID: definition.id}
		for _, command := range definition.commands {
			executable, err := m.runner.LookPath(command)
			if err != nil || len(executable) > 512 || (!filepath.IsAbs(executable) && !path.IsAbs(executable)) {
				continue
			}
			item.Installed, item.Path = true, executable
			if ctx.Err() == nil {
				probeCtx, stop := context.WithTimeout(ctx, time.Second)
				args := append([]string{"-i", "LC_ALL=C", "LANG=C", "GOTOOLCHAIN=local", executable}, definition.args...)
				stdout, stderr, runErr := m.runRuntimeVersion(probeCtx, "env", args...)
				stop()
				if runErr == nil && len(stdout) <= 4096 && len(stderr) <= 4096 {
					output := append(append([]byte(nil), stdout...), '\n')
					output = append(output, stderr...)
					if match := definition.version.FindSubmatch(output); len(match) == 2 {
						item.Version = string(match[1])
					}
				}
			}
			break
		}
		items = append(items, item)
	}
	return items
}
