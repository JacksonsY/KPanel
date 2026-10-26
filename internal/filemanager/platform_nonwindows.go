//go:build !windows

package filemanager

import (
	"context"
	"github.com/kejilion/kejilion-panel/internal/contract"
)

func platformVirtualRoot(string) bool                             { return false }
func platformPathKey(value string) string                         { return value }
func platformNameValid(string) bool                               { return true }
func (m *Manager) platformCanonical(value string) (string, error) { return value, nil }
func (m *Manager) listPlatformRoot(context.Context, string, ListOptions) (contract.FileDirectory, error, bool) {
	return contract.FileDirectory{}, nil, false
}
