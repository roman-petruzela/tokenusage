package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRead(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	tests := []struct {
		name    string
		content *string
		wantErr error
		bad     bool // any non-sentinel error
		wantSub string
	}{
		{name: "valid", content: p(`{"claudeAiOauth":{"accessToken":"x","expiresAt":2000000,"subscriptionType":"max","rateLimitTier":"t"}}`), wantSub: "max"},
		{name: "expired", content: p(`{"claudeAiOauth":{"accessToken":"x","expiresAt":500000,"subscriptionType":"pro"}}`), wantErr: ErrExpired, wantSub: "pro"},
		{name: "missing", wantErr: ErrMissing},
		{name: "malformed json", content: p(`{nope`), bad: true},
		{name: "no oauth", content: p(`{}`), bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "c.json")
			if tt.content != nil {
				if err := os.WriteFile(path, []byte(*tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			c, err := Read(path, now)
			switch {
			case tt.wantErr != nil && !errors.Is(err, tt.wantErr):
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			case tt.bad && err == nil:
				t.Fatal("want error")
			case tt.wantErr == nil && !tt.bad && err != nil:
				t.Fatal(err)
			}
			if c.SubscriptionType != tt.wantSub {
				t.Errorf("sub = %q, want %q", c.SubscriptionType, tt.wantSub)
			}
		})
	}
}

func p(s string) *string { return &s }
