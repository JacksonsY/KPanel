package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

var ErrInvalidPasswordHash = errors.New("invalid password hash")

// PasswordHasher keeps password storage replaceable without coupling HTTP or
// persistence code to a particular implementation.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encoded string) (bool, error)
}

type Argon2idParams struct {
	MemoryKiB  uint32
	Iterations uint32
	Threads    uint8
	SaltLength uint32
	KeyLength  uint32
}

type Argon2idHasher struct {
	params Argon2idParams
}

func DefaultArgon2idParams() Argon2idParams {
	threads := min(runtime.NumCPU(), 4)
	if threads < 1 {
		threads = 1
	}
	return Argon2idParams{
		MemoryKiB:  64 * 1024,
		Iterations: 3,
		Threads:    uint8(threads),
		SaltLength: 16,
		KeyLength:  32,
	}
}

func NewArgon2idHasher(params Argon2idParams) (*Argon2idHasher, error) {
	if err := validateParams(params); err != nil {
		return nil, err
	}
	return &Argon2idHasher{params: params}, nil
}

func (h *Argon2idHasher) Hash(password string) (string, error) {
	if password == "" {
		return "", errors.New("password is empty")
	}
	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey(
		[]byte(password),
		salt,
		h.params.Iterations,
		h.params.MemoryKiB,
		h.params.Threads,
		h.params.KeyLength,
	)
	return encodeArgon2id(h.params, salt, key), nil
}

// dummyHash has the same verification cost as a real password hash. Its random
// digest is deliberately unrelated to a password: missing accounts are always
// rejected after Verify, so startup need not run Argon2 just to obtain a digest.
func (h *Argon2idHasher) dummyHash() (string, error) {
	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate verifier salt: %w", err)
	}
	key := make([]byte, h.params.KeyLength)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate verifier digest: %w", err)
	}
	return encodeArgon2id(h.params, salt, key), nil
}

func encodeArgon2id(params Argon2idParams, salt, key []byte) string {
	encoding := base64.RawStdEncoding
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.MemoryKiB,
		params.Iterations,
		params.Threads,
		encoding.EncodeToString(salt),
		encoding.EncodeToString(key),
	)
}

func (h *Argon2idHasher) Verify(password, encoded string) (bool, error) {
	params, salt, expected, err := parseArgon2id(encoded)
	if err != nil {
		return false, err
	}
	if err := validateVerificationBudget(params, h.params); err != nil {
		return false, err
	}
	actual := argon2.IDKey(
		[]byte(password),
		salt,
		params.Iterations,
		params.MemoryKiB,
		params.Threads,
		params.KeyLength,
	)
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

// The PHC envelope is untrusted, even when syntactically valid. It may not
// raise the memory or total memory-pass work chosen by the trusted verifier.
// Keep parallelism portable across hosts and accept cheaper legacy profiles.
func validateVerificationBudget(params, configured Argon2idParams) error {
	if params.MemoryKiB > configured.MemoryKiB || uint64(params.MemoryKiB)*uint64(params.Iterations) > uint64(configured.MemoryKiB)*uint64(configured.Iterations) {
		return ErrInvalidPasswordHash
	}
	return nil
}

func parseArgon2id(encoded string) (Argon2idParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}

	versionText, ok := strings.CutPrefix(parts[2], "v=")
	if !ok {
		return Argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}
	version, err := strconv.ParseUint(versionText, 10, 32)
	if err != nil || version != argon2.Version {
		return Argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}

	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return Argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}
	if parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", memory, iterations, threads) {
		return Argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}

	encoding := base64.RawStdEncoding
	if len(parts[4]) > 128 || len(parts[5]) > 128 {
		return Argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}
	salt, err := encoding.DecodeString(parts[4])
	if err != nil {
		return Argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}
	key, err := encoding.DecodeString(parts[5])
	if err != nil {
		return Argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}
	if len(salt) < 16 || len(salt) > 64 || len(key) < 16 || len(key) > 64 {
		return Argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}

	params := Argon2idParams{
		MemoryKiB:  memory,
		Iterations: iterations,
		Threads:    threads,
		SaltLength: uint32(len(salt)),
		KeyLength:  uint32(len(key)),
	}
	if err := validateParams(params); err != nil {
		return Argon2idParams{}, nil, nil, ErrInvalidPasswordHash
	}
	return params, salt, key, nil
}

func validateParams(params Argon2idParams) error {
	switch {
	case params.MemoryKiB < 8*1024 || params.MemoryKiB > 256*1024:
		return errors.New("argon2 memory must be between 8 MiB and 256 MiB")
	case params.Iterations < 1 || params.Iterations > 10:
		return errors.New("argon2 iterations must be between 1 and 10")
	case params.Threads < 1 || params.Threads > 16:
		return errors.New("argon2 parallelism must be between 1 and 16")
	case params.SaltLength < 16 || params.SaltLength > 64:
		return errors.New("argon2 salt length must be between 16 and 64 bytes")
	case params.KeyLength < 16 || params.KeyLength > 64:
		return errors.New("argon2 key length must be between 16 and 64 bytes")
	default:
		return nil
	}
}
