package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kejilion/kejilion-panel/internal/contract"
)

const remoteAccessFile = "cluster-remote-access.json"

var (
	ErrRemoteAccessDisabled = errors.New("remote operation disabled by this center")
	ErrRemoteAccessStorage  = errors.New("remote access settings unavailable")
)

// RemoteAccess is a center preference, independent of peer grants. An older
// binary ignores the sidecar; downgrading therefore does not enforce this policy.
type RemoteAccess struct {
	TerminalEnabled bool   `json:"terminalEnabled"`
	FilesEnabled    bool   `json:"filesEnabled"`
	ResourceVersion string `json:"resourceVersion"`
	Available       bool   `json:"available"`
}

type remoteAccessDisk struct {
	SchemaVersion   int                    `json:"schemaVersion"`
	TerminalEnabled *bool                  `json:"terminalEnabled"`
	FilesEnabled    *bool                  `json:"filesEnabled"`
	Revision        string                 `json:"revision"`
	Hosts           []remoteHostPreference `json:"hosts,omitempty"`
}

type remoteAccessControl struct {
	mu              sync.RWMutex
	path            string
	value           RemoteAccess
	ops             atomicFileOpsV2
	terminalContext context.Context
	stopTerminal    context.CancelFunc
	fileContext     context.Context
	stopFiles       context.CancelFunc
	hosts           map[string]remoteHostPreference
	hostEpochs      map[string]*remoteHostEpochs
}

func openRemoteAccess(directory string) *remoteAccessControl {
	c := &remoteAccessControl{path: filepath.Join(directory, remoteAccessFile), ops: defaultAtomicFileOpsV2(), hosts: make(map[string]remoteHostPreference), hostEpochs: make(map[string]*remoteHostEpochs)}
	c.terminalContext, c.stopTerminal = context.WithCancel(context.Background())
	c.fileContext, c.stopFiles = context.WithCancel(context.Background())
	c.value = RemoteAccess{TerminalEnabled: true, FilesEnabled: true, ResourceVersion: "initial", Available: true}
	// Do not recover an older backup here: that could undo an explicit denial.
	// Corrupt/incomplete settings disable remote operations without taking down telemetry.
	content, err := readRegularFileV2(c.path, 1<<20, true)
	_, previousErr := os.Lstat(c.path + ".previous")
	if errors.Is(err, os.ErrNotExist) {
		// A crash between the two renames must not look like an untouched install.
		if errors.Is(previousErr, os.ErrNotExist) {
			return c
		}
	}
	var disk remoteAccessDisk
	if err != nil || !errors.Is(previousErr, os.ErrNotExist) || decodeStrictJSONV2(content, &disk) != nil || disk.SchemaVersion != 1 ||
		disk.TerminalEnabled == nil || disk.FilesEnabled == nil || !validID(disk.Revision) || !validRemoteHostPreferences(disk.Hosts) {
		c.value = RemoteAccess{Available: false}
		c.stopTerminal()
		c.stopFiles()
		return c
	}
	c.value = RemoteAccess{TerminalEnabled: *disk.TerminalEnabled, FilesEnabled: *disk.FilesEnabled, ResourceVersion: disk.Revision, Available: true}
	for _, preference := range disk.Hosts {
		c.hosts[preference.HostID] = preference
	}
	if !c.value.TerminalEnabled {
		c.stopTerminal()
	}
	if !c.value.FilesEnabled {
		c.stopFiles()
	}
	return c
}

func (s *Service) RemoteAccess() RemoteAccess {
	if s.remoteAccess == nil {
		return RemoteAccess{TerminalEnabled: true, FilesEnabled: true, ResourceVersion: "initial", Available: true}
	}
	s.remoteAccess.mu.RLock()
	defer s.remoteAccess.mu.RUnlock()
	return s.remoteAccess.value
}

