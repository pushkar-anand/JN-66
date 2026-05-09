package model

import "errors"

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrDuplicate is returned when an insert would violate a uniqueness constraint.
var ErrDuplicate = errors.New("duplicate")
