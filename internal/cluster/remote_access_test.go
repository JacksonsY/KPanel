package cluster

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/terminal"
)

func TestRemoteAccessDefaultsPersistenceAndEpochIsolation(t *testing.T) {
	dir := t.TempDir()
	s := &Service{remoteAccess: openRemoteAccess(dir)}
	initial := s.RemoteAccess()
	if !initial.Available || !initial.TerminalEnabled || !initial.FilesEnabled {
		t.Fatal(initial)
	}
	if _, err := os.Stat(filepath.Join(dir, remoteAccessFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read created a settings file", err)
	}
	term, stop, err := s.BeginRemoteOperation(context.Background(), "terminal")
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	file, stopFile, err := s.BeginRemoteOperation(context.Background(), "files")
	if err != nil {
		t.Fatal(err)
	}
	defer stopFile()
	changed, err := s.UpdateRemoteAccess(initial.ResourceVersion, false, true)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-term.Done():
	case <-time.After(time.Second):
		t.Fatal("active terminal context not cancelled")
	}
	if file.Err() != nil {
		t.Fatal("terminal switch cancelled independent file access")
	}
	if _, err = s.UpdateRemoteAccess(initial.ResourceVersion, true, true); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	restarted := &Service{remoteAccess: openRemoteAccess(dir)}
	if restarted.RemoteAccess() != changed {
		t.Fatalf("policy lost on restart: %#v", restarted.RemoteAccess())
	}
	if _, err = s.UpdateRemoteAccess(changed.ResourceVersion, true, true); err != nil {
		t.Fatal(err)
	}
	if term.Err() == nil {
		t.Fatal("old request resurrected after enabling")
	}
	current, release, err := s.BeginRemoteOperation(context.Background(), "terminal")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if current.Err() != nil {
		t.Fatal("new interval unavailable")
	}
	if _, _, err := s.BeginRemoteOperation(term, "terminal"); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatalf("nested old operation joined a new interval: %v", err)
	}
}

func TestRemoteAccessCorruptSettingsFailClosedWithoutReplacingEvidence(t *testing.T) {
	for _, raw := range []string{`{}`, `{"schemaVersion":1,"terminalEnabled":true,"filesEnabled":true}`, `{"schemaVersion":2}`, `{"schemaVersion":1,"terminalEnabled":true,"terminalEnabled":false}`, `null`} {
		t.Run(raw, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, remoteAccessFile)
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			s := &Service{remoteAccess: openRemoteAccess(dir)}
			if s.RemoteAccess().Available || s.RemoteAccess().FilesEnabled || s.RemoteAccess().TerminalEnabled {
				t.Fatal(s.RemoteAccess())
			}
			if _, err := s.UpdateRemoteAccess("", true, true); !errors.Is(err, ErrRemoteAccessStorage) {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(path)
			if string(got) != raw {
				t.Fatal("corrupt evidence overwritten")
			}
		})
	}
	// Interrupted replacement must not restore default-on through an absent target.
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, remoteAccessFile)+".previous", []byte("saved state"), 0o600)
	if (&Service{remoteAccess: openRemoteAccess(dir)}).RemoteAccess().Available {
		t.Fatal("interrupted replacement became default-on")
	}
}

func TestRemoteAccessFailedSavePreservesMemoryDiskAndConnections(t *testing.T) {
	dir := t.TempDir()
	s := &Service{remoteAccess: openRemoteAccess(dir)}
	initial := s.RemoteAccess()
	ctx, cancel, err := s.BeginRemoteOperation(context.Background(), "files")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	s.remoteAccess.ops.syncFile = func(*os.File) error { return errors.New("disk full") }
	if _, err = s.UpdateRemoteAccess(initial.ResourceVersion, true, false); !errors.Is(err, ErrRemoteAccessStorage) {
		t.Fatal(err)
	}
	if s.RemoteAccess() != initial || ctx.Err() != nil {
		t.Fatal("failed persistence published a policy")
	}
	if (&Service{remoteAccess: openRemoteAccess(dir)}).RemoteAccess() != initial {
		t.Fatal("failed write changed restart policy")
	}
}

func TestRemoteAccessBlocksEveryOutgoingTransportWithoutChangingLocalCapabilities(t *testing.T) {
	s := &Service{remoteAccess: openRemoteAccess(t.TempDir())}
	if _, err := s.UpdateRemoteAccess("initial", false, false); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := s.TerminalOpen(ctx, "remote", TerminalOpenRequest{}); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatal(err)
	}
	if _, err := s.TerminalOutput(ctx, "remote", TerminalOutputRequest{}); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatal(err)
	}
	if err := s.TerminalInput(ctx, "remote", TerminalInputRequest{}); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatal(err)
	}
	if err := s.TerminalResize(ctx, "remote", TerminalResizeRequest{}); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatal(err)
	}
	if _, err := s.BeginTerminalInput(ctx, "remote", "session", terminal.InputFrame{}); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatal(err)
	}
	for _, open := range []func(context.Context, string, LightFileRequest) (*http.Response, error){s.OpenLightFile, s.OpenRemotePanelFile, s.OpenRemotePanelFileV1} {
		if _, err := open(ctx, "remote", LightFileRequest{}); !errors.Is(err, ErrRemoteAccessDisabled) {
			t.Fatal(err)
		}
	}
	if _, _, err := s.OpenRemoteFileV2(ctx, "remote", FederationFileOpenRequest{}); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatal(err)
	}
	if _, err := s.ManagedRemote(ctx, "remote", ManagedRequest{Tool: "host_file_list"}); !errors.Is(err, ErrRemoteAccessDisabled) {
		t.Fatal(err)
	}
	local := Host{IsLocal: true, TerminalAvailable: true, FileManagementAvailable: true, Scope: SummaryTerminalFilesScope}
	if s.applyRemoteAccess(local) != local {
		t.Fatal("local capabilities changed")
	}
	remote := local
	remote.IsLocal = false
	remote = s.applyRemoteAccess(remote)
	if remote.TerminalAvailable || remote.FileManagementAvailable || !remote.RemoteFilesDisabled || remote.Scope != local.Scope {
		t.Fatal(remote)
	}
}

func TestRemoteAccessCancelsReturnedFileBody(t *testing.T) {
	s := &Service{remoteAccess: openRemoteAccess(t.TempDir())}
	reader, writer := io.Pipe()
	defer writer.Close()
	response, err := s.remoteFile(context.Background(), func(context.Context) (*http.Response, error) { return &http.Response{Body: reader}, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	done := make(chan error, 1)
	go func() { _, err := response.Body.Read(make([]byte, 1)); done <- err }()
	if _, err = s.UpdateRemoteAccess("initial", true, false); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("file read succeeded after cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("returned body remained blocked")
	}
}
