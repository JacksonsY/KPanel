package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

func enrollmentReferenceForTest(t *testing.T, token string, change func(*cluster.LightEnrollmentTokenReference)) string {
	t.Helper()
	digest := sha256.Sum256([]byte(token))
	reference := cluster.LightEnrollmentTokenReference{Version: 2, TokenFD: 3, SHA256: hex.EncodeToString(digest[:])}
	if change != nil {
		change(&reference)
	}
	content, err := json.Marshal(reference)
	if err != nil {
		t.Fatal(err)
	}
	prefix, _, _ := strings.Cut(token, ".")
	return prefix + "." + base64.RawURLEncoding.EncodeToString(content)
}

type enrollmentTokenTestReader struct {
	io.Reader
	closed bool
}

func (r *enrollmentTokenTestReader) Close() error { r.closed = true; return nil }

func TestEnrollmentTokenReferenceResolvesOnlyLocalBoundDescriptor(t *testing.T) {
	for _, token := range []string{
		lightTokenForTest(t, "https://panel.example", time.Now().Add(time.Minute)),
		lightBatchTokenForNodeTest(t, "https://panel.example", time.Now().Add(24*time.Hour)),
	} {
		reference := enrollmentReferenceForTest(t, token, nil)
		if _, err := enrollmentTargetFromToken(reference); err == nil {
			t.Fatal("local transport reference accepted as a server wire token")
		}
		for _, suffix := range []string{"", "\n"} {
			reader := &enrollmentTokenTestReader{Reader: strings.NewReader(token + suffix)}
			resolved, err := resolveEnrollmentTokenArgument(reference, func() (io.ReadCloser, error) { return reader, nil })
			if err != nil || resolved != token || !reader.closed {
				t.Fatalf("reference resolution = %q, %v, closed=%v", resolved, err, reader.closed)
			}
		}
		if resolved, err := resolveEnrollmentTokenArgument(token, func() (io.ReadCloser, error) {
			t.Fatal("legacy literal unexpectedly opened FD3")
			return nil, nil
		}); err != nil || resolved != token {
			t.Fatalf("legacy enrollment compatibility failed: %v", err)
		}
	}
}

func TestEnrollmentTokenReferenceRejectsUnsafeInputsWithoutDisclosingBody(t *testing.T) {
	token := lightTokenForTest(t, "https://panel.example", time.Now().Add(time.Minute))
	reference := enrollmentReferenceForTest(t, token, nil)
	for name, test := range map[string]struct {
		argument string
		input    string
		openErr  error
	}{
		"missing-descriptor": {reference, "", errors.New("descriptor missing")},
		"wrong-body":         {reference, "kpl1.SYNTHETIC_PRIVATE_BODY", nil},
		"wrong-type":         {reference, strings.Replace(token, "kpl1.", "kpb1.", 1), nil},
		"extra-newline":      {reference, token + "\n\n", nil},
		"overlong":           {reference, strings.Repeat("a", 2050), nil},
		"nested-reference":   {enrollmentReferenceForTest(t, reference, nil), reference, nil},
		"arbitrary-fd": {enrollmentReferenceForTest(t, token, func(ref *cluster.LightEnrollmentTokenReference) {
			ref.TokenFD = 4
		}), token, nil},
		"wrong-digest": {enrollmentReferenceForTest(t, token, func(ref *cluster.LightEnrollmentTokenReference) {
			ref.SHA256 = strings.Repeat("0", 64)
		}), token, nil},
		"expired": {func() string {
			expired := lightTokenForTest(t, "https://panel.example", time.Now().Add(-time.Minute))
			return enrollmentReferenceForTest(t, expired, nil)
		}(), lightTokenForTest(t, "https://panel.example", time.Now().Add(-time.Minute)), nil},
	} {
		t.Run(name, func(t *testing.T) {
			reader := &enrollmentTokenTestReader{Reader: strings.NewReader(test.input)}
			resolved, err := resolveEnrollmentTokenArgument(test.argument, func() (io.ReadCloser, error) {
				return reader, test.openErr
			})
			if err == nil || resolved != "" || strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "PRIVATE_BODY") {
				t.Fatalf("unsafe reference accepted or disclosed its body: %q, %v", resolved, err)
			}
		})
	}
}
