package auth

import (
	"errors"
	"strings"
	"testing"
)

func testHasher(t *testing.T) *Argon2idHasher {
	t.Helper()
	hasher, err := NewArgon2idHasher(Argon2idParams{
		MemoryKiB: 8 * 1024, Iterations: 1, Threads: 1, SaltLength: 16, KeyLength: 32,
	})
	if err != nil {
		t.Fatal(err)
	}
	return hasher
}

func TestArgon2idHashAndVerify(t *testing.T) {
	hasher := testHasher(t)
	encoded, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Fatalf("unexpected PHC encoding: %q", encoded)
	}
	valid, err := hasher.Verify("correct horse battery staple", encoded)
	if err != nil || !valid {
		t.Fatalf("expected password to verify, valid=%v err=%v", valid, err)
	}
	valid, err = hasher.Verify("wrong password", encoded)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("wrong password verified")
	}
}

func TestArgon2idVerifyRejectsUnsafeParameters(t *testing.T) {
	hasher := testHasher(t)
	cases := []string{
		"",
		"$argon2id$v=19$m=999999999,t=3,p=4$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZg",
		"$argon2id$v=19$m=65536,t=999,p=4$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZg",
		"$argon2id$v=19$m=65536,t=3,p=255$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZg",
		"$argon2id$v=18$m=65536,t=3,p=4$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZg",
	}
	for _, encoded := range cases {
		if _, err := hasher.Verify("password", encoded); !errors.Is(err, ErrInvalidPasswordHash) {
			t.Errorf("expected ErrInvalidPasswordHash for %q, got %v", encoded, err)
		}
	}
}

func TestArgon2idDummyHashPreservesVerificationParameters(t *testing.T) {
	hasher := testHasher(t)
	first, err := hasher.dummyHash()
	if err != nil {
		t.Fatal(err)
	}
	second, err := hasher.dummyHash()
	if err != nil {
		t.Fatal(err)
	}
	params, salt, key, err := parseArgon2id(first)
	if err != nil {
		t.Fatal(err)
	}
	if params != hasher.params || len(salt) != int(params.SaltLength) || len(key) != int(params.KeyLength) {
		t.Fatalf("dummy verifier changed password cost or encoding: %+v", params)
	}
	if first == second {
		t.Fatal("dummy verifiers must use fresh randomness")
	}
	for _, password := range []string{"kejilion-panel-invalid-password", "a-strong-password-1"} {
		valid, err := hasher.Verify(password, first)
		if err != nil || valid {
			t.Fatalf("dummy verifier accepted %q: valid=%v err=%v", password, valid, err)
		}
	}
}
