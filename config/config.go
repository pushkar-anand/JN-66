// Package config loads and exposes typed application configuration.
package config

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config is the root application configuration.
type Config struct {
	Database DatabaseConfig    `koanf:"database"`
	LLM      LLMConfig         `koanf:"llm"`
	Agent    AgentConfig       `koanf:"agent"`
	Channel  ChannelConfig     `koanf:"channel"`
	API      APIConfig         `koanf:"api"`
	Log      LogConfig         `koanf:"log"`
	Users    []UserSeed        `koanf:"users"`
	APIKeys  map[string]string `koanf:"api_keys"`
	Zerodha  ZerodhaConfig     `koanf:"zerodha"`
}

// ZerodhaConfig holds Kite Connect credentials for all users.
type ZerodhaConfig struct {
	ServerSecret string                      `koanf:"server_secret"`
	Users        map[string]ZerodhaUserCreds `koanf:"users"`
}

// ZerodhaUserCreds holds the Kite Connect api_key and api_secret for one user.
// Overridable via FINAGENT_ZERODHA__USERS__<USERNAME>__API_KEY etc.
type ZerodhaUserCreds struct {
	APIKey    string `koanf:"api_key"`
	APISecret string `koanf:"api_secret"`
}

// LogConfig controls log level and output format.
type LogConfig struct {
	Level  string `koanf:"level"`  // debug|info|warn|error (default "info")
	Format string `koanf:"format"` // text|json              (default "text")
}

// UserSeed defines a user to be created or updated on startup.
type UserSeed struct {
	Username string `koanf:"username"`
	Name     string `koanf:"name"`
	Email    string `koanf:"email"`
	Timezone string `koanf:"timezone"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	URL         string `koanf:"url"`
	MaxConns    int32  `koanf:"max_connections"`
	AutoMigrate bool   `koanf:"auto_migrate"`
}

// LLMConfig holds LLM provider settings.
type LLMConfig struct {
	BaseURL string        `koanf:"base_url"`
	APIKey  string        `koanf:"api_key"`
	Routing RoutingConfig `koanf:"routing"`
}

// RoutingConfig maps task types to model IDs.
type RoutingConfig struct {
	ChatModel      string `koanf:"chat_model"`
	AnalysisModel  string `koanf:"analysis_model"`
	TaggingModel   string `koanf:"tagging_model"`
	EmbedModel     string `koanf:"embed_model"`
	SummarizeModel string `koanf:"summarize_model"`
}

// AgentConfig controls agent loop behaviour.
type AgentConfig struct {
	MaxToolRounds   int `koanf:"max_tool_rounds"`
	HistoryMessages int `koanf:"history_messages"`
}

// ChannelConfig holds per-channel configuration.
type ChannelConfig struct {
	CLI CLIConfig `koanf:"cli"`
}

// CLIConfig holds CLI-specific configuration.
type CLIConfig struct {
	DefaultUser string `koanf:"default_user"`
}

// APIConfig holds HTTP server configuration.
type APIConfig struct {
	Listen string `koanf:"listen"`
}

// Load reads config.yaml from configPath, then overlays FINAGENT_ environment variables.
func Load(configPath string) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("load config file: %w", err)
	}

	// ENV override: FINAGENT_LLM__API_KEY → llm.api_key (double-underscore = dot separator)
	if err := k.Load(env.Provider("FINAGENT_", ".", func(s string) string {
		s = strings.TrimPrefix(s, "FINAGENT_")
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "__", ".")
		return s
	}), nil); err != nil {
		return nil, fmt.Errorf("load env config: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
