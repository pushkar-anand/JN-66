package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalYAML = `
database:
  url: "postgres://test:test@localhost/test"
  max_connections: 5
  auto_migrate: false
llm:
  base_url: "http://localhost:3000/api"
  api_key: ""
  routing:
    chat_model: "gpt-4o-mini"
    analysis_model: "gpt-4o"
    tagging_model: "gpt-4o-mini"
    embed_model: "text-embedding-3-small"
    summarize_model: "gpt-4o-mini"
agent:
  max_tool_rounds: 8
  history_messages: 20
channel:
  cli:
    default_user: "alice"
api:
  listen: ":8080"
`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o600))
	return f
}

func TestLoad_FileValues(t *testing.T) {
	cfg, err := Load(writeTempConfig(t, minimalYAML))
	require.NoError(t, err)
	assert.Equal(t, "postgres://test:test@localhost/test", cfg.Database.URL)
	assert.Equal(t, int32(5), cfg.Database.MaxConns)
	assert.False(t, cfg.Database.AutoMigrate)
	assert.Equal(t, "http://localhost:3000/api", cfg.LLM.BaseURL)
	assert.Equal(t, "gpt-4o-mini", cfg.LLM.Routing.ChatModel)
	assert.Equal(t, "alice", cfg.Channel.CLI.DefaultUser)
	assert.Equal(t, ":8080", cfg.API.Listen)
}

func TestLoad_EnvOverride_APIKey(t *testing.T) {
	t.Setenv("FINAGENT_LLM__API_KEY", "secret-token")
	cfg, err := Load(writeTempConfig(t, minimalYAML))
	require.NoError(t, err)
	assert.Equal(t, "secret-token", cfg.LLM.APIKey)
}

func TestLoad_EnvOverride_BaseURL(t *testing.T) {
	t.Setenv("FINAGENT_LLM__BASE_URL", "http://other-host/api")
	cfg, err := Load(writeTempConfig(t, minimalYAML))
	require.NoError(t, err)
	assert.Equal(t, "http://other-host/api", cfg.LLM.BaseURL)
}

func TestLoad_EnvOverride_DatabaseURL(t *testing.T) {
	t.Setenv("FINAGENT_DATABASE__URL", "postgres://prod:pwd@prod-host/db")
	cfg, err := Load(writeTempConfig(t, minimalYAML))
	require.NoError(t, err)
	assert.Equal(t, "postgres://prod:pwd@prod-host/db", cfg.Database.URL)
}

func TestLoad_EnvOverride_DefaultUser(t *testing.T) {
	t.Setenv("FINAGENT_CHANNEL__CLI__DEFAULT_USER", "bob")
	cfg, err := Load(writeTempConfig(t, minimalYAML))
	require.NoError(t, err)
	assert.Equal(t, "bob", cfg.Channel.CLI.DefaultUser)
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load config file")
}

const matrixYAML = `
database:
  url: "postgres://test:test@localhost/test"
llm:
  base_url: "http://localhost:3000/api"
channel:
  matrix:
    homeserver_url: "https://matrix.example.com"
    user_id: "@bot:example.com"
    access_token: "syt_token"
    encryption_enabled: true
    crypto_store_path: "/var/lib/bot/crypto.db"
    pickle_key: "s3cr3t"
    recovery_key: "EsTkey"
    allowed_users:
      - "@alice:example.com"
      - "@partner:example.com"
    users:
      "@alice:example.com": "uuid-alice"
      "@partner:example.com": "uuid-partner"
`

func TestLoad_MatrixConfig(t *testing.T) {
	cfg, err := Load(writeTempConfig(t, matrixYAML))
	require.NoError(t, err)

	m := cfg.Channel.Matrix
	assert.Equal(t, "https://matrix.example.com", m.HomeserverURL)
	assert.Equal(t, "@bot:example.com", m.UserID)
	assert.Equal(t, "syt_token", m.AccessToken)
	assert.True(t, m.EncryptionEnabled)
	assert.Equal(t, "/var/lib/bot/crypto.db", m.CryptoStorePath)
	assert.Equal(t, "s3cr3t", m.PickleKey)
	assert.Equal(t, "EsTkey", m.RecoveryKey)
	assert.Equal(t, []string{"@alice:example.com", "@partner:example.com"}, m.AllowedUsers)
	assert.Equal(t, map[string]string{
		"@alice:example.com":   "uuid-alice",
		"@partner:example.com": "uuid-partner",
	}, m.Users)
}
