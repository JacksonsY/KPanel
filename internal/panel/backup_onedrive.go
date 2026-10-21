package panel

import (
	"context"
	"net/http"
	"time"

	"github.com/kejilion/kejilion-panel/internal/backup"
	"github.com/kejilion/kejilion-panel/internal/backupremote"
)

func (s *Server) serveOneDriveAuth(w http.ResponseWriter, r *http.Request, p []string) {
	select {
	case s.backupRemoteGate <- struct{}{}:
		defer func() { <-s.backupRemoteGate }()
	default:
		s.backupError(w, r, backup.ErrBusy)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	var err error
	switch {
	case len(p) == 2 && r.Method == "POST":
		var input struct {
			Revision string               `json:"revision"`
			Storage  backupremote.Storage `json:"storage"`
		}
		if err = decodeBackupSettings(r, &input); err != nil {
			break
		}
		if input.Revision != s.backupRemote.Snapshot().Revision {
			err = backupremote.ErrConflict
			break
		}
		if input.Storage.ID != "" {
			if _, err = s.backupRemote.Storage(input.Storage.ID); err != nil {
				break
			}
		}
		var auth backupremote.DeviceAuthorization
		auth, err = s.backupOneDriveAuth.Start(ctx, input.Revision, input.Storage)
		if err == nil {
			s.writeJSON(w, 200, auth)
			return
		}
	case len(p) == 3 && backup.ValidID(p[2]) && r.Method == "POST":
		var storage backupremote.Storage
		var revision string
		var pending bool
		storage, revision, pending, err = s.backupOneDriveAuth.Poll(ctx, p[2])
		if err != nil {
			break
		}
		if pending {
			s.writeJSON(w, 200, map[string]bool{"pending": true})
			return
		}
		// Start assigned an ID for validation; PutStorage creates new entries.
		if _, e := s.backupRemote.Storage(storage.ID); e != nil {
			storage.ID = ""
		}
		if err = s.backupRemote.PutStorage(revision, storage); err == nil {
			s.writeJSON(w, 200, map[string]any{"pending": false, "settings": s.backupSettingsSnapshot()})
			return
		}
	case len(p) == 3 && backup.ValidID(p[2]) && r.Method == "DELETE":
		s.backupOneDriveAuth.Cancel(p[2])
		s.writeJSON(w, 200, map[string]bool{"ok": true})
		return
	default:
		err = backupremote.ErrInvalid
	}
	s.remoteBackupError(w, r, err)
}
