package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pushkaranand/finagent/config"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

func strPtr(s string) *string { return &s }

// stubUsers is a simple in-memory stub for userLookup.
type stubUsers struct {
	byUsername map[string]sqlcgen.User
	byEmail    map[string]sqlcgen.User
	all        []sqlcgen.User
	listErr    error
}

func (s *stubUsers) GetByUsername(_ context.Context, username string) (*sqlcgen.User, error) {
	if u, ok := s.byUsername[username]; ok {
		return &u, nil
	}
	return nil, errors.New("not found")
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
	u := sqlcgen.User{ID: knownUID, Username: "alice", Name: "Alice", Email: strPtr("alice@example.com")}
	return &stubUsers{
		byUsername: map[string]sqlcgen.User{"alice": u},
		byEmail:    map[string]sqlcgen.User{"alice@example.com": u},
		all:        []sqlcgen.User{u},
	}
}

func TestResolveUser_EmptyIdentifier_SingleUser(t *testing.T) {
	u, err := resolveUser(t.Context(), testStub(), "", "")
	require.NoError(t, err)
	assert.Equal(t, knownUID, u.ID)
}

func TestResolveUser_EmptyIdentifier_MultipleUsers(t *testing.T) {
	stub := &stubUsers{
		byUsername: map[string]sqlcgen.User{},
		byEmail:    map[string]sqlcgen.User{},
		all: []sqlcgen.User{
			{ID: knownUID, Username: "alice", Name: "Alice", Email: strPtr("alice@example.com")},
			{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Username: "bob", Name: "Bob", Email: strPtr("bob@example.com")},
		},
	}
	_, err := resolveUser(t.Context(), stub, "", "")
	assert.Error(t, err)
}

func TestResolveUser_EmptyIdentifier_NoUsers(t *testing.T) {
	stub := &stubUsers{byUsername: map[string]sqlcgen.User{}, byEmail: map[string]sqlcgen.User{}, all: []sqlcgen.User{}}
	_, err := resolveUser(t.Context(), stub, "", "")
	assert.Error(t, err)
}

func TestResolveUser_ByUsername(t *testing.T) {
	u, err := resolveUser(t.Context(), testStub(), "alice", "")
	require.NoError(t, err)
	assert.Equal(t, knownUID, u.ID)
}

func TestResolveUser_ByEmail(t *testing.T) {
	u, err := resolveUser(t.Context(), testStub(), "alice@example.com", "")
	require.NoError(t, err)
	assert.Equal(t, knownUID, u.ID)
}

func TestResolveUser_ByName(t *testing.T) {
	u, err := resolveUser(t.Context(), testStub(), "Alice", "")
	require.NoError(t, err)
	assert.Equal(t, knownUID, u.ID)
}

func TestResolveUser_NotFound(t *testing.T) {
	stub := &stubUsers{byUsername: map[string]sqlcgen.User{}, byEmail: map[string]sqlcgen.User{}, all: []sqlcgen.User{}}
	_, err := resolveUser(t.Context(), stub, "unknown@example.com", "")
	assert.Error(t, err)
}

func TestResolveUser_DefaultIdentifier(t *testing.T) {
	u, err := resolveUser(t.Context(), testStub(), "", "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, knownUID, u.ID)
}

func TestToMatrixChannelConfig(t *testing.T) {
	in := config.MatrixConfig{
		HomeserverURL:     "https://matrix.example.com",
		UserID:            "@bot:example.com",
		AccessToken:       "syt_token",
		EncryptionEnabled: true,
		CryptoStorePath:   "/var/lib/bot/crypto.db",
		PickleKey:         "s3cr3t",
		RecoveryKey:       "EsTkey",
		AllowedUsers:      []string{"@alice:example.com", "@bob:example.com"},
		Users: map[string]string{
			"@alice:example.com": "uuid-alice",
			"@bob:example.com":   "uuid-bob",
		},
	}

	out := toMatrixChannelConfig(in)

	assert.Equal(t, in.HomeserverURL, out.HomeserverURL)
	assert.Equal(t, in.UserID, out.UserID)
	assert.Equal(t, in.AccessToken, out.AccessToken)
	assert.Equal(t, in.EncryptionEnabled, out.EncryptionEnabled)
	assert.Equal(t, in.CryptoStorePath, out.CryptoStorePath)
	assert.Equal(t, in.PickleKey, out.PickleKey)
	assert.Equal(t, in.RecoveryKey, out.RecoveryKey)
	assert.Equal(t, in.AllowedUsers, out.AllowedUsers)
	assert.Equal(t, in.Users, out.Users)
}
