package store

import (
	"encoding/base64"
	"strings"
	"time"
)

func validateUserAvatar(avatar string) error {
	if avatar == "" {
		return nil
	}
	mediaType, encoded, ok := strings.Cut(avatar, ",")
	if !ok || encoded == "" || len(encoded) > base64.StdEncoding.EncodedLen(MaxUserAvatarBytes) {
		return ErrInvalidRecord
	}
	switch mediaType {
	case "data:image/png;base64", "data:image/jpeg;base64", "data:image/webp;base64":
	default:
		return ErrInvalidRecord
	}
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data) > MaxUserAvatarBytes {
		return ErrInvalidRecord
	}
	return nil
}

// ReplaceUserAvatar stores the authenticated user's validated avatar without
// changing credentials or revoking their active sessions.
func (s *Store) ReplaceUserAvatar(userID, avatar string, updatedAt time.Time) error {
	if err := validateUserAvatar(avatar); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	userIndex := s.userIndexLocked(userID)
	if userIndex < 0 {
		return ErrNotFound
	}

	previous := cloneDiskState(s.data)
	s.data.Users[userIndex].Avatar = avatar
	s.data.Users[userIndex].UpdatedAt = updatedAt
	if err := s.persistLocked(); err != nil {
		s.data = previous
		return err
	}
	return nil
}
