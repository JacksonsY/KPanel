package ai

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/chacha20poly1305"
)

type SecretBox struct {
	key []byte
}

func OpenSecretBox(path string, encryptedSecretsExist bool) (*SecretBox, error) {
	content, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(content) != chacha20poly1305.KeySize {
			return nil, errors.New("AI secret key has an invalid length")
		}
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("AI secret key must be a regular non-symlink file")
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return nil, fmt.Errorf("protect AI secret key: %w", err)
		}
		return &SecretBox{key: append([]byte(nil), content...)}, nil
	case errors.Is(err, os.ErrNotExist):
		if encryptedSecretsExist {
			return nil, errors.New("AI secret key is missing while encrypted AI state exists")
		}
	default:
		return nil, fmt.Errorf("read AI secret key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create AI secret directory: %w", err)
	}
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate AI secret key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create AI secret key: %w", err)
	}
	written := false
	defer func() {
		_ = file.Close()
		if !written {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(key); err != nil {
		return nil, fmt.Errorf("write AI secret key: %w", err)
	}
	if err := file.Sync(); err != nil {
		return nil, fmt.Errorf("sync AI secret key: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close AI secret key: %w", err)
	}
	written = true
	return &SecretBox{key: key}, nil
}

func (box *SecretBox) Seal(providerID, value string) ([]byte, error) {
	if box == nil || len(box.key) != chacha20poly1305.KeySize {
		return nil, errors.New("AI secret box is unavailable")
	}
	aead, err := chacha20poly1305.NewX(box.key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, []byte(value), []byte(providerID)), nil
}

func (box *SecretBox) Open(providerID string, value []byte) (string, error) {
	if box == nil || len(box.key) != chacha20poly1305.KeySize {
		return "", errors.New("AI secret box is unavailable")
	}
	aead, err := chacha20poly1305.NewX(box.key)
	if err != nil {
		return "", err
	}
	if len(value) < aead.NonceSize() {
		return "", errors.New("encrypted provider key is invalid")
	}
	plain, err := aead.Open(nil, value[:aead.NonceSize()], value[aead.NonceSize():], []byte(providerID))
	if err != nil {
		return "", errors.New("decrypt provider key")
	}
	return string(plain), nil
}
