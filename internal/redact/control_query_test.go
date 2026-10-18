package redact

import (
	"strings"
	"testing"
)

func TestCredentialsAreRedactedAfterTerminalNormalization(t *testing.T) {
	for _, input := range []string{
		"password\x1b[0m=hidden-value",
		"pa\x00ssword=hidden-value",
		"https://example.test/?client_secret=hidden-value&safe=yes",
		"https://example.test/?private_token=hidden-value&safe=yes",
		"https://example.test/?auth_token=hidden-value&safe=yes",
		"https://example.test/?access_key=hidden-value&safe=yes",
		"https://example.test/?access%5Ftoken=hidden-value&safe=yes",
		"https://example.test/?access%5Fkey=hidden-value&safe=yes",
		"https://example.test/?api%5Fkey=hidden-value&safe=yes",
		"https://example.test/?%74oken=hidden-value&safe=yes",
		"https://example.test/?pass%77ord=hidden-value&safe=yes",
	} {
		got := Text(input)
		if strings.Contains(got, "hidden-value") {
			t.Fatalf("credential survived: %q", got)
		}
		if strings.Contains(input, "safe=yes") && !strings.Contains(got, "safe=yes") {
			t.Fatalf("non-secret query lost: input=%q output=%q", input, got)
		}
	}
	if got := Text("-----BEGIN PGP PRIVATE\x1b[0m KEY BLOCK-----\nprivate-material\n-----END PGP PRIVATE KEY BLOCK-----"); strings.Contains(got, "private-material") {
		t.Fatal("decorated private key survived")
	}
}
