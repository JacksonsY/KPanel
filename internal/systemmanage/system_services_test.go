package systemmanage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

func serviceDetailFixture(active, enabled string) []byte {
	return []byte("Id=ssh.service\nDescription=Secure Shell\nLoadState=loaded\nActiveState=" + active + "\nSubState=running\nUnitFileState=" + enabled + "\nFragmentPath=/usr/lib/systemd/system/ssh.service\nMainPID=42\nCanReload=yes\nCanStart=yes\nCanStop=yes\nNeedDaemonReload=no\n")
}

const serviceUnitFixture = "[Service]\nExecStart=/usr/sbin/sshd -D\n"

func versionedServiceFixture(active, enabled string) contract.SystemServiceDetail {
	detail, _ := parseSystemServiceDetail("ssh.service", serviceDetailFixture(active, enabled))
	bindSystemServiceConfiguration(&detail, []byte(serviceUnitFixture))
	return detail
}

func serviceTestManager(t *testing.T, runner Runner) *Manager {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("systemd operations require Linux; pure contract/parser tests run everywhere")
	}
	m, _, _, _ := testManager(t, runner)
	if err := os.MkdirAll(filepath.Join(m.runRoot, "systemd", "system"), 0755); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSystemServicesMergeInstalledUnloadedAndRunning(t *testing.T) {
	files := []byte("ssh.service enabled enabled\nidle.service disabled enabled\n")
	units := []byte("ssh.service loaded active running Secure Shell\ntransient.service loaded activating start Starting Worker\n")
	services, truncated := parseSystemServices(files, units)
	if truncated || len(services) != 3 {
		t.Fatalf("services=%#v truncated=%v", services, truncated)
	}
	if services[0].Name != "idle.service" || services[0].LoadState != "unloaded" || services[0].UnitFileState != "disabled" {
		t.Fatalf("unloaded service missing: %#v", services[0])
	}
	if services[0].ActiveState != "unknown" {
		t.Fatal("installed-only unit falsely reported inactive")
	}
	if services[1].Name != "ssh.service" || services[1].ActiveState != "active" || services[1].Description != "Secure Shell" || services[1].UnitFileState != "enabled" {
		t.Fatalf("running service=%#v", services[1])
	}
	if services[2].Name != "transient.service" || services[2].UnitFileState != "" {
		t.Fatalf("transient=%#v", services[2])
	}
}

func TestSystemServicesBoundEntriesAndDoNotParsePartialLines(t *testing.T) {
	var files strings.Builder
	for i := 0; i <= contract.SystemServiceMaxEntries; i++ {
		fmt.Fprintf(&files, "worker-%04d.service disabled enabled\n", i)
	}
	services, truncated := parseSystemServices([]byte(files.String()), nil)
	if len(services) != contract.SystemServiceMaxEntries || !truncated {
		t.Fatalf("entries=%d truncated=%v", len(services), truncated)
	}
	services, _ = parseSystemServices([]byte("ssh.service enabled"), nil)
	if len(services) != 0 {
		t.Fatal("partial stdout record accepted")
	}
}

func TestSystemServiceDetailVersionsRelevantStateNotPID(t *testing.T) {
	before, err := parseSystemServiceDetail("ssh.service", serviceDetailFixture("active", "enabled"))
	if err != nil {
		t.Fatal(err)
	}
	pidOnly, err := parseSystemServiceDetail("ssh.service", []byte(strings.ReplaceAll(string(serviceDetailFixture("active", "enabled")), "MainPID=42", "MainPID=999")))
	if err != nil || before.ResourceVersion != pidOnly.ResourceVersion {
		t.Fatalf("PID-only version changed: %v", err)
	}
	after, err := parseSystemServiceDetail("ssh.service", serviceDetailFixture("inactive", "disabled"))
	if err != nil || before.ResourceVersion == after.ResourceVersion {
		t.Fatalf("state change did not change version: %v", err)
	}
	for _, output := range [][]byte{[]byte("Id=ssh.service\n"), append(serviceDetailFixture("active", "enabled"), []byte("Id=other.service\n")...), []byte(strings.ReplaceAll(string(serviceDetailFixture("active", "enabled")), "LoadState=loaded", "LoadState=not-found"))} {
		if _, err := parseSystemServiceDetail("ssh.service", output); err == nil {
			t.Fatal("invalid service detail accepted")
		}
	}
}

