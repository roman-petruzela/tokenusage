// Package localusage aggregates token counts from Claude Code JSONL transcripts.
package localusage

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/roman-petruzela/tokenusage/internal/cache"
	"github.com/roman-petruzela/tokenusage/internal/model"
)

const blockLen = 5 * time.Hour

// Record is one deduplicable assistant message usage entry.
type Record struct {
	Key   string `json:"k,omitempty"` // message.id + ":" + requestId; empty = never deduped
	TS    int64  `json:"t"`           // unix seconds
	Model string `json:"m"`
	In    int64  `json:"i"`
	Out   int64  `json:"o"`
	CC    int64  `json:"cc"`
	CR    int64  `json:"cr"`
}

type fileEntry struct {
	Size    int64    `json:"size"`
	MTime   int64    `json:"mtime"`
	Records []Record `json:"records"`
}

// Index caches parsed records per file keyed by path.
type Index map[string]fileEntry

type line struct {
	Timestamp string `json:"timestamp"`
	RequestID string `json:"requestId"`
	Message   *struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			In  int64 `json:"input_tokens"`
			Out int64 `json:"output_tokens"`
			CC  int64 `json:"cache_creation_input_tokens"`
			CR  int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// parseFile streams a JSONL file, skipping malformed lines and entries older than cutoff.
func parseFile(path string, cutoff time.Time) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var out []Record
	for sc.Scan() {
		var l line
		if json.Unmarshal(sc.Bytes(), &l) != nil || l.Message == nil || l.Message.Usage == nil {
			continue
		}
		ts, err := time.Parse(time.RFC3339, l.Timestamp)
		if err != nil || ts.Before(cutoff) || strings.HasPrefix(l.Message.Model, "<") {
			continue
		}
		key := ""
		if l.Message.ID != "" || l.RequestID != "" {
			key = l.Message.ID + ":" + l.RequestID
		}
		u := l.Message.Usage
		out = append(out, Record{Key: key, TS: ts.Unix(), Model: l.Message.Model, In: u.In, Out: u.Out, CC: u.CC, CR: u.CR})
	}
	return out, nil // scanner errors (e.g. over-long line) keep what was parsed so far
}

// retention is how far back records are kept: the start of the month or 7 days, whichever is older.
func retention(now time.Time) time.Time {
	ms := monthStart(now)
	if w := now.AddDate(0, 0, -7); w.Before(ms) {
		return w
	}
	return ms
}

func monthStart(now time.Time) time.Time {
	y, m, _ := now.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
}

// Scan reads all transcripts under dir, reusing and updating the index at indexPath
// ("" disables persistence). Records are returned undeduplicated.
func Scan(dir, indexPath string, now time.Time) ([]Record, error) {
	cutoff := retention(now)
	idx := Index{}
	if indexPath != "" {
		_ = cache.ReadJSON(indexPath, &idx) // corrupt/missing index = rebuild
	}
	next := Index{}
	var all []Record
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		st, err := d.Info()
		if err != nil {
			return nil
		}
		if st.ModTime().Before(cutoff) {
			return nil // untouched since cutoff: nothing relevant inside
		}
		e, ok := idx[p]
		if !ok || e.Size != st.Size() || e.MTime != st.ModTime().UnixNano() {
			recs, err := parseFile(p, cutoff)
			if err != nil {
				return nil
			}
			e = fileEntry{Size: st.Size(), MTime: st.ModTime().UnixNano(), Records: recs}
		}
		next[p] = e
		all = append(all, e.Records...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if indexPath != "" {
		_ = cache.WriteJSON(indexPath, next)
	}
	return all, nil
}

// Aggregate dedupes records and buckets them into block/today/week/month.
// blockStart overrides block detection when non-nil (e.g. derived from the API reset time).
func Aggregate(recs []Record, now time.Time, blockStart *time.Time) model.Tokens {
	best := map[string]Record{}
	var uniq []Record
	for _, r := range recs {
		if r.Key == "" {
			uniq = append(uniq, r)
		} else if old, ok := best[r.Key]; !ok || r.Out > old.Out {
			best[r.Key] = r // streamed messages repeat with growing output; keep the fullest
		}
	}
	for _, r := range best {
		uniq = append(uniq, r)
	}

	bs := time.Time{}
	if blockStart != nil {
		bs = *blockStart
	} else {
		bs = currentBlockStart(uniq, now)
	}
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	week := now.AddDate(0, 0, -7)
	month := monthStart(now)

	var t model.Tokens
	for _, r := range uniq {
		ts := time.Unix(r.TS, 0)
		if ts.After(now) {
			continue
		}
		if !bs.IsZero() && !ts.Before(bs) {
			t.Block5h.Add(r.Model, r.In, r.Out, r.CC, r.CR)
		}
		if !ts.Before(today) {
			t.Today.Add(r.Model, r.In, r.Out, r.CC, r.CR)
		}
		if !ts.Before(week) {
			t.Week.Add(r.Model, r.In, r.Out, r.CC, r.CR)
		}
		if !ts.Before(month) {
			t.Month.Add(r.Model, r.In, r.Out, r.CC, r.CR)
		}
	}
	return t
}

// currentBlockStart finds the start of the active 5h block (ccusage-style: the block
// opens at the hour of its first message and lasts 5h). Zero time if none is active.
func currentBlockStart(recs []Record, now time.Time) time.Time {
	ts := make([]int64, 0, len(recs))
	for _, r := range recs {
		ts = append(ts, r.TS)
	}
	slices.Sort(ts)
	var start time.Time
	for _, s := range ts {
		t := time.Unix(s, 0)
		if start.IsZero() || !t.Before(start.Add(blockLen)) {
			start = t.Truncate(time.Hour)
		}
	}
	if start.IsZero() || !now.Before(start.Add(blockLen)) {
		return time.Time{}
	}
	return start
}
