package localusage

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func msg(id, req, ts, model string, in, out, cc, cr int) string {
	return `{"type":"assistant","timestamp":"` + ts + `","requestId":"` + req + `","message":{"id":"` + id + `","model":"` + model +
		`","usage":{"input_tokens":` + itoa(in) + `,"output_tokens":` + itoa(out) + `,"cache_creation_input_tokens":` + itoa(cc) + `,"cache_read_input_tokens":` + itoa(cr) + `}}}` + "\n"
}

func itoa(i int) string { return strconv.Itoa(i) }

func TestScanAggregate(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	a := msg("m1", "r1", "2026-03-15T11:00:00Z", "opus", 10, 20, 30, 40) + // block, today, week, month
		msg("m1", "r1", "2026-03-15T11:00:00Z", "opus", 10, 20, 30, 40) + // duplicate
		"not json\n" +
		`{"type":"user","message":{"content":"hi"}}` + "\n" +
		msg("m2", "r2", "2026-03-15T01:00:00Z", "sonnet", 1, 1, 0, 0) + // today, before block
		msg("m3", "r3", "2026-03-12T10:00:00Z", "sonnet", 5, 5, 0, 0) + // week
		msg("m4", "r4", "2026-03-02T10:00:00Z", "sonnet", 7, 0, 0, 0) + // month only
		msg("m5", "r5", "2026-02-28T10:00:00Z", "sonnet", 100, 0, 0, 0) // previous month: excluded
	write(t, dir, "proj/a.jsonl", a)
	write(t, dir, "proj2/b.jsonl", msg("m1", "r1", "2026-03-15T11:00:00Z", "opus", 10, 20, 30, 40)) // dup across files

	idx := filepath.Join(t.TempDir(), "idx.json")
	for pass := 0; pass < 2; pass++ { // second pass hits the index
		recs, err := Scan(dir, idx, now)
		if err != nil {
			t.Fatal(err)
		}
		// block starts 11:00 (first message 11:00 truncated to hour)
		tk := Aggregate(recs, now, nil)
		check := func(name string, got, want int64) {
			if got != want {
				t.Errorf("pass %d %s = %d, want %d", pass, name, got, want)
			}
		}
		check("block", tk.Block5h.Total, 100)
		check("today", tk.Today.Total, 102)
		check("week", tk.Week.Total, 112)
		check("month", tk.Month.Total, 119)
		check("month opus out", tk.Month.ByModel["opus"].Output, 20)
		check("month sonnet in", tk.Month.ByModel["sonnet"].Input, 13)
	}
}

func TestBlockExpired(t *testing.T) {
	now := time.Date(2026, 3, 15, 20, 0, 0, 0, time.UTC)
	recs := []Record{{TS: time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC).Unix(), Model: "x", In: 1}}
	if got := Aggregate(recs, now, nil).Block5h.Total; got != 0 {
		t.Errorf("expired block total = %d, want 0", got)
	}
	bs := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	if got := Aggregate(recs, now, &bs).Block5h.Total; got != 1 {
		t.Errorf("override block total = %d, want 1", got)
	}
}
