package cluster

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"testing"
)

func TestSignedFileBodyAuthenticatesContentOrderAndTermination(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("sample-content"), 12000)
	encoded, err := io.ReadAll(newSignedFileBody(io.NopCloser(bytes.NewReader(payload)), nil, private, "request-a"))
	if err != nil {
		t.Fatal(err)
	}
	decode := func(data []byte, binding string) ([]byte, error) {
		return io.ReadAll(newSignedFileBody(io.NopCloser(bytes.NewReader(data)), public, nil, binding))
	}
	result, err := decode(encoded, "request-a")
	if err != nil || !bytes.Equal(result, payload) {
		t.Fatalf("roundtrip: %v", err)
	}
	for _, name := range []string{"modified", "truncated", "removed-end", "appended", "cross-request", "reordered"} {
		t.Run(name, func(t *testing.T) {
			data := append([]byte(nil), encoded...)
			binding := "request-a"
			switch name {
			case "modified":
				data[100] ^= 1
			case "truncated":
				data = data[:len(data)-10]
			case "removed-end":
				data = data[:len(data)-68]
			case "appended":
				data = append(data, 'x')
			case "cross-request":
				binding = "request-b"
			case "reordered":
				size := 4 + fileBodyChunkBytes + ed25519.SignatureSize
				first := append([]byte(nil), data[:size]...)
				copy(data[:size], data[size:2*size])
				copy(data[size:2*size], first)
			}
			if _, err := decode(data, binding); err == nil {
				t.Fatal("unauthenticated body accepted")
			}
		})
	}
	// A tampered first chunk is never exposed to a consumer.
	encoded[4] ^= 1
	got, err := decode(encoded, "request-a")
	if len(got) != 0 || !errors.Is(err, ErrAuthentication) {
		t.Fatalf("tampered bytes exposed: len=%d error=%v", len(got), err)
	}
}