func TestSystemServiceActionQueuesFixedLifecycleAndPreservesUnconfirmedState(t *testing.T) {
	runner := &fakeRunner{}
	runner.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), " cat ") {
			return []byte(serviceUnitFixture), nil
		}
		if strings.Contains(strings.Join(args, " "), " show ") {
			return serviceDetailFixture("inactive", "disabled"), nil
		}
		if name != "systemctl" || strings.Join(args, " ") != "--no-pager --no-ask-password --no-block start -- ssh.service" {
			t.Fatalf("unexpected mutation %s %v", name, args)
		}
		return nil, nil
	}
	m := serviceTestManager(t, runner)
	detail := versionedServiceFixture("inactive", "disabled")
	result, err := m.ExecuteSystemServiceAction(context.Background(), contract.SystemServiceActionRequest{Name: "ssh.service", Action: "start", ExpectedResourceVersion: detail.ResourceVersion})
	if err != nil || !result.Accepted || result.Service == nil || result.Service.ActiveState != "inactive" {
		t.Fatalf("queued action falsely completed: %#v %v", result, err)
	}
	if len(runner.commands) != 5 {
		t.Fatalf("commands=%#v", runner.commands)
	}
	for _, limit := range runner.resourceLimits {
		if limit != 16<<10 && limit != 256<<10 {
			t.Errorf("unbounded command limit=%d", limit)
		}
	}
}

func TestSystemServiceActionVersionConflictAndBusyNeverMutate(t *testing.T) {
	runner := &fakeRunner{run: func(context.Context, string, ...string) ([]byte, error) {
		return serviceDetailFixture("active", "enabled"), nil
	}}
	m := serviceTestManager(t, runner)
	input := contract.SystemServiceActionRequest{Name: "ssh.service", Action: "stop", ExpectedResourceVersion: strings.Repeat("a", 64)}
	if _, err := m.ExecuteSystemServiceAction(context.Background(), input); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	if len(runner.commands) != 2 {
		t.Fatalf("stale request mutated: %#v", runner.commands)
	}
	m.servicesMu.Lock()
	_, err := m.ExecuteSystemServiceAction(context.Background(), input)
	m.servicesMu.Unlock()
	if !errors.Is(err, ErrSystemServicesBusy) || len(runner.commands) != 2 {
		t.Fatalf("busy err=%v calls=%v", err, runner.commands)
	}
}

func TestSystemServiceAutostartReadbackAndNoNow(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			mutated := false
			runner := &fakeRunner{run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				joined := strings.Join(args, " ")
				if strings.Contains(joined, " cat ") {
					return []byte(serviceUnitFixture), nil
				}
				if strings.Contains(joined, " show ") {
					state := "disabled"
					if mutated && changed {
						state = "enabled"
					}
					return serviceDetailFixture("inactive", state), nil
				}
				if joined != "--no-pager --no-ask-password enable -- ssh.service" {
					t.Fatalf("unexpected enable argv %q", joined)
				}
				mutated = true
				return nil, nil
			}}
			m := serviceTestManager(t, runner)
			detail := versionedServiceFixture("inactive", "disabled")
			result, err := m.ExecuteSystemServiceAction(context.Background(), contract.SystemServiceActionRequest{Name: "ssh.service", Action: "enable", ExpectedResourceVersion: detail.ResourceVersion})
			if changed {
				if err != nil || result.Accepted || result.Service.UnitFileState != "enabled" {
					t.Fatalf("enable=%#v err=%v", result, err)
				}
			} else if !errors.Is(err, ErrNeedsAttention) {
				t.Fatalf("inconsistent readback reported success: %v", err)
			}
		})
	}
}