func (s *Service) UpdateRemoteAccess(version string, terminalEnabled, filesEnabled bool) (RemoteAccess, error) {
	c := s.remoteAccess
	if c == nil {
		return RemoteAccess{}, ErrRemoteAccessStorage
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.value.Available {
		return c.value, ErrRemoteAccessStorage
	}
	if version == "" || version != c.value.ResourceVersion {
		return c.value, ErrConflict
	}
	if terminalEnabled == c.value.TerminalEnabled && filesEnabled == c.value.FilesEnabled {
		return c.value, nil
	}
	revision, err := randomHex(16)
	if err != nil {
		return c.value, ErrRemoteAccessStorage
	}
	content, err := json.Marshal(remoteAccessDisk{SchemaVersion: 1, TerminalEnabled: &terminalEnabled, FilesEnabled: &filesEnabled, Revision: revision, Hosts: c.hostPreferences()})
	if err != nil {
		return c.value, ErrRemoteAccessStorage
	}
	// The replacement is durable before publishing the new admission state.
	if err := atomicWriteFileV2(c.path, content, 0o600, true, c.ops); err != nil {
		return c.value, ErrRemoteAccessStorage
	}
	if terminalEnabled != c.value.TerminalEnabled {
		c.stopTerminal()
		if terminalEnabled {
			c.terminalContext, c.stopTerminal = context.WithCancel(context.Background())
		}
	}
	if filesEnabled != c.value.FilesEnabled {
		c.stopFiles()
		if filesEnabled {
			c.fileContext, c.stopFiles = context.WithCancel(context.Background())
		}
	}
	terminalChanged, filesChanged := terminalEnabled != c.value.TerminalEnabled, filesEnabled != c.value.FilesEnabled
	c.value = RemoteAccess{TerminalEnabled: terminalEnabled, FilesEnabled: filesEnabled, ResourceVersion: revision, Available: true}
	for id, epochs := range c.hostEpochs {
		if terminalChanged {
			epochs.stopTerminal()
			epochs.terminal, epochs.stopTerminal = c.newHostEpoch(id, "terminal")
		}
		if filesChanged {
			epochs.stopFiles()
			epochs.files, epochs.stopFiles = c.newHostEpoch(id, "files")
		}
	}
	return c.value, nil
}

// BeginRemoteOperation binds a request/task to the current policy epoch. Closing
// a switch cancels in-flight work; reopening cannot resurrect an old request.
type remoteAccessEpochKey string

func (s *Service) BeginRemoteOperation(parent context.Context, domain string, hostIDs ...string) (context.Context, context.CancelFunc, error) {
	if parent == nil {
		parent = context.Background()
	}
	if domain != "terminal" && domain != "files" {
		return nil, nil, ErrRemoteAccessDisabled
	}
	c := s.remoteAccess
	if c == nil {
		ctx, cancel := context.WithCancel(parent)
		return ctx, cancel, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	enabled, epoch := c.value.FilesEnabled, c.fileContext
	if domain == "terminal" {
		enabled, epoch = c.value.TerminalEnabled, c.terminalContext
	}
	if !c.value.Available || !enabled {
		return nil, nil, ErrRemoteAccessDisabled
	}
	key := remoteAccessEpochKey(domain)
	if len(hostIDs) > 0 && hostIDs[0] != "" && hostIDs[0] != LocalHostID {
		key = remoteAccessEpochKey(domain + ":" + hostIDs[0])
		epoch = c.hostEpoch(hostIDs[0], domain)
	}
	if admitted, ok := parent.Value(key).(context.Context); ok {
		epoch = admitted
	}
	if epoch.Err() != nil {
		return nil, nil, ErrRemoteAccessDisabled
	}
	parent = context.WithValue(parent, key, epoch)
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(epoch, cancel)
	return ctx, func() { stop(); cancel() }, nil
}

// RemoteOperationLifetime is immutable for one enabled interval. Queued work
// binds this lifetime at admission, so later enabling cannot replay old work.
func (s *Service) RemoteOperationLifetime(domain string, hostIDs ...string) context.Context {
	c := s.remoteAccess
	if c == nil {
		return context.Background()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(hostIDs) > 0 && hostIDs[0] != "" && hostIDs[0] != LocalHostID {
		return c.hostEpoch(hostIDs[0], domain)
	}
	if domain == "terminal" {
		return c.terminalContext
	}
	return c.fileContext
}

func (s *Service) TerminalOpen(ctx context.Context, id string, input TerminalOpenRequest) (TerminalOpenResponse, error) {
	ctx, cancel, err := s.BeginRemoteOperation(ctx, "terminal", id)
	if err != nil {
		return TerminalOpenResponse{}, err
	}
	defer cancel()
	opened, err := s.terminalOpen(ctx, id, input)
	if err == nil && (ctx.Err() != nil || s.remoteTerminalParent(ctx, id).Err() != nil) {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if closeErr := s.TerminalClose(cleanup, id, TerminalCloseRequest{SessionID: opened.SessionID}); closeErr != nil {
			// A failed cleanup is not proof that the PTY never existed. Preserve
			// its identity for the Panel's close-requested registry.
			return opened, ErrRemoteAccessDisabled
		}
		return TerminalOpenResponse{}, ErrRemoteAccessDisabled
	}
	return opened, err
}

func (s *Service) remoteTerminalParent(ctx context.Context, id string) context.Context {
	if epoch, ok := ctx.Value(remoteAccessEpochKey("terminal:" + id)).(context.Context); ok {
		return epoch
	}
	if s.streams != nil {
		return s.streams.ctx
	}
	return context.Background()
}

func (s *Service) OpenRemoteFileV2(ctx context.Context, id string, input FederationFileOpenRequest) (io.ReadCloser, contract.FileTransferMetadata, error) {
	ctx, cancelGlobal, err := s.BeginRemoteOperation(ctx, "files")
	if err != nil {
		return nil, contract.FileTransferMetadata{}, err
	}
	hostID, err := s.FileSourceHostID(id)
	if err != nil {
		cancelGlobal()
		return nil, contract.FileTransferMetadata{}, err
	}
	ctx, cancelHost, err := s.BeginRemoteOperation(ctx, "files", hostID)
	if err != nil {
		cancelGlobal()
		return nil, contract.FileTransferMetadata{}, err
	}
	cancel := func() { cancelHost(); cancelGlobal() }
	body, metadata, err := s.openRemoteFileV2(ctx, id, input)
	if err != nil {
		cancel()
		return nil, metadata, err
	}
	return remoteBody(ctx, cancel, body), metadata, nil
}

func (s *Service) applyRemoteAccess(host Host) Host {
	if host.IsLocal {
		return host
	}
	policy := s.HostRemoteAccess(host.ID)
	host.RemoteTerminalDisabled = !policy.Available || !policy.EffectiveTerminalEnabled
	host.RemoteFilesDisabled = !policy.Available || !policy.EffectiveFilesEnabled
	if host.RemoteTerminalDisabled {
		host.TerminalAvailable = false
	}
	if host.RemoteFilesDisabled {
		host.FileManagementAvailable = false
		host.FileTransferAvailable = false
		host.MutualFileTransferAvailable = false
	}
	return host
}

type remoteAccessBody struct {
	io.ReadCloser
	once   sync.Once
	cancel context.CancelFunc
	stop   func() bool
}

func (b *remoteAccessBody) Close() (err error) {
	b.once.Do(func() { b.stop(); b.cancel(); err = b.ReadCloser.Close() })
	return err
}
func remoteBody(ctx context.Context, cancel context.CancelFunc, body io.ReadCloser) io.ReadCloser {
	b := &remoteAccessBody{ReadCloser: body, cancel: cancel}
	b.stop = context.AfterFunc(ctx, func() { _ = body.Close() })
	return b
}

func (s *Service) remoteFile(ctx context.Context, open func(context.Context) (*http.Response, error), hostIDs ...string) (*http.Response, error) {
	ctx, cancel, err := s.BeginRemoteOperation(ctx, "files", hostIDs...)
	if err != nil {
		return nil, err
	}
	response, err := open(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	response.Body = remoteBody(ctx, cancel, response.Body)
	return response, nil
}

func (s *Service) OpenLightFile(ctx context.Context, id string, input LightFileRequest) (*http.Response, error) {
	return s.remoteFile(ctx, func(ctx context.Context) (*http.Response, error) { return s.openLightFile(ctx, id, input) }, id)
}
func (s *Service) OpenRemotePanelFile(ctx context.Context, id string, input LightFileRequest) (*http.Response, error) {
	return s.remoteFile(ctx, func(ctx context.Context) (*http.Response, error) { return s.openRemotePanelFile(ctx, id, input) }, id)
}
func (s *Service) OpenRemotePanelFileV1(ctx context.Context, id string, input LightFileRequest) (*http.Response, error) {
	return s.remoteFile(ctx, func(ctx context.Context) (*http.Response, error) { return s.openRemotePanelFileV1(ctx, id, input) }, id)
}
