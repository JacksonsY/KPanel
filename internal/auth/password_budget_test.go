package auth

import (
	"errors"
	"github.com/kejilion/kejilion-panel/internal/store"
	"runtime"
	"testing"
)

func TestPasswordVerifierUsesConfiguredWorkBudget(t *testing.T) {
	h := testHasher(t)
	for _, p := range []Argon2idParams{{MemoryKiB: 32 * 1024, Iterations: 1, Threads: 1, SaltLength: 16, KeyLength: 32}, {MemoryKiB: 8 * 1024, Iterations: 2, Threads: 1, SaltLength: 16, KeyLength: 32}} {
		encoded := encodeArgon2id(p, make([]byte, 16), make([]byte, 32))
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, err := h.Verify("ordinary", encoded)
		runtime.ReadMemStats(&after)
		if !errors.Is(err, ErrInvalidPasswordHash) {
			t.Errorf("untrusted hash raised configured work: params=%+v err=%v", p, err)
		}
		if after.TotalAlloc-before.TotalAlloc > 1<<20 {
			t.Errorf("over-budget verifier ran KDF: allocated=%d", after.TotalAlloc-before.TotalAlloc)
		}
	}
}
func TestBackupPasswordWorkAdmissionDoesNotRunKDF(t *testing.T) {
	for _, p := range []Argon2idParams{{MemoryKiB: 128 * 1024, Iterations: 1, Threads: 1, SaltLength: 16, KeyLength: 32}, {MemoryKiB: 64 * 1024, Iterations: 4, Threads: 1, SaltLength: 16, KeyLength: 32}} {
		user := store.User{Username: "admin", PasswordHash: encodeArgon2id(p, make([]byte, 16), make([]byte, 32))}
		if err := ValidateBackupIdentity(user, nil); !errors.Is(err, ErrInvalidPasswordHash) {
			t.Errorf("over-budget backup accepted: params=%+v err=%v", p, err)
		}
	}
	p := DefaultArgon2idParams()
	user := store.User{Username: "admin", PasswordHash: encodeArgon2id(p, make([]byte, 16), make([]byte, 32))}
	if err := ValidateBackupIdentity(user, nil); err != nil {
		t.Fatal(err)
	}
}
func TestPasswordWorkBudgetPreservesLegacyAndExplicitProfiles(t *testing.T) {
	legacy := Argon2idParams{MemoryKiB: 8 * 1024, Iterations: 4, Threads: 4, SaltLength: 16, KeyLength: 32}
	old, err := NewArgon2idHasher(legacy)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := old.Hash("legacy-correct-password")
	if err != nil {
		t.Fatal(err)
	}
	trusted := legacy
	trusted.MemoryKiB = 32 * 1024
	trusted.Iterations = 2
	trusted.Threads = 1
	current, err := NewArgon2idHasher(trusted)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := current.Verify("legacy-correct-password", encoded)
	if err != nil || !ok {
		t.Fatalf("cheaper legacy/different threads rejected: valid=%v err=%v", ok, err)
	}
	explicit := trusted
	explicit.MemoryKiB = 128 * 1024
	high, err := NewArgon2idHasher(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateVerificationBudget(explicit, high.params); err != nil {
		t.Fatal("explicit trusted profile rejected")
	}
	cheap := DefaultArgon2idParams()
	cheap.MemoryKiB = 8 * 1024
	cheap.Iterations = 10
	cheap.Threads = 16
	user := store.User{Username: "admin", PasswordHash: encodeArgon2id(cheap, make([]byte, 16), make([]byte, 32))}
	if err := ValidateBackupIdentity(user, nil); err != nil {
		t.Fatalf("cheaper total-work backup rejected: %v", err)
	}
}
