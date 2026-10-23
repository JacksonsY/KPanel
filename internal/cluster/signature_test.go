package cluster

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func signedTestRequest(t *testing.T, now time.Time) (*http.Request, ed25519.PublicKey, string, string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	controllerID := strings.Repeat("a", 32)
	targetID := strings.Repeat("b", 32)
	request := &http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{Path: summaryPath},
		Header: make(http.Header),
	}
	if err := SignRequest(
		request,
		controllerID,
		targetID,
		privateKey,
		now,
		strings.Repeat("c", 32),
	); err != nil {
		t.Fatalf("SignRequest() error: %v", err)
	}
	return request, publicKey, controllerID, targetID
}

func TestSignAndVerifyRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	request, publicKey, controllerID, targetID := signedTestRequest(t, now)
	gotController, gotNonce, err := VerifyRequest(request, targetID, publicKey, now.Add(30*time.Second))
	if err != nil {
		t.Fatalf("VerifyRequest() error: %v", err)
	}
	if gotController != controllerID || gotNonce != strings.Repeat("c", 32) {
		t.Fatalf("VerifyRequest() = %q, %q; want %q and signed nonce", gotController, gotNonce, controllerID)
	}
}

func TestVerifyRequestRejectsTamperingAndExpiredTimestamps(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*http.Request)
		verify time.Time
		target func(string) string
	}{
		{
			name: "path tampered",
			mutate: func(request *http.Request) {
				request.URL.Path = revokePath
			},
			verify: now,
		},
		{
			name: "method tampered",
			mutate: func(request *http.Request) {
				request.Method = http.MethodDelete
			},
			verify: now,
		},
		{
			name: "nonce tampered",
			mutate: func(request *http.Request) {
				request.Header.Set(headerNonce, strings.Repeat("d", 32))
			},
			verify: now,
		},
		{
			name:   "wrong target",
			mutate: func(*http.Request) {},
			verify: now,
			target: func(string) string { return strings.Repeat("d", 32) },
		},
		{
			name:   "timestamp too old",
			mutate: func(*http.Request) {},
			verify: now.Add(61 * time.Second),
		},
		{
			name:   "timestamp too far in future",
			mutate: func(*http.Request) {},
			verify: now.Add(-61 * time.Second),
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request, publicKey, _, targetID := signedTestRequest(t, now)
			test.mutate(request)
			if test.target != nil {
				targetID = test.target(targetID)
			}
			if _, _, err := VerifyRequest(request, targetID, publicKey, test.verify); !errors.Is(err, ErrAuthentication) {
				t.Fatalf("VerifyRequest() error = %v, want ErrAuthentication", err)
			}
		})
	}
}

func TestReplayGuardRejectsDuplicateSignedNonce(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	request, publicKey, controllerID, targetID := signedTestRequest(t, now)
	verifiedController, nonce, err := VerifyRequest(request, targetID, publicKey, now)
	if err != nil {
		t.Fatalf("VerifyRequest() error: %v", err)
	}

	guard := newReplayGuard(4, 4, 5*time.Minute)
	if err := guard.Accept(verifiedController, nonce, now); err != nil {
		t.Fatalf("first replay guard acceptance error: %v", err)
	}
	if err := guard.Accept(verifiedController, nonce, now.Add(time.Second)); !errors.Is(err, ErrReplay) {
		t.Fatalf("duplicate replay guard acceptance error = %v, want ErrReplay", err)
	}
	if err := guard.Accept(strings.Repeat("d", 32), nonce, now.Add(time.Second)); err != nil {
		t.Fatalf("same nonce from a distinct controller should not collide: %v", err)
	}
	if err := guard.Accept(controllerID, nonce, now.Add(5*time.Minute)); err != nil {
		t.Fatalf("nonce should be reusable only after replay window expiry: %v", err)
	}
}
