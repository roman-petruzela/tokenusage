# JSON schema (version 1)

Printed by `tokenusage json`, stored in `$XDG_CACHE_HOME/tokenusage/usage.json`.
Adding fields is non-breaking; removing or changing one bumps `schema`.

```json
{
  "schema": 1,
  "updated_at": "2026-01-01T12:00:00Z",
  "status": "ok",
  "error": "",
  "plan": {"subscription": "max", "tier": "default_claude_max_20x"},
  "limits": {
    "five_hour":        {"utilization": 42.0, "resets_at": "2026-01-01T14:00:00Z", "resets_in_seconds": 7980},
    "seven_day":        {"utilization": 10.0, "resets_at": null, "resets_in_seconds": 0},
    "seven_day_opus":   null,
    "seven_day_sonnet": null
  },
  "tokens": {
    "block_5h": {"input": 0, "output": 0, "cache_creation": 0, "cache_read": 0, "total": 0,
                 "by_model": {"claude-sonnet-5-5": {"input": 0, "output": 0, "cache_creation": 0, "cache_read": 0, "total": 0}}},
    "today": {}, "week": {}, "month": {}
  }
}
```

## Fields

| Field | Meaning |
|-------|---------|
| `status` | `ok`, `auth_expired`, `network_error`, `stale` (cache older than the max age and refetch failed) |
| `error` | Human readable message, empty when `ok` |
| `limits.*` | `null` when the API does not report that window |
| `limits.*.utilization` | Percent 0–100 |
| `limits.*.resets_at` | RFC3339 or `null` |
| `limits.*.resets_in_seconds` | Seconds from `updated_at`-time of rendering; 0 if unknown/past |
| `tokens.block_5h` | Local Claude Code usage in the current 5h block |
| `tokens.week` | Last 7 days; `month` is the current calendar month (local time) |

On fetch failure the last good `limits` are kept and only `status`/`error` change.
Token counts come from local Claude Code transcripts only (not claude.ai web/app usage).
