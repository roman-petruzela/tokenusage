// Package model defines the normalized, versioned usage schema (the widget contract).
package model

import "time"

const SchemaVersion = 1

const (
	StatusOK           = "ok"
	StatusAuthExpired  = "auth_expired"
	StatusNetworkError = "network_error"
	StatusStale        = "stale"
)

// Usage is the root document written to the cache and printed by `tokenusage json`.
type Usage struct {
	Schema    int       `json:"schema"`
	UpdatedAt time.Time `json:"updated_at"`
	Status    string    `json:"status"`
	Error     string    `json:"error"`
	Plan      Plan      `json:"plan"`
	Limits    Limits    `json:"limits"`
	Tokens    Tokens    `json:"tokens"`
}

type Plan struct {
	Subscription string `json:"subscription"`
	Tier         string `json:"tier"`
}

type Limits struct {
	FiveHour       *Window `json:"five_hour"`
	SevenDay       *Window `json:"seven_day"`
	SevenDayOpus   *Window `json:"seven_day_opus"`
	SevenDaySonnet *Window `json:"seven_day_sonnet"`
}

// Window is one rate-limit window. ResetsAt is nil when unknown.
type Window struct {
	Utilization     float64    `json:"utilization"`
	ResetsAt        *time.Time `json:"resets_at"`
	ResetsInSeconds int64      `json:"resets_in_seconds"`
}

// Fill computes ResetsInSeconds relative to now (never negative).
func (w *Window) Fill(now time.Time) {
	if w == nil || w.ResetsAt == nil {
		return
	}
	w.ResetsInSeconds = max(int64(w.ResetsAt.Sub(now).Seconds()), 0)
}

type Tokens struct {
	Block5h Counts `json:"block_5h"`
	Today   Counts `json:"today"`
	Week    Counts `json:"week"`
	Month   Counts `json:"month"`
}

// Counts are token totals with a per-model breakdown.
type Counts struct {
	Input         int64             `json:"input"`
	Output        int64             `json:"output"`
	CacheCreation int64             `json:"cache_creation"`
	CacheRead     int64             `json:"cache_read"`
	Total         int64             `json:"total"`
	ByModel       map[string]Counts `json:"by_model,omitempty"`
}

// Add accumulates one message's usage into c (and the model bucket).
func (c *Counts) Add(modelName string, in, out, cc, cr int64) {
	c.Input += in
	c.Output += out
	c.CacheCreation += cc
	c.CacheRead += cr
	c.Total += in + out + cc + cr
	if c.ByModel == nil {
		c.ByModel = map[string]Counts{}
	}
	m := c.ByModel[modelName]
	m.Input += in
	m.Output += out
	m.CacheCreation += cc
	m.CacheRead += cr
	m.Total += in + out + cc + cr
	c.ByModel[modelName] = m
}
