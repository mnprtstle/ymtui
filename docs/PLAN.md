# ymtui — YouTube Music TUI player: project plan

## Context
`/home/milan/golearn/ymtui` is empty. Goal: a keyboard-first (vim-style) terminal YouTube Music player in Go + Bubble Tea, using yt-dlp + mpv. This is the user's first Go project. This plan covers the stack choices, how the project is laid out, and a milestone order. **No setup or code yet.**

Environment found: Go 1.26.5, `mpv`, `deno` and `yt-dlp` installed (Fedora, Linux).---

## 1. Features → what each one needs
| Feature | Provided by |
|---|---|
| Search songs / albums / artists / playlists | metadata layer (see §2, choice A) |
| Browse album tracks, artist page (top songs, albums) | metadata layer, "browse" endpoints |
| Play / pause / seek / volume / next / prev | mpv over JSON IPC |
| Queue & local playlists | our own Go code + JSON files on disk |
| OS media keys / desktop widgets | MPRIS over D-Bus (Linux) |
| Vim-style UI | Bubble Tea + Bubbles + Lip Gloss |

## 2. Stack decisions (with alternatives)

### A. Metadata (search, albums, artists): the biggest decision
1. **Talk to YouTube Music's internal "InnerTube" API from Go (recommended).** POST JSON to `https://music.youtube.com/youtubei/v1/search` and `/browse` with a WEB_REMIX client context. This is the same API the Python library `ytmusicapi` uses, and its source is the reference for request shapes and response parsing. Pros: fast (one HTTP call), structured results for songs/albums/artists, pure Go, a good way to learn `net/http` and `encoding/json`. Cons: the responses are deeply nested and undocumented, and YouTube can change them.
2. **yt-dlp only.** Search with `yt-dlp --flat-playlist -J "https://music.youtube.com/search?q=..."` and read albums/artists by URL. Pros: no reverse-engineering. Cons: each call spawns a process (about 1–3 s), and the metadata for artist/album structure is thin.
3. **Shell out to Python `ytmusicapi`** through a small helper script. Pros: the most complete and well-maintained option. Cons: adds a Python dependency, so the app is no longer a single Go binary.

→ Use **option 1**, and keep it behind a Go `interface` so you can switch to option 2 or 3 later if needed.

