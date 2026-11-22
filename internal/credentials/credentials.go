// Package credentials reads Claude Code OAuth credentials. Strictly read-only:
// the token is never refreshed or written (refresh would rotate the refresh
// token and break Claude Code).
package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrMissing = errors.New("credentials file not found")
	ErrExpired = errors.New("access token expired")
)

type Creds struct {
	AccessToken      string
	ExpiresAt        time.Time
	SubscriptionType string
	RateLimitTier    string
}

// Expired reports whether the token is past (or within 10s of) expiry.
func (c Creds) Expired(now time.Time) bool {
	return !c.ExpiresAt.IsZero() && !now.Before(c.ExpiresAt.Add(-10*time.Second))
}

// DefaultPath honors $CLAUDE_CONFIG_DIR, else ~/.claude.
func DefaultPath() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, ".credentials.json")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".claude", ".credentials.json")
}

type file struct {
	ClaudeAiOauth *struct {
		AccessToken      string `json:"accessToken"`
		ExpiresAt        int64  `json:"expiresAt"`
		SubscriptionType string `json:"subscriptionType"`
		RateLimitTier    string `json:"rateLimitTier"`
	} `json:"claudeAiOauth"`
}

// Read loads credentials from path ("" = DefaultPath). A returned Creds is
// valid even with ErrExpired so callers can still show plan info.
func Read(path string, now time.Time) (Creds, error) {
	if path == "" {
		path = DefaultPath()
	}
	if strings.HasPrefix(path, "~/") {
		h, _ := os.UserHomeDir()
		path = filepath.Join(h, path[2:])
	}
	st, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Creds{}, fmt.Errorf("%w: %s", ErrMissing, path)
	}
	if err != nil {
		return Creds{}, err
	}
	if st.Mode().Perm()&0o044 != 0 {
		slog.Warn("credentials file is readable by others", "path", path, "mode", st.Mode().Perm().String())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Creds{}, err
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		return Creds{}, fmt.Errorf("malformed credentials file: %w", err)
	}
	if f.ClaudeAiOauth == nil || f.ClaudeAiOauth.AccessToken == "" {
		return Creds{}, errors.New("malformed credentials file: no claudeAiOauth.accessToken")
	}
	o := f.ClaudeAiOauth
	c := Creds{AccessToken: o.AccessToken, SubscriptionType: o.SubscriptionType, RateLimitTier: o.RateLimitTier}
	if o.ExpiresAt > 0 {
		c.ExpiresAt = time.UnixMilli(o.ExpiresAt)
	}
	if c.Expired(now) {
		return c, ErrExpired
	}
	return c, nil
}
