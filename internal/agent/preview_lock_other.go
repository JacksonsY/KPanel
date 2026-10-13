//go:build !linux

package agent

import (
	"errors"
	"os"
)

// Persistence requires both private directory permissions and process locking.
// Unsupported platforms retain the existing memory-only behavior.
func lockPreviewDisk(*os.File) error {
	return errors.New("persistent previews require Linux file locking")
}