**What yt-dlp is still for.** InnerTube gives *metadata* (titles, IDs, albums, artists). It does **not** give a playable audio URL. YouTube scrambles stream URLs with frequently-changing player JS and anti-bot tokens. yt-dlp keeps up with that, and that JS is why it needs deno. So the responsibilities split like this:
- Go → InnerTube: search, browse, video IDs.
- mpv → yt-dlp (via mpv's built-in `ytdl_hook`): video ID → audio stream → playback.
- Later option: call `yt-dlp -g -f bestaudio <id>` from Go ourselves, to pre-resolve the next track (gapless playback), cache URLs, and show clearer errors.

### B. Playback
1. **mpv as a child process + JSON IPC (recommended).** Start it with `mpv --idle --no-video --input-ipc-server=<socket> --ytdl-format=bestaudio`. mpv's built-in ytdl hook calls yt-dlp itself, so we only send `loadfile https://music.youtube.com/watch?v=ID`. We send commands (`set_property pause`, `seek`, `volume`) and receive events (`time-pos`, `end-file`) through the socket.
   - No hook to write: `ytdl_hook.lua` ships inside mpv and is on by default (`--ytdl=yes`). When `loadfile` gets a web URL, mpv runs yt-dlp itself. Our only job is choosing startup flags: `--ytdl-format=bestaudio`, and `--script-opts=ytdl_hook-ytdl_path=yt-dlp` if it needs to be explicit. Sanity check from a shell: `mpv --no-video 'https://music.youtube.com/watch?v=<id>'`.
   - Client: write a small one yourself (about 150 lines, good practice with `net`, goroutines, and channels) or use `github.com/DexterLB/mpvipc`.
2. **libmpv through cgo** (`github.com/gen2brain/go-mpv`). Tighter integration, but cgo makes builds harder. Not recommended for a first project.

### C. TUI
- `github.com/charmbracelet/bubbletea`: Elm architecture (Model/Update/View).
- `github.com/charmbracelet/bubbles`: `list`, `textinput`, `viewport`, `progress`, `help`, `key` (keybinding definitions plus an auto-generated help view).
- `github.com/charmbracelet/lipgloss`: styling and layout.
- **v1 vs v2:** Bubble Tea, Bubbles, and Lip Gloss v2 have major versions under the `charm.land/...` import paths. Most tutorials and examples online are still v1. Decide at setup by checking which one is current and stable. The architecture below is the same either way.

### D. OS media keys
1. **Implement MPRIS yourself** with `github.com/godbus/dbus/v5` (recommended later). This exposes `org.mpris.MediaPlayer2.Player` so GNOME/KDE media keys, `playerctl`, and the lock-screen widget control the app and show the correct title and artist.
2. **Shortcut:** load the `mpv-mpris` plugin into mpv (`--script=/usr/lib64/mpv/mpris.so`, from the Fedora package `mpv-mpris`). It works almost for free, but mpv owns the metadata and controls, and next/prev won't know about our queue.
→ Start with 2 for a quick win, then replace it with 1.

### E. Storage and config
- Playlists, queue, and history as **JSON files** in `$XDG_DATA_HOME/ymtui/` (from `os.UserConfigDir()` / `github.com/adrg/xdg`). This is simple and readable. If it outgrows that, move to pure-Go SQLite (`modernc.org/sqlite`, no cgo).
- Config (keybinds, theme, default volume) as TOML via `github.com/BurntSushi/toml`.

## 3. Go project setup (when we start)
```
go mod init github.com/<you>/ymtui      # module path = future repo URL
git init
```
Tooling: `gopls` (editor LSP), `gofmt`/`goimports`, `go vet`, `golangci-lint`, `go test ./...`, and optionally `delve` for debugging. Bubble Tea owns stdout, so debug-log to a file with `tea.LogToFile`.

Layout (standard Go convention: `cmd/` holds binaries and `internal/` holds private packages):
```
ymtui/
  cmd/ymtui/main.go          # wiring only: config, start mpv, start tea.Program
  internal/
    ytmusic/                 # InnerTube client: Search, Album, Artist, types (Track, Album, Artist)
    player/                  # mpv process + IPC client; Play/Pause/Seek/Volume; event channel
    queue/                   # queue logic (next/prev/shuffle/repeat): pure, unit-testable
    store/                   # playlist + config persistence (JSON/TOML)
    mpris/                   # D-Bus MPRIS server
    ui/                      # root model + sub-models
      app.go  keys.go  styles.go
      search.go  results.go  album.go  artist.go  queue.go  playlists.go  nowplaying.go
  go.mod  go.sum  Makefile (optional)  README.md
```

## 4. Architecture
- **Root model** holds a *view stack* (Search → Artist → Album …). `l`/Enter pushes a view and `h`/Esc pops it, like browsing directories. A persistent **now-playing bar** sits at the bottom.
- **Never block in `Update`.** Network calls and mpv commands run as `tea.Cmd`s that return messages (`searchResultsMsg`, `albumLoadedMsg`, `errMsg`).
- **mpv events → UI:** a goroutine reads the IPC socket and calls `program.Send(playerEventMsg{...})`. Alternatively, use a Cmd that waits on a channel and re-arms itself after each message.
- **MPRIS → app:** D-Bus method calls (PlayPause, Next) go through the same `program.Send`. The OS and the keyboard therefore follow one code path.
- Package rule: `ui` depends on `ytmusic`/`player`/`queue`/`store`, never the reverse.

### Layout: sidebar + main pane (chosen)
```
╭─ ymtui ───────────╮╭─ Search · radiohead ──── Songs  Albums  Artists ─╮
│ ▸ Search          ││   #  Title             Artist      Album   Time  │
│   Queue        12 ││ ♪ 1  Creep             Radiohead   Pablo   3:58  │
│   Liked           ││ ▌ 2  Karma Police      Radiohead   OK Co…  4:24  │
│                   ││   3  No Surprises      Radiohead   OK Co…  3:49  │
│ PLAYLISTS         ││   4  Paranoid Android  Radiohead   OK Co…  6:27  │
│   late night   24 ││   5  Reckoner          Radiohead   In Ra…  4:50  │
│   focus        51 ││                                                  │
│   road trip    18 ││                                           5/20   │
╰───────────────────╯╰──────────────────────────────────────────────────╯
╭──────────────────────────────────────────────────────────────────────╮
│  Creep                                       ⇄ shuffle  ↻ all  70%  │
│  Radiohead · Pablo Honey                                             │
│  2:01 ━━━━━━━━━━━━━━━━━━━━━━━━●───────────────────────────────  3:58 │
╰──────────────────────────────────────────────────────────────────────╯
 NORMAL   j/k move  l open  / search  space play  ? help
```
Design details:
- **Panes:** the sidebar (Search, Queue, Liked, playlists with track counts) and the main pane. The focused pane's border uses the accent color; the unfocused one is dimmed. `Tab` or `ctrl+h/l` moves focus.
- **Main pane:** a table with columns `#`, Title, Artist, Album, Time. Columns truncate with `…` and are sized from the terminal width. In narrow terminals Album is dropped first, then Artist.
- **Row markers:** `▌` + highlight marks the cursor row; `♪` marks the currently playing track.
- **Pane title:** shows a breadcrumb (`Radiohead › OK Computer`) when drilling into an artist or album. On search, the active result tab (Songs/Albums/Artists) is highlighted.
- **Player bar:** title in bold, artist · album dimmed, a progress bar that fills the full width, shuffle/repeat/volume state.
- **Status line:** mode indicator (NORMAL / SEARCH / COMMAND) plus context-sensitive key hints. Errors appear here as short toasts.
- **Narrow terminal (< ~90 cols):** the sidebar collapses and `1–4` switch views instead.
- **Theme:** an adaptive palette that works on light and dark terminals, with one accent color, configurable later. No Nerd Font icons required; optional via config.

Default keys: `j/k` move, `gg/G` top/bottom, `ctrl+d/u` half page, `l`/Enter open/play, `h`/Esc back, `/` search, `tab` switch result type (songs/albums/artists), `space` play/pause, `n/N` next/prev track, `H/L` or `←/→` seek ±5 s, `+/-` volume, `a` add to queue, `A` add to playlist, `1–4` jump to Search/Queue/Playlists/Library, `?` help, `q` quit. Later: `:` command line (`:w playlist-name`).

## 5. Milestones (each one leaves something runnable)
1. **Hello mpv:** a plain Go CLI that starts mpv, plays a hard-coded video ID, and pauses/seeks over IPC. You learn processes, sockets, JSON, and goroutines.
   - **Status: done** (branch `phase/01-hello-mpv`). Own IPC client in `internal/player` (no `mpvipc` dependency); `go run ./cmd/ymtui [-id VIDEO_ID]` runs the scripted demo.
   - Decisions: mpv is started with `exec.Command` (not `CommandContext`) and stopped by `Player.Close`; `--no-terminal` keeps mpv off our stdin/stdout (add `--log-file` with debug logging later).
   - Runtime dep note: yt-dlp needs its EJS challenge-solver scripts, not just deno. On Fedora install `yt-dlp+default`; without it YouTube returns 403 on the audio stream.
2. **Search in Go:** an InnerTube search that prints songs to stdout, plus unit tests against saved JSON fixtures.
   - **Status: done** (branch `phase/02-search`). `internal/ytmusic`: `Client.Search` plus a pure `parseSongs`; `go run ./cmd/ymtui -search "query"` prints songs.
   - Decisions: plain `net/http` (no auth, cookie `SOCS=CAI`, client `WEB_REMIX`); songs filter param `EgWKAQIIAWoMEA4QChADEAQQCRAF`. The fixture is a real response trimmed to 6 rows. Artist/album/time are split from one subtitle column on `" • "`; the video ID comes from `playlistItemData.videoId`.
3. **First TUI:** search box, results list, Enter plays, now-playing bar with a progress ticker.
4. **Queue:** add/next/prev, auto-advance on mpv `end-file`, shuffle/repeat.
5. **Browse:** album and artist views with the view stack and `h/l` navigation.
6. **Local playlists:** create, add, remove, play, saved as JSON.
7. **Media keys:** start with mpv-mpris, then write our own MPRIS server.
8. **Polish:** config file, theming, help overlay, error toasts, resume last session.

## 6. Verification (per milestone)
- `go vet ./... && go test ./...`. `queue` and the `ytmusic` parsers get table-driven tests using recorded JSON fixtures.
- Manual: `go run ./cmd/ymtui`, search, play, seek, change volume, then check that `playerctl status` / `playerctl next` and the hardware media keys work once MPRIS is in.
- Debug log: `tail -f debug.log` while the TUI is running.

## 7. Known risks
- YouTube changes InnerTube responses or adds anti-bot measures. Keep the parsing isolated in `ytmusic/` and keep yt-dlp updated (`yt-dlp -U` or dnf).
- yt-dlp needs a JS runtime (deno) for YouTube.
- Some tracks are age- or region-restricted. Show an error instead of crashing.
- Linux-first. On macOS/Windows, MPRIS and the Unix socket would need different implementations.

## 8. Learning approach: just-in-time
Learn each concept at the start of the phase that uses it, as a short primer before any code. Don't study ahead.

| Phase | Learn right before |
|---|---|
| 1 Hello mpv | `os/exec`, Unix sockets (`net`), goroutines + channels, JSON-per-line IPC protocol |
| 2 Search | `net/http`, `encoding/json` structs and nested/unknown shapes, InnerTube request/response (read ytmusicapi source as reference) |
| 3-5 TUI | Elm architecture (Model/Update/View), `tea.Cmd` / messages, Lip Gloss layout |
| 7 Media keys | D-Bus basics, MPRIS interface (the `org.mpris.MediaPlayer2.Player` methods and properties) |

Worth knowing before phase 1: Go error handling (`if err != nil`), interfaces, packages/modules, and a first look at goroutines and channels.


