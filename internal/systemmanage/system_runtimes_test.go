package systemmanage

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type runtimeProbeRunner struct {
	paths map[string]string
	probe func(context.Context, string) ([]byte, []byte, error)
	calls int
}

func (r *runtimeProbeRunner) LookPath(name string) (string, error) {
	if value := r.paths[name]; value != "" {
		return value, nil
	}
	return "", errors.New("not found")
}

func (*runtimeProbeRunner) Run(context.Context, string, ...string) ([]byte, error) {
	panic("runtime versions must use bounded resource commands")
}

func (r *runtimeProbeRunner) RunResource(ctx context.Context, limit int, input []byte, name string, args ...string) ([]byte, []byte, error) {
	r.calls++
	if name != "env" || limit != 4096 || input != nil || len(args) != 6 ||
		!reflect.DeepEqual(args[:4], []string{"-i", "LC_ALL=C", "LANG=C", "GOTOOLCHAIN=local"}) {
		return nil, nil, errors.New("unsafe version invocation")
	}
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > time.Second {
		return nil, nil, errors.New("missing probe deadline")
	}
	return r.probe(ctx, args[4]+" "+args[5])
}

func TestSystemRuntimesDetectVersionsAndFallbackPaths(t *testing.T) {
	runner := &runtimeProbeRunner{paths: map[string]string{
		"python": "/usr/bin/python", "nodejs": "/usr/bin/nodejs", "java": "/usr/bin/java", "go": "/usr/local/go/bin/go",
	}, probe: func(_ context.Context, command string) ([]byte, []byte, error) {
		switch command {
		case "/usr/bin/python --version":
			return []byte("Python 3.13.7\n"), nil, nil
		case "/usr/bin/nodejs --version":
			return []byte("v24.8.0\n"), nil, nil
		case "/usr/bin/java -version":
			return nil, []byte("openjdk version \"21.0.8\" 2025-07-15\nOpenJDK Runtime Environment\n"), nil
		case "/usr/local/go/bin/go version":
			return []byte("go version go1.25.1 linux/amd64\n"), nil, nil
		}
		return nil, nil, errors.New("unexpected command")
	}}
	items := (&Manager{runner: runner}).readSystemRuntimes(context.Background())
	wantVersions := []string{"3.13.7", "24.8.0", "21.0.8", "1.25.1"}
	for index, item := range items {
		if !item.Installed || item.Version != wantVersions[index] || item.Path == "" {
			t.Fatalf("runtime %d: %#v", index, item)
		}
	}
	if len(items) != 4 || runner.calls != 4 {
		t.Fatalf("items=%v calls=%d", items, runner.calls)
	}
}

func TestSystemRuntimesMissingAndCancelledNeverExecute(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		runner := &runtimeProbeRunner{paths: map[string]string{}}
		ctx, cancel := context.WithCancel(context.Background())
		if cancelled {
			runner.paths["python3"] = "/usr/bin/python3"
			cancel()
		}
		items := (&Manager{runner: runner}).readSystemRuntimes(ctx)
		cancel()
		if len(items) != 4 || runner.calls != 0 || items[0].Installed != cancelled || items[0].Version != "" {
			t.Fatalf("cancelled=%t items=%v calls=%d", cancelled, items, runner.calls)
		}
	}
}

func TestSystemRuntimesVersionFailuresDoNotClaimMissingOrExposeOutput(t *testing.T) {
	for _, mode := range []string{"failed", "timeout", "oversized", "malformed", "relative-path"} {
		t.Run(mode, func(t *testing.T) {
			path := "/usr/bin/python3"
			if mode == "relative-path" {
				path = "./python3"
			}
			runner := &runtimeProbeRunner{paths: map[string]string{"python3": path}, probe: func(context.Context, string) ([]byte, []byte, error) {
				switch mode {
				case "failed":
					return nil, nil, errors.New("private diagnostic")
				case "timeout":
					return nil, nil, context.DeadlineExceeded
				case "oversized":
					return []byte("Python 3.13.7\n" + strings.Repeat("x", 4096)), nil, nil
				default:
					return []byte("<script>private output</script>"), nil, nil
				}
			}}
			item := (&Manager{runner: runner}).readSystemRuntimes(context.Background())[0]
			if item.Installed != (mode != "relative-path") || item.Version != "" {
				t.Fatalf("runtime=%#v", item)
			}
		})
	}
}
