package virtualmachine

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const maxImageBytes int64 = 8 << 30

func imageSource(id, arch string) (base, file, sums string, err error) {
	switch id {
	case "debian-12":
		return "https://cloud.debian.org/images/cloud/bookworm/latest/", "debian-12-genericcloud-" + arch + ".qcow2", "SHA512SUMS", nil
	case "ubuntu-24.04":
		return "https://cloud-images.ubuntu.com/noble/current/", "noble-server-cloudimg-" + arch + ".img", "SHA256SUMS", nil
	}
	return "", "", "", ErrInvalid
}

func checksumFor(data []byte, name string, bytes int) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		digest, err := hex.DecodeString(fields[0])
		if err != nil || len(digest) != bytes {
			return "", errors.New("invalid publisher checksum")
		}
		return strings.ToLower(fields[0]), nil
	}
	return "", errors.New("publisher checksum does not list selected image")
}

func (s *Service) downloadImage(ctx context.Context, id string) (string, error) {
	base, file, sums, err := imageSource(id, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 25 * time.Minute, Transport: &http.Transport{Proxy: nil, DisableCompression: true, ResponseHeaderTimeout: 20 * time.Second, TLSHandshakeTimeout: 10 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("image redirects are not allowed") }}
	defer client.CloseIdleConnections()
	get := func(name string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+name, nil)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("image source returned HTTP %d", response.StatusCode)
		}
		return response, nil
	}
	res, err := get(sums)
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (256<<10)+1))
	res.Body.Close()
	if err != nil {
		return "", err
	}
	if len(data) > 256<<10 {
		return "", errors.New("checksum manifest exceeds limit")
	}
	var h hash.Hash = sha256.New()
	if sums == "SHA512SUMS" {
		h = sha512.New()
	}
	expected, err := checksumFor(data, file, h.Size())
	if err != nil {
		return "", err
	}
	res, err = get(file)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.ContentLength > maxImageBytes {
		return "", errors.New("image exceeds download limit")
	}
	f, err := os.CreateTemp(s.dir, ".cloud-image-*")
	if err != nil {
		return "", err
	}
	success := false
	defer func() {
		f.Close()
		if !success {
			os.Remove(f.Name())
		}
	}()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, maxImageBytes+1))
	if err != nil {
		return "", err
	}
	if n > maxImageBytes {
		return "", errors.New("image exceeds download limit")
	}
	if hex.EncodeToString(h.Sum(nil)) != expected {
		return "", errors.New("image does not match publisher checksum; retry from a fresh task")
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	success = true
	return f.Name(), nil
}
