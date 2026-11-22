// Package collect builds the normalized Usage document from API + local transcripts + cache.
package collect

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/roman-petruzela/tokenusage/internal/api"
	"github.com/roman-petruzela/tokenusage/internal/cache"
	"github.com/roman-petruzela/tokenusage/internal/config"
	"github.com/roman-petruzela/tokenusage/internal/credentials"
	"github.com/roman-petruzela/tokenusage/internal/localusage"
	"github.com/roman-petruzela/tokenusage/internal/model"
)

// StaleAfter is the cache age after which a cached-only read reports status "stale".
const StaleAfter = 10 * time.Minute

const AuthHint = "run `claude` once to refresh login"

// ReadCache returns the cached document, if any.
func ReadCache() (model.Usage, bool) {
	var u model.Usage
	if cache.ReadJSON(cache.UsagePath(), &u) != nil || u.Schema != model.SchemaVersion {
		return model.Usage{}, false
	}
	return u, true
}

// Refresh fetches fresh data under the cache lock and writes the cache.
// On failure the last good limits are kept and Status/Error describe the problem.
func Refresh(ctx context.Context, cfg config.Config, version string) model.Usage {
	if l, err := cache.Acquire(); err == nil {
		defer l.Release()
	}
	now := time.Now()
	u, _ := ReadCache()
	u.Schema = model.SchemaVersion
	u.Status, u.Error = model.StatusOK, ""

	creds, err := credentials.Read(cfg.CredentialsPath, now)
	if creds.SubscriptionType != "" {
		u.Plan = model.Plan{Subscription: creds.SubscriptionType, Tier: creds.RateLimitTier}
	}
	switch {
	case errors.Is(err, credentials.ErrExpired):
		u.Status, u.Error = model.StatusAuthExpired, AuthHint
	case err != nil:
		u.Status, u.Error = model.StatusAuthExpired, err.Error()
	default:
		res, ferr := api.New(creds.AccessToken, version).Fetch(ctx)
		switch {
		case errors.Is(ferr, api.ErrAuth):
			u.Status, u.Error = model.StatusAuthExpired, AuthHint
		case ferr != nil:
			u.Status, u.Error = model.StatusNetworkError, ferr.Error()
		default:
			u.Limits = res.Limits
			u.UpdatedAt = now.UTC()
		}
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = now.UTC()
	}

	// Local counts always refresh; they need no network.
	var bs *time.Time
	if w := u.Limits.FiveHour; w != nil && w.ResetsAt != nil {
		t := w.ResetsAt.Add(-5 * time.Hour)
		bs = &t
	}
	if recs, err := localusage.Scan(projectsDir(cfg), cache.IndexPath(), now); err == nil {
		u.Tokens = localusage.Aggregate(recs, now, bs)
	}
	_ = cache.WriteJSON(cache.UsagePath(), u)
	return Prepare(u, now)
}

// Load returns cached data when fresh enough (or cachedOnly), else refreshes.
func Load(ctx context.Context, cfg config.Config, version string, maxAge time.Duration, cachedOnly bool) model.Usage {
	now := time.Now()
	u, ok := ReadCache()
	if cachedOnly {
		if !ok {
			return model.Usage{Schema: model.SchemaVersion, UpdatedAt: now.UTC(), Status: model.StatusNetworkError, Error: "no cache yet; run `tokenusage refresh`"}
		}
		if u.Status == model.StatusOK && now.Sub(u.UpdatedAt) > StaleAfter {
			u.Status = model.StatusStale
		}
		return Prepare(u, now)
	}
	if ok && u.Status == model.StatusOK && now.Sub(u.UpdatedAt) < maxAge {
		return Prepare(u, now)
	}
	return Refresh(ctx, cfg, version)
}

// Prepare recomputes the relative reset times for now.
func Prepare(u model.Usage, now time.Time) model.Usage {
	for _, w := range []*model.Window{u.Limits.FiveHour, u.Limits.SevenDay, u.Limits.SevenDayOpus, u.Limits.SevenDaySonnet} {
		w.Fill(now)
	}
	return u
}

func projectsDir(cfg config.Config) string {
	if cfg.ProjectsDir != "" {
		return cfg.ProjectsDir
	}
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "projects")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".claude", "projects")
}
