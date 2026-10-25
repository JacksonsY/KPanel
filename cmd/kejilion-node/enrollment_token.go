package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/kejilion/kejilion-panel/internal/cluster"
)

func readEnrollmentTokenArgument(argument string) (string, error) {
	return resolveEnrollmentTokenArgument(argument, openEnrollmentTokenDescriptor)
}

func resolveEnrollmentTokenArgument(argument string, openDescriptor func() (io.ReadCloser, error)) (string, error) {
	invalid := errors.New("enrollment token transport is invalid")
	argument = strings.TrimSpace(argument)
	prefix, body, found := strings.Cut(argument, ".")
	if !found || (prefix != "kpl1" && prefix != "kpb1") || len(argument) > 2048 {
		return "", invalid
	}
	content, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", invalid
	}
	var version struct {
		Version int `json:"v"`
	}
	if err := json.Unmarshal(content, &version); err != nil {
		return "", invalid
	}
	if version.Version == 1 {
		// Literal arguments remain accepted for existing centers/installers.
		// Newly generated commands always use the protected descriptor route.
		return argument, nil
	}
	var reference cluster.LightEnrollmentTokenReference
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reference); err != nil || reference.Version != 2 || reference.TokenFD != 3 {
		return "", invalid
	}
	digest, err := hex.DecodeString(reference.SHA256)
	if err != nil || len(digest) != sha256.Size || reference.SHA256 != strings.ToLower(reference.SHA256) {
		return "", invalid
	}
	reader, err := openDescriptor()
	if err != nil {
		return "", invalid
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, 2050))
	if err != nil {
		return "", invalid
	}
	defer clear(data)
	// The generated Bash here-string contributes exactly one trailing LF.
	// Reject oversized, nested and differently typed references before any
	// credential request or resumable batch state can be created.
	data = bytes.TrimSuffix(data, []byte{'\n'})
	if len(data) == 0 || len(data) > 2048 || !bytes.HasPrefix(data, []byte(prefix+".")) {
		return "", invalid
	}
	actual := sha256.Sum256(data)
	if !bytes.Equal(actual[:], digest) {
		return "", invalid
	}
	token := string(data)
	if _, err := enrollmentTargetFromToken(token); err != nil {
		return "", invalid
	}
	return token, nil
}