func TestSystemServicesUnsupportedAndDisabledFailBeforeCommands(t *testing.T) {
	runner := &fakeRunner{}
	m := NewManager(Config{Runner: runner, Enabled: false, EffectiveUID: func() int { return 0 }, RunRoot: t.TempDir()})
	input := contract.SystemServiceActionRequest{Name: "ssh.service", Action: "restart", ExpectedResourceVersion: strings.Repeat("a", 64)}
	if _, err := m.ExecuteSystemServiceAction(context.Background(), input); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled=%v", err)
	}
	snapshot, err := m.SystemServices(context.Background())
	if err != nil || snapshot.Supported || snapshot.Reason == "" || len(runner.commands) != 0 {
		t.Fatalf("unsupported=%#v err=%v commands=%v", snapshot, err, runner.commands)
	}
	input.Name = "*.service"
	if _, err := m.ExecuteSystemServiceAction(context.Background(), input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unsafe name=%v", err)
	}
}

func TestSystemServiceJournalIsBoundedRedactedAndExact(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "journalctl" || strings.Join(args, " ") != "--no-pager --quiet --output=short-iso --lines=101 --unit=ssh.service" {
			t.Fatalf("unexpected journal command %s %v", name, args)
		}
		return []byte("password=hunter2 Bearer abc.def\n"), nil
	}}
	m := serviceTestManager(t, runner)
	logs, err := m.SystemServiceLogs(context.Background(), "ssh.service")
	if err != nil || strings.Contains(logs.Content, "hunter2") || strings.Contains(logs.Content, "abc.def") || !strings.Contains(logs.Content, "REDACTED") {
		t.Fatalf("logs=%#v err=%v", logs, err)
	}
	if runner.resourceLimits[0] != contract.SystemServiceLogOutputLimit {
		t.Fatal("journal output is not bounded")
	}
}

func TestSystemServiceActionTimeoutDoesNotRetry(t *testing.T) {
	calls := 0
	runner := &fakeRunner{run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), " cat ") {
			return []byte(serviceUnitFixture), nil
		}
		if strings.Contains(strings.Join(args, " "), " show ") {
			return serviceDetailFixture("active", "enabled"), nil
		}
		calls++
		return nil, context.DeadlineExceeded
	}}
	m := serviceTestManager(t, runner)
	detail := versionedServiceFixture("active", "enabled")
	_, err := m.ExecuteSystemServiceAction(context.Background(), contract.SystemServiceActionRequest{Name: "ssh.service", Action: "restart", ExpectedResourceVersion: detail.ResourceVersion})
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
		t.Fatalf("timeout=%v mutation calls=%d", err, calls)
	}
}

func TestSystemServiceConfigChangeAfterDaemonReloadConflicts(t *testing.T) {
	configuration := serviceUnitFixture
	mutations := 0
	runner := &fakeRunner{run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, " cat ") {
			return []byte(configuration), nil
		}
		if strings.Contains(joined, " show ") {
			return serviceDetailFixture("active", "enabled"), nil
		}
		mutations++
		return nil, nil
	}}
	m := serviceTestManager(t, runner)
	before, err := m.SystemService(context.Background(), "ssh.service")
	if err != nil {
		t.Fatal(err)
	}
	// Properties, fragment path and NeedDaemonReload=no remain identical after
	// an external edit followed by daemon-reload. The raw file hash must differ.
	configuration += "\n# /etc/systemd/system/ssh.service.d/override.conf\n[Service]\nExecStart=\nExecStart=/usr/local/bin/new-sshd --token SECRET_CONFIG\n"
	_, err = m.ExecuteSystemServiceAction(context.Background(), contract.SystemServiceActionRequest{Name: "ssh.service", Action: "restart", ExpectedResourceVersion: before.ResourceVersion})
	if !errors.Is(err, ErrConflict) || mutations != 0 {
		t.Fatalf("changed configuration restarted: err=%v mutations=%d", err, mutations)
	}
	after, err := m.SystemService(context.Background(), "ssh.service")
	if err != nil || before.ResourceVersion == after.ResourceVersion || strings.Contains(fmt.Sprint(after), "SECRET_CONFIG") {
		t.Fatalf("configuration version is not isolated: %#v %v", after, err)
	}
}

