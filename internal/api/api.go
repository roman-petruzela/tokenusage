// Package api talks to the (undocumented) Claude OAuth usage endpoint.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/roman-petruzela/tokenusage/internal/model"
)

const DefaultURL = "https://api.anthropic.com/api/oauth/usage"

var (
	// ErrAuth means the token was rejected (401/403).
	ErrAuth = errors.New("authentication failed")
	// ErrNetwork covers transport failures and non-auth HTTP errors.
	ErrNetwork = errors.New("network error")
)

type Client struct {
	URL     string
	Token   string
	Version string
	HTTP    *http.Client
	// RetryDelay is the wait before the single retry on 5xx.
	RetryDelay time.Duration
}

func New(token, version string) *Client {
	return &Client{URL: DefaultURL, Token: token, Version: version,
		HTTP: &http.Client{Timeout: 10 * time.Second}, RetryDelay: time.Second}
}

// Result is the parsed response plus the untouched body for debugging.
type Result struct {
	Limits model.Limits
	Raw    json.RawMessage
}

// Fetch calls the endpoint, retrying once on 5xx.
func (c *Client) Fetch(ctx context.Context) (Result, error) {
	body, err := c.get(ctx)
	if err != nil && errors.Is(err, errServer) {
		select {
		case <-time.After(c.RetryDelay):
		case <-ctx.Done():
			return Result{}, fmt.Errorf("%w: %v", ErrNetwork, ctx.Err())
		}
		body, err = c.get(ctx)
	}
	if err != nil {
		return Result{}, err
	}
	l, err := Parse(body)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrNetwork, err)
	}
	return Result{Limits: l, Raw: body}, nil
}

var errServer = errors.New("server error")

func (c *Client) get(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "tokenusage/"+c.Version)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		// url.Error may embed the URL only, never headers, so the token is safe.
		return nil, fmt.Errorf("%w: %v", ErrNetwork, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetwork, err)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%w: HTTP %d", ErrAuth, resp.StatusCode)
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("%w: HTTP %d: %w", ErrNetwork, resp.StatusCode, errServer)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%w: HTTP %d", ErrNetwork, resp.StatusCode)
	}
	return b, nil
}

type rawWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    *string  `json:"resets_at"`
}

// Parse tolerantly extracts known windows; unknown fields and bad windows are ignored.
func Parse(body []byte) (model.Limits, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return model.Limits{}, fmt.Errorf("invalid JSON: %w", err)
	}
	return model.Limits{
		FiveHour:       window(m["five_hour"]),
		SevenDay:       window(m["seven_day"]),
		SevenDayOpus:   window(m["seven_day_opus"]),
		SevenDaySonnet: window(m["seven_day_sonnet"]),
	}, nil
}

func window(raw json.RawMessage) *model.Window {
	if len(raw) == 0 {
		return nil
	}
	var r rawWindow
	if json.Unmarshal(raw, &r) != nil || r.Utilization == nil {
		return nil
	}
	w := &model.Window{Utilization: *r.Utilization}
	if r.ResetsAt != nil {
		if t, err := time.Parse(time.RFC3339, *r.ResetsAt); err == nil {
			w.ResetsAt = &t
		}
	}
	return w
}
