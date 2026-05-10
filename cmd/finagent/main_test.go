package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestResolveUser_EmptyIdentifier_SingleUser(t *testing.T) {
	// No identifier — falls back to the sole user in the DB.
	u, err := resolveUser(context.Background(), testStub(), "", "")
	require.NoError(t, err)
	assert.Equal(t, knownUID, u.ID)
}

func TestResolveUser_EmptyIdentifier_MultipleUsers(t *testing.T) {
	stub := &stubUsers{
		byEmail: map[string]sqlcgen.User{},
		all: []sqlcgen.User{
			{ID: knownUID, Name: "Alice", Email: "alice@example.com"},
			{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Name: "Bob", Email: "bob@example.com"},
		},
	}
	_, err := resolveUser(context.Background(), stub, "", "")
	assert.Error(t, err)
}

func TestResolveUser_EmptyIdentifier_NoUsers(t *testing.T) {
	stub := &stubUsers{byEmail: map[string]sqlcgen.User{}, all: []sqlcgen.User{}}
	_, err := resolveUser(context.Background(), stub, "", "")
	assert.Error(t, err)
}

func TestResolveUser_ByEmail(t *testing.T) {
	u, err := resolveUser(context.Background(), testStub(), "alice@example.com", "")
	require.NoError(t, err)
	assert.Equal(t, knownUID, u.ID)
}

func TestResolveUser_ByName(t *testing.T) {
	u, err := resolveUser(context.Background(), testStub(), "Alice", "")
	require.NoError(t, err)
	assert.Equal(t, knownUID, u.ID)
}

func TestResolveUser_NotFound(t *testing.T) {
	stub := &stubUsers{byEmail: map[string]sqlcgen.User{}, all: []sqlcgen.User{}}
	_, err := resolveUser(context.Background(), stub, "unknown@example.com", "")
	assert.Error(t, err)
}

func TestResolveUser_DefaultIdentifier(t *testing.T) {
	// No explicit identifier — falls back to defaultIdentifier.
	u, err := resolveUser(context.Background(), testStub(), "", "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, knownUID, u.ID)
}

func TestCmp(t *testing.T) {
	assert.Equal(t, "a", cmp("a", "b"))
	assert.Equal(t, "b", cmp("", "b"))
	assert.Equal(t, "", cmp("", ""))
}
