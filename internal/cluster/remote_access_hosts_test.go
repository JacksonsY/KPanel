package cluster

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoteAccessHostIsolationGlobalPrecedenceAndRestart(t *testing.T) {
	dir := t.TempDir()
	s := &Service{remoteAccess: openRemoteAccess(dir)}
	a, b := strings.Repeat("a", 32), strings.Repeat("b", 32)
	aTerminal := s.RemoteOperationLifetime("terminal", a)
	aFiles := s.RemoteOperationLifetime("files", a)
	bTerminal := s.RemoteOperationLifetime("terminal", b)
	value, err := s.UpdateHostRemoteAccess(a, "initial", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if aTerminal.Err() == nil || aFiles.Err() != nil || bTerminal.Err() != nil {
		t.Fatal("host or domain cancellation crossed its boundary")
	}
	if value.EffectiveTerminalEnabled || !value.FilesEnabled || !s.HostRemoteAccess(b).EffectiveTerminalEnabled {
		t.Fatal(value)
	}
	if _, err = s.TerminalOpen(context.Background(), a, TerminalOpenRequest{}); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatal(err)
	}
	_, release, err := s.BeginRemoteOperation(context.Background(), "files", a)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	global, err := s.UpdateRemoteAccess(value.ResourceVersion, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if aFiles.Err() == nil || bTerminal.Err() == nil {
		t.Fatal("global shutdown did not retire host epochs")
	}
	if !s.HostRemoteAccess(b).TerminalEnabled || s.HostRemoteAccess(b).EffectiveTerminalEnabled {
		t.Fatal("global state overwrote independent preferences")
	}
	if _, err = s.UpdateRemoteAccess(global.ResourceVersion, true, true); err != nil {
		t.Fatal(err)
	}
	if s.HostRemoteAccess(a).EffectiveTerminalEnabled || !s.HostRemoteAccess(a).EffectiveFilesEnabled || !s.HostRemoteAccess(b).EffectiveTerminalEnabled {
		t.Fatal("global reopening lost host preference")
	}
	if bTerminal.Err() == nil || aTerminal.Err() == nil {
		t.Fatal("reopening resurrected an old task")
	}
	restarted := &Service{remoteAccess: openRemoteAccess(dir)}
	if restarted.HostRemoteAccess(a) != s.HostRemoteAccess(a) || restarted.HostRemoteAccess(b) != s.HostRemoteAccess(b) {
		t.Fatal("host preferences lost after restart")
	}
	if _, err = s.UpdateHostRemoteAccess(a, value.ResourceVersion, true, true); !errors.Is(err, ErrConflict) {
		t.Fatal("stale host update overwrote global state", err)
	}
	_, releaseOther, err := s.BeginRemoteOperation(context.Background(), "terminal", b)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOther()
}

func TestRemoteAccessHostFilesRejectEveryTransportAndCancelOnlyThatHostBody(t *testing.T) {
	s := &Service{remoteAccess: openRemoteAccess(t.TempDir())}
	a, b := strings.Repeat("a", 32), strings.Repeat("b", 32)
	aReader, aWriter := io.Pipe()
	defer aWriter.Close()
	bReader, bWriter := io.Pipe()
	defer bWriter.Close()
	aResponse, err := s.remoteFile(context.Background(), func(context.Context) (*http.Response, error) { return &http.Response{Body: aReader}, nil }, a)
	if err != nil {
		t.Fatal(err)
	}
	defer aResponse.Body.Close()
	bResponse, err := s.remoteFile(context.Background(), func(context.Context) (*http.Response, error) { return &http.Response{Body: bReader}, nil }, b)
	if err != nil {
		t.Fatal(err)
	}
	defer bResponse.Body.Close()
	done := make(chan error, 1)
	go func() { _, err := aResponse.Body.Read(make([]byte, 1)); done <- err }()
	if _, err = s.UpdateHostRemoteAccess(a, "initial", true, false); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("disabled host body still readable")
		}
	case <-time.After(time.Second):
		t.Fatal("disabled host body stayed blocked")
	}
	written := make(chan error, 1)
	go func() { _, err := bWriter.Write([]byte("b")); written <- err }()
	if _, err = bResponse.Body.Read(make([]byte, 1)); err != nil {
		t.Fatal("other host body cancelled", err)
	}
	if err = <-written; err != nil {
		t.Fatal(err)
	}
	for _, open := range []func(context.Context, string, LightFileRequest) (*http.Response, error){s.OpenLightFile, s.OpenRemotePanelFile, s.OpenRemotePanelFileV1} {
		if _, err = open(context.Background(), a, LightFileRequest{}); !errors.Is(err, ErrRemoteAccessDisabled) {
			t.Fatal(err)
		}
	}
	if _, _, err = s.OpenRemoteFileV2(context.Background(), a, FederationFileOpenRequest{}); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatal(err)
	}
	if _, err = s.ManagedRemote(context.Background(), a, ManagedRequest{Tool: "host_file_list"}); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatal(err)
	}
}

func TestRemoteAccessHostSaveFailureKeepsEpochAndDisk(t *testing.T) {
	dir := t.TempDir()
	s := &Service{remoteAccess: openRemoteAccess(dir)}
	id := strings.Repeat("a", 32)
	epoch := s.RemoteOperationLifetime("files", id)
	s.remoteAccess.ops.syncFile = func(*os.File) error { return errors.New("disk full") }
	if _, err := s.UpdateHostRemoteAccess(id, "initial", true, false); !errors.Is(err, ErrRemoteAccessStorage) {
		t.Fatal(err)
	}
	if epoch.Err() != nil || !s.HostRemoteAccess(id).EffectiveFilesEnabled || !(&Service{remoteAccess: openRemoteAccess(dir)}).HostRemoteAccess(id).EffectiveFilesEnabled {
		t.Fatal("failed save published a host denial")
	}
}

func TestRemoteAccessRejectsMalformedHostPreferences(t *testing.T) {
	for _, hosts := range []string{`[{"hostId":"invalid","terminalEnabled":true,"filesEnabled":false}]`, `[{"hostId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","terminalEnabled":true}]`, `[{"hostId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","terminalEnabled":false,"filesEnabled":true},{"hostId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","terminalEnabled":true,"filesEnabled":true}]`} {
		dir := t.TempDir()
		raw := `{"schemaVersion":1,"terminalEnabled":true,"filesEnabled":true,"revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","hosts":` + hosts + `}`
		if err := os.WriteFile(filepath.Join(dir, remoteAccessFile), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if (&Service{remoteAccess: openRemoteAccess(dir)}).RemoteAccess().Available {
			t.Fatal("invalid host policy accepted")
		}
	}
}

func TestRemoteAccessHostCancellationDoesNotRejoinANewEpoch(t *testing.T) {
	s := &Service{remoteAccess: openRemoteAccess(t.TempDir())}
	id := strings.Repeat("a", 32)
	ctx, release, err := s.BeginRemoteOperation(context.Background(), "terminal", id)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	disabled, err := s.UpdateHostRemoteAccess(id, "initial", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateHostRemoteAccess(id, disabled.ResourceVersion, true, true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("host request was not cancelled")
	}
	if _, _, err = s.BeginRemoteOperation(ctx, "terminal", id); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatal("old host operation entered new epoch", err)
	}
}
