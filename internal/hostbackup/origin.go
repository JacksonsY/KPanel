package hostbackup

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"

	"github.com/kejilion/kejilion-panel/internal/backup"
)

// A host payload exported by this Agent carries an origin tag: an HMAC over
// its decompressed stream, keyed by a secret that only this Agent holds and
// that never enters a backup. The tag lives in a gzip header extra subfield,
// which older readers ignore, so the payload format stays unchanged. A
// package without a valid tag (older exports, other hosts, edited payloads)
// is treated as foreign and its restore needs an explicit confirmation.
const (
	originKeyName    = "origin.key"
	originKeyBytes   = 32
	originVersion    = 1
	originDataBytes  = 1 + sha256.Size
	originExtraBytes = 4 + originDataBytes
	// Go's gzip writer emits a 10-byte fixed header and the 2-byte XLEN
	// before Extra; Name and Comment follow it, so the offset is constant.
	originExtraOffset = 12
)

var originSubfield = [2]byte{'K', 'O'}
var originContext = []byte("kpanel-hostbackup-origin-v1\x00")

func (e *Engine) originKeyPath() (string, error) {
	stateDir, err := backup.CanonicalRoot(e.StateDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, "backup-center", originKeyName), nil
}

// originKey returns nil without an error when no key exists and create is
// false: nothing can then be local. A key is published once by hard link and
// never replaced, so earlier local exports stay verifiable.
func (e *Engine) originKey(create bool) ([]byte, error) {
	path, err := e.originKeyPath()
	if err != nil {
		return nil, err
	}
	key, err := backup.ReadFile(path, originKeyBytes)
	if err == nil {
		if len(key) != originKeyBytes {
			return nil, backup.ErrInvalid
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if !create {
		return nil, nil
	}
	if err := backup.PrivateDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	key = make([]byte, originKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".origin-key-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(temp.Name())
	_, err = temp.Write(key)
	if err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if err := os.Link(temp.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return e.originKey(false)
		}
		return nil, err
	}
	if err := backup.SyncDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	return key, nil
}

func originMAC(key, digest []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(originContext)
	mac.Write(digest)
	return mac.Sum(nil)
}

// originPlaceholder reserves the subfield while the payload is written; the
// tag is filled in place once the stream digest is known.
func originPlaceholder() []byte {
	extra := make([]byte, originExtraBytes)
	extra[0], extra[1] = originSubfield[0], originSubfield[1]
	binary.LittleEndian.PutUint16(extra[2:4], originDataBytes)
	extra[4] = originVersion
	return extra
}

func writeOriginTag(f *os.File, key, digest []byte) error {
	header := make([]byte, originExtraOffset+originExtraBytes)
	if _, err := f.ReadAt(header, 0); err != nil {
		return err
	}
	if header[3]&0x04 == 0 || binary.LittleEndian.Uint16(header[10:12]) != originExtraBytes || !bytes.Equal(header[originExtraOffset:originExtraOffset+5], originPlaceholder()[:5]) {
		return backup.ErrInvalid
	}
	_, err := f.WriteAt(originMAC(key, digest), originExtraOffset+5)
	return err
}

// originTag returns the tag from a gzip Extra field, or nil when the field is
// absent, malformed, repeated or of an unknown version.
func originTag(extra []byte) []byte {
	var tag []byte
	found := 0
	for len(extra) >= 4 {
		size := int(binary.LittleEndian.Uint16(extra[2:4]))
		if len(extra) < 4+size {
			return nil
		}
		if extra[0] == originSubfield[0] && extra[1] == originSubfield[1] {
			found++
			data := extra[4 : 4+size]
			if size == originDataBytes && data[0] == originVersion {
				tag = data[1:]
			}
		}
		extra = extra[4+size:]
	}
	if len(extra) != 0 || found != 1 {
		return nil
	}
	return tag
}

func originLocal(key, tag, digest []byte) bool {
	return len(key) == originKeyBytes && len(tag) == sha256.Size && hmac.Equal(tag, originMAC(key, digest))
}
