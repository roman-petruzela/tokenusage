package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchParse(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		five, week, opus bool
		fiveUtil         float64
		fiveHasReset     bool
	}{
		{"full", `{"five_hour":{"utilization":42.5,"resets_at":"2026-01-01T14:00:00.123+00:00"},"seven_day":{"utilization":10,"resets_at":"2026-01-05T00:00:00Z"},"seven_day_opus":{"utilization":3,"resets_at":null},"seven_day_sonnet":null}`, true, true, true, 42.5, true},
		{"missing windows", `{"five_hour":{"utilization":1,"resets_at":null}}`, true, false, false, 1, false},
		{"nulls", `{"five_hour":null,"seven_day":null,"seven_day_opus":null}`, false, false, false, 0, false},
		{"extra fields", `{"extra_usage":{"x":1},"five_hour":{"utilization":7,"resets_at":"2026-01-01T14:00:00Z","new":true},"unknown":[1]}`, true, false, false, 7, true},
		{"bad window ignored", `{"five_hour":{"utilization":"nope"},"seven_day":{"utilization":2,"resets_at":"garbage"}}`, false, true, false, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("anthropic-beta") != "oauth-2025-04-20" {
					t.Errorf("bad headers: %v", r.Header)
				}
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			c := New("tok", "test")
			c.URL = srv.URL
			res, err := c.Fetch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			l := res.Limits
			if (l.FiveHour != nil) != tt.five || (l.SevenDay != nil) != tt.week || (l.SevenDayOpus != nil) != tt.opus {
				t.Fatalf("window presence wrong: %+v", l)
			}
			if l.FiveHour != nil && (l.FiveHour.Utilization != tt.fiveUtil || (l.FiveHour.ResetsAt != nil) != tt.fiveHasReset) {
				t.Errorf("five_hour = %+v", l.FiveHour)
			}
			if string(res.Raw) != tt.body {
				t.Error("raw not preserved")
			}
		})
	}
}

func TestFetchErrors(t *testing.T) {
	var calls atomic.Int32
	tests := []struct {
		name      string
		handler   http.HandlerFunc
		want      error
		wantCalls int32
	}{
		{"401", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }, ErrAuth, 1},
		{"retry 5xx", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }, ErrNetwork, 2},
		{"5xx then ok", func(w http.ResponseWriter, r *http.Request) {
			if calls.Load() == 1 {
				w.WriteHeader(500)
				return
			}
			w.Write([]byte(`{}`))
		}, nil, 2},
		{"invalid json", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`<html>`)) }, ErrNetwork, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls.Store(0)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				tt.handler(w, r)
			}))
			defer srv.Close()
			c := New("tok", "test")
			c.URL, c.RetryDelay = srv.URL, time.Millisecond
			_, err := c.Fetch(context.Background())
			if !errors.Is(err, tt.want) && (tt.want != nil || err != nil) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if calls.Load() != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls.Load(), tt.wantCalls)
			}
		})
	}
}
