package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var appearanceWallpaperPattern = regexp.MustCompile(`^(classic|xp|orbit|horizon|rift|prism|pack:[a-z0-9][a-z0-9-]{0,39}|custom:[0-9a-f]{32})$`)
var appearanceColorPattern = regexp.MustCompile(`^#[0-9a-f]{6}$`)

func ValidateAppearance(value Appearance) error {
	if value.Branding != nil && !validSiteBranding(*value.Branding) {
		return ErrInvalidRecord
	}
	if value.Theme != "system" && value.Theme != "light" && value.Theme != "dark" {
		return ErrInvalidRecord
	}
	if !appearanceWallpaperPattern.MatchString(value.Wallpaper) {
		return ErrInvalidRecord
	}
	if value.ClassicLevel != "off" && value.ClassicLevel != "ambient" && value.ClassicLevel != "clear" {
		return ErrInvalidRecord
	}
	if value.Colors != nil && (!appearanceColorPattern.MatchString(value.Colors.Brand) ||
		!appearanceColorPattern.MatchString(value.Colors.Neutral) ||
		!appearanceColorPattern.MatchString(value.Colors.Signature)) {
		return ErrInvalidRecord
	}
	return nil
}

func cloneAppearance(value *Appearance) *Appearance {
	if value == nil {
		return nil
	}
	copy := *value
	if value.Branding != nil {
		branding := *value.Branding
		copy.Branding = &branding
	}
	if value.Colors != nil {
		colors := *value.Colors
		copy.Colors = &colors
	}
	return &copy
}

func validSiteBranding(value SiteBranding) bool {
	if !utf8.ValidString(value.Name) || utf8.RuneCountInString(value.Name) > 64 || strings.TrimSpace(value.Name) != value.Name {
		return false
	}
	for _, char := range value.Name {
		if unicode.IsControl(char) {
			return false
		}
	}
	if value.Icon == "" {
		return true
	}
	const prefix = "data:image/png;base64,"
	const maxBytes = 64 << 10
	if !strings.HasPrefix(value.Icon, prefix) || len(value.Icon) > len(prefix)+base64.StdEncoding.EncodedLen(maxBytes) {
		return false
	}
	body, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(value.Icon, prefix))
	if err != nil || len(body) > maxBytes {
		return false
	}
	config, err := png.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 128 || config.Height > 128 {
		return false
	}
	_, err = png.Decode(bytes.NewReader(body))
	return err == nil
}

func appearanceVersion(value *Appearance) string {
	payload, _ := json.Marshal(value)
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("sha256:%x", digest[:])
}

func (s *Store) Appearance() (*Appearance, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneAppearance(s.data.Appearance), appearanceVersion(s.data.Appearance)
}

func (s *Store) ReplaceAppearance(expectedVersion string, value Appearance) error {
	if err := ValidateAppearance(value); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if expectedVersion != appearanceVersion(s.data.Appearance) {
		return ErrConflict
	}
	previous := cloneDiskState(s.data)
	s.data.Appearance = cloneAppearance(&value)
	if err := s.persistLocked(); err != nil {
		s.data = previous
		return err
	}
	return nil
}
