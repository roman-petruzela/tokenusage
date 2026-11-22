// Package cache provides atomic JSON files and an advisory file lock.
package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"

	"github.com/adrg/xdg"
)

// Dir returns $XDG_CACHE_HOME/tokenusage.
func Dir() string { return filepath.Join(xdg.CacheHome, "tokenusage") }

func UsagePath() string { return filepath.Join(Dir(), "usage.json") }
func IndexPath() string { return filepath.Join(Dir(), "jsonl-index.json") }

// WriteJSON writes v atomically (temp file + rename).
func WriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := json.NewEncoder(tmp).Encode(v); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// ReadJSON decodes path into v. Missing file returns an os.ErrNotExist error.
func ReadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Lock is an exclusive flock on <dir>/lock.
type Lock struct{ f *os.File }

// Acquire blocks until the lock is held.
func Acquire() (*Lock, error) {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(Dir(), "lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{f}, nil
}

func (l *Lock) Release() {
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	l.f.Close()
}
