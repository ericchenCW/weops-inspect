package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EmailConfig contains SMTP credentials and routing.
type EmailConfig struct {
	Enabled  bool     `json:"enabled"`
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	UseTLS   bool     `json:"smtp_use_tls"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

// TriggerConfig controls when notifications fire.
type TriggerConfig struct {
	MinIntervalMinutes int  `json:"min_interval_minutes"`
	SendRecovery       bool `json:"send_recovery"`
}

// PersistenceConfig gates alerts behind N consecutive runs of confirmation.
// ConsecutiveRuns=1 disables the feature entirely (every alert passes through
// on first sight). Default at load time is 2.
type PersistenceConfig struct {
	ConsecutiveRuns int `json:"consecutive_runs"`
}

// Config is the top-level notification configuration.
type Config struct {
	Email       EmailConfig       `json:"email"`
	Trigger     TriggerConfig     `json:"trigger"`
	Persistence PersistenceConfig `json:"persistence"`

	// path is the resolved file path used at load time. Not serialised.
	path string `json:"-"`
}

// ConfigPath returns the resolved config path. WEOPS_CONFIG overrides the
// default ~/.config/weops/config.json.
func ConfigPath() (string, error) {
	if p := os.Getenv("WEOPS_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "weops", "config.json"), nil
}

// StatePath returns the path of the notification state file, sibling to the
// config file.
func StatePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "state.json")
}

// Load reads notification config. Returns (nil, nil) if the file is absent
// (caller should silently skip notifications). Returns an error only when the
// file exists but cannot be parsed.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	if mode := info.Mode().Perm(); mode != 0o600 {
		fmt.Fprintf(os.Stderr, "notify: 配置文件权限 %o 不安全，建议 chmod 600 %s\n", mode, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.path = path

	if cfg.Trigger.MinIntervalMinutes <= 0 {
		cfg.Trigger.MinIntervalMinutes = 120
	}
	if cfg.Persistence.ConsecutiveRuns <= 0 {
		if cfg.Persistence.ConsecutiveRuns < 0 {
			fmt.Fprintf(os.Stderr, "notify: persistence.consecutive_runs %d 非法，回退默认 2\n", cfg.Persistence.ConsecutiveRuns)
		}
		cfg.Persistence.ConsecutiveRuns = 2
	}

	return &cfg, nil
}

// Validate checks that required fields are present when Enabled. Returns a
// human-readable error describing the first missing piece.
func (c *Config) Validate() error {
	if !c.Email.Enabled {
		return nil
	}
	if c.Email.SMTPHost == "" {
		return fmt.Errorf("email.smtp_host 未配置")
	}
	if c.Email.SMTPPort == 0 {
		return fmt.Errorf("email.smtp_port 未配置")
	}
	if c.Email.From == "" {
		return fmt.Errorf("email.from 未配置")
	}
	if len(c.Email.To) == 0 {
		return fmt.Errorf("email.to 未配置")
	}
	return nil
}
