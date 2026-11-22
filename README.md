# tokenusage

Claude **Pro/Max** subscription usage in your terminal and status bar: 5-hour and weekly
limit percentages, reset times, and local token counts (5h block / today / 7 days / month).
Ships a Bubble Tea dashboard and JSON output for Waybar and Quickshell.

## Install

```sh
# from source
go install github.com/roman-petruzela/tokenusage/cmd/tokenusage@latest
# or: make build

# Arch (once published to the AUR)
yay -S tokenusage
```

Requires a logged-in Claude Code (`~/.claude/.credentials.json`).

## Usage

| Command | What it does |
|---------|--------------|
| `tokenusage` / `tokenusage tui` | Dashboard. `r` refresh, `?` help, `q` quit |
| `tokenusage json [--cached] [--max-age 60s]` | Normalized JSON, see [docs/schema.md](docs/schema.md) |
| `tokenusage waybar [--cached] [--max-age 60s]` | Waybar custom-module JSON |
| `tokenusage refresh` | Fetch + write cache. Exit 0 ok, 2 auth, 3 network |
| `tokenusage status` | One-line summary |
| `tokenusage config init\|path` | Write default config / print its path |

Config: `$XDG_CONFIG_HOME/tokenusage/config.toml` (`tokenusage config init`).
Cache: `$XDG_CACHE_HOME/tokenusage/`.

## Background refresh (recommended)

```sh
systemctl --user enable --now tokenusage-refresh.timer
```

Units live in `contrib/systemd/` (installed to `/usr/lib/systemd/user/` by the package).
Widgets then read the cache with `--cached` and never touch the network.

## Waybar

Merge `contrib/waybar/config.jsonc` into your config and `contrib/waybar/style.css`
into your stylesheet. Classes: `ok`, `warn`, `crit`, `error`.

## Quickshell

Copy `contrib/quickshell/ClaudeUsage.qml` into your shell config and instantiate `ClaudeUsage {}`.

## How it works / caveats

- Limits come from `https://api.anthropic.com/api/oauth/usage`, an **undocumented** endpoint
  used by Claude Code. It may change; parsing is tolerant and the JSON schema is versioned.
- Credentials are read-only. The token is **never refreshed** by this tool (that would rotate
  the refresh token and log Claude Code out). If it expires, status is `auth_expired`:
  run `claude` once.
- Token counts come from local Claude Code transcripts (`~/.claude/projects`), so they exclude
  claude.ai web/app usage.

## AUR packaging

```sh
cd packaging/aur
makepkg -si          # build + install locally
makepkg --printsrcinfo > .SRCINFO
```

Publishing (manual): clone `ssh://aur@aur.archlinux.org/tokenusage.git`, copy `PKGBUILD` and
`.SRCINFO`, commit, push.

## License

MIT
