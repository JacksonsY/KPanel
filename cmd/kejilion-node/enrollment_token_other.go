//go:build !linux

package main

import (
	"errors"
	"io"
)

func openEnrollmentTokenDescriptor() (io.ReadCloser, error) {
	return nil, errors.New("enrollment token descriptor requires Linux")
}
