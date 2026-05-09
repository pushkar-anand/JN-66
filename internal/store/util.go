package store

import (
	"fmt"

	"github.com/google/uuid"
)

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid uuid %q: %w", s, err)
	}
	return id, nil
}
