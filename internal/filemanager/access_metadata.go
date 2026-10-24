package filemanager

import "os"

// applySourceAccess is the shared contract for replacement and relocation.
// Apply after the last content write: chown can clear set-ID/capabilities and
// chmod changes the ACL mask. Exact xattrs (including absence) are applied last.
// Replacement content must never inherit the old executable's capabilities.
func applySourceAccess(target, source *os.File, sameContent bool) error {
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if err := preserveFileOwnership(target, info); err != nil {
		return err
	}
	mode := info.Mode().Perm()
	if sameContent {
		mode |= info.Mode() & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	}
	if err := target.Chmod(mode); err != nil {
		return err
	}
	return preserveFileExtendedAttributes(target, source, sameContent)
}
