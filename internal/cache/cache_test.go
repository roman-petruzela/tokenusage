package cache

import (
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "x.json")
	if err := WriteJSON(p, map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	var got map[string]int
	if err := ReadJSON(p, &got); err != nil || got["a"] != 1 {
		t.Fatalf("got %v, err %v", got, err)
	}
}