func TestSystemServiceFilelessConfigurationAllowsPIDChurnAndVersionsCommands(t *testing.T) {
	runtimeMetadata := " ; start_time=[n/a] ; stop_time=[n/a] ; pid=42 ; code=(null) ; status=0/0"
	command := "/usr/bin/worker"
	runner := &fakeRunner{run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, " cat ") {
			t.Fatal("fileless unit sent to systemctl cat")
		}
		if strings.Contains(joined, "--property="+serviceRuntimeConfigProperties) {
			return []byte("ExecStart={ path=" + command + " ; argv[]=" + command + runtimeMetadata + " }\nEnvironment=TOKEN=SECRET_CONFIG\n"), nil
		}
		return []byte(strings.ReplaceAll(string(serviceDetailFixture("active", "static")), "/usr/lib/systemd/system/ssh.service", "")), nil
	}}
	m := serviceTestManager(t, runner)
	before, err := m.SystemService(context.Background(), "ssh.service")
	if err != nil {
		t.Fatal(err)
	}
	runtimeMetadata = " ; start_time=[Sun 2026-10-11 12:00:00 CST] ; stop_time=[n/a] ; pid=999 ; code=exited ; status=0"
	after, err := m.SystemService(context.Background(), "ssh.service")
	if err != nil || before.ResourceVersion != after.ResourceVersion {
		t.Fatalf("PID churn conflicts: %v", err)
	}
	command = "/usr/bin/new-worker"
	after, err = m.SystemService(context.Background(), "ssh.service")
	if err != nil || before.ResourceVersion == after.ResourceVersion {
		t.Fatalf("fileless command update ignored: %v", err)
	}
}

func TestSystemServiceMaskedUnitDoesNotRequireFragmentContent(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, " cat ") {
			t.Fatal("masked unit sent to cat")
		}
		if strings.Contains(joined, "--property="+serviceRuntimeConfigProperties) {
			return []byte("ExecStart=\n"), nil
		}
		return []byte(strings.ReplaceAll(string(serviceDetailFixture("inactive", "masked")), "LoadState=loaded", "LoadState=masked")), nil
	}}
	m := serviceTestManager(t, runner)
	detail, err := m.SystemService(context.Background(), "ssh.service")
	if err != nil || detail.UnitFileState != "masked" || detail.ResourceVersion == "" {
		t.Fatalf("masked detail=%#v err=%v", detail, err)
	}
}

func TestSystemServiceConfigurationFailureNeverMutates(t *testing.T) {
	mutations := 0
	runner := &fakeRunner{run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, " cat ") {
			return nil, errResourceOutputTooLarge
		}
		if strings.Contains(joined, " show ") {
			return serviceDetailFixture("active", "enabled"), nil
		}
		mutations++
		return nil, nil
	}}
	m := serviceTestManager(t, runner)
	before := versionedServiceFixture("active", "enabled")
	_, err := m.ExecuteSystemServiceAction(context.Background(), contract.SystemServiceActionRequest{Name: "ssh.service", Action: "restart", ExpectedResourceVersion: before.ResourceVersion})
	if err == nil || mutations != 0 {
		t.Fatalf("unreadable configuration mutated: err=%v calls=%d", err, mutations)
	}
}

func TestSystemServiceRuntimeDigestPreservesNonExecutionFields(t *testing.T) {
	tuple := " ; start_time=[n/a] ; stop_time=[n/a] ; pid=42 ; code=(null) ; status=0/0 }"
	input := "ExecCondition={ path=/usr/bin/test" + tuple + "\nEnvironment=METADATA={" + tuple + "\n"
	stable := string(stableServiceRuntimeConfiguration([]byte(input)))
	if !strings.Contains(stable, "Environment=METADATA={"+tuple) || !strings.Contains(stable, "ExecCondition={ path=/usr/bin/test }\n") {
		t.Fatalf("runtime digest removed configuration: %q", stable)
	}
}
