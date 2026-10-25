package cluster

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// LightEnrollmentTokenReference is a local CLI transport envelope, never a
// server enrollment credential. Keep its prefix and stable digest compatible
// with the existing installer's batch selection and resumable fingerprint.
type LightEnrollmentTokenReference struct {
	Version int    `json:"v"`
	TokenFD int    `json:"tokenFd"`
	SHA256  string `json:"sha256"`
}

func (s *Service) lightEnrollmentCommand(token, name string) (string, error) {
	prefix, _, found := strings.Cut(token, ".")
	if !found || (prefix != "kpl1" && prefix != "kpb1") || len(token) <= len(prefix)+1 || len(token) > 2048 {
		return "", ErrPairingCode
	}
	for _, ch := range token[len(prefix)+1:] {
		if !(ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-') {
			return "", ErrPairingCode
		}
	}
	digest := sha256.Sum256([]byte(token))
	reference, err := json.Marshal(LightEnrollmentTokenReference{Version: 2, TokenFD: 3, SHA256: hex.EncodeToString(digest[:])})
	if err != nil {
		return "", err
	}
	argument := prefix + "." + base64.RawURLEncoding.EncodeToString(reference)
	command := "bash <(curl -fsSL https://kejilion.sh) kpanel node join " + shellSingleQuote(argument)
	if name != "" {
		command += " --name " + shellSingleQuote(name)
	}
	// Bash constructs this descriptor without putting its body in any child
	// argv or environment. stdin remains the installer's interactive input.
	return command + " 3<<< " + shellSingleQuote(token), nil
}
