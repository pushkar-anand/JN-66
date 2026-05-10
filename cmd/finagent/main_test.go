package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// stubUsers is a simple in-memory stub for userLookup.
type stubUsers struct {
	byEmail map[string]sqlcgen.User
	all     []sqlcgen.User
	listErr error
}

func (s *stubUsers) GetByEmail(_ context.Context, email string) (*sqlcgen.User, error) {
	if u, ok := s.byEmail[email]; ok {
		return &u, nil
	}
	return nil, errors.New("not found")
}

func (s *stubUsers) List(_ context.Context) ([]sqlcgen.User, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.all, nil
}

var knownUID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

func testStub() *stubUsers {
	u := sqlcgen.User{ID: knownUID, Name: "Alice", Email: "alice@example.com"}
	return &stubUsers{
		byEmail: map[string]sqlcgen.User{"alice@example.com": u},
		all:     []sqlcgen.User{u},
	}
}

func TestResolveUser_EmptyIdentifier(t *testing.T) {
	got := resolveUser(context.Background(), testStub(), "")
	assert.Equal(t, "", got)
}

func TestResolveUser_ByEmail(t *testing.T) {
	got := resolveUser(context.Background(), testStub(), "alice@example.com")
	assert.Equal(t, knownUID.String(), got)
}

func TestResolveUser_ByName(t *testing.T) {
	stub := testStub()
	// GetByEmail will fail (wrong key), List will return the user by name.
	got := resolveUser(context.Background(), stub, "Alice")
	assert.Equal(t, knownUID.String(), got)
}

func TestResolveUser_DirectUUID(t *testing.T) {
	stub := &stubUsers{byEmail: map[string]sqlcgen.User{}}
	got := resolveUser(context.Background(), stub, knownUID.String())
	assert.Equal(t, knownUID.String(), got)
}

func TestResolveUser_NotFoundNotUUID(t *testing.T) {
	stub := &stubUsers{byEmail: map[string]sqlcgen.User{}}
	got := resolveUser(context.Background(), stub, "unknown@example.com")
	// Must NOT return the raw email — must return "".
	assert.Equal(t, "", got)
}

func TestCmp(t *testing.T) {
	assert.Equal(t, "a", cmp("a", "b"))
	assert.Equal(t, "b", cmp("", "b"))
	assert.Equal(t, "", cmp("", ""))
}
