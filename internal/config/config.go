// Package config loads the TOML config with defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
)

type Config struct {
	// CredentialsPath overrides the Claude Code credentials file location.
	CredentialsPath string `toml:"credentials_path"`
	// ProjectsDir overrides the Claude Code transcripts directory.
	ProjectsDir string `toml:"projects_dir"`
	// RefreshInterval is the TUI auto-refresh period (min 30s).
	RefreshInterval Duration `toml:"refresh_interval"`
	WarnThreshold   float64  `toml:"warn_threshold"`
	CritThreshold   float64  `toml:"crit_threshold"`
	// WaybarText is a template; placeholders: {five_hour} {seven_day} {seven_day_opus} {seven_day_sonnet}.
	WaybarText string `toml:"waybar_text"`
}

// Duration is a time.Duration that decodes from a TOML string such as "60s".
type Duration time.Duration

func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	*d = Duration(v)
	return err
}

func (d Duration) MarshalText() ([]byte, error) { return []byte(time.Duration(d).String()), nil }

func (d Duration) D() time.Duration { return time.Duration(d) }

const MinRefresh = 30 * time.Second

func Default() Config {
	return Config{
		RefreshInterval: Duration(60 * time.Second),
		WarnThreshold:   70,
		CritThreshold:   90,
		WaybarText:      "{five_hour}% · {seven_day}%",
	}
}

// Path returns the config file path.
func Path() string { return filepath.Join(xdg.ConfigHome, "tokenusage", "config.toml") }

// Load reads the config file; a missing file yields defaults.
func Load() (Config, error) {
	c := Default()
	_, err := toml.DecodeFile(Path(), &c)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Default(), fmt.Errorf("config %s: %w", Path(), err)
	}
	if c.RefreshInterval.D() < MinRefresh {
		c.RefreshInterval = Duration(MinRefresh)
	}
	return c, nil
}

const defaultFile = `# tokenusage configuration
# credentials_path = "~/.claude/.credentials.json"   # default honors $CLAUDE_CONFIG_DIR
# projects_dir = "~/.claude/projects"
refresh_interval = "60s"   # TUI auto-refresh, minimum 30s
warn_threshold = 70
crit_threshold = 90
waybar_text = "{five_hour}% · {seven_day}%"
`

// Init writes the default config; it refuses to overwrite an existing file.
func Init() (string, error) {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return p, err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return p, err
	}
	defer f.Close()
	_, err = f.WriteString(defaultFile)
	return p, err
}
