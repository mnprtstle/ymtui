# ymtui

Keyboard-first (vim-style) YouTube Music player for the terminal, written in Go with Bubble Tea.
**The full plan, stack decisions and milestones are in `docs/PLAN.md`. Read the current phase's section before starting work.**

## How we work
- The user is new to Go but not to programming. Write idiomatic Go and explain *Go-specific* things (goroutines, interfaces, error wrapping, module layout) briefly when they first appear. Skip general programming basics.
- Learning is just-in-time: at the start of each phase, give a short primer on the concepts that phase introduces (see PLAN.md §8), then build.
- At the start of each phase, ask whether the user wants to type the code themselves (Claude guides and reviews) or have Claude write it (with explanation).
- Prefer the standard library and small, readable code over abstractions. Don't add dependencies that aren't in PLAN.md without asking.
- When a phase is done, update its status in `docs/PLAN.md` and note any decision that changed.

## Git rules (strict)
- **Never commit on `main`.** Each phase lives on its own branch: `phase/NN-short-name` (e.g. `phase/01-hello-mpv`), branched from an up-to-date `main`.
- Make small, focused commits with imperative messages ("Add mpv IPC client").
- **Never merge, rebase onto, or push to `main`.** The user reviews and merges with `git merge --no-ff`.
- A pre-commit hook rejects commits on `main`. Don't bypass it with `--no-verify`.

## Stack
- Go (see `go.mod` for the version), Bubble Tea + Bubbles + Lip Gloss
- Metadata: YouTube Music InnerTube API (`/youtubei/v1/search`, `/browse`), called from Go in `internal/ytmusic`. Use `ytmusicapi` (Python) source as the reference for request and response shapes.
- Playback: `mpv --idle --no-video --input-ipc-server=<sock> --ytdl-format=bestaudio`, controlled over JSON IPC. mpv's built-in ytdl_hook runs yt-dlp; we only `loadfile` music.youtube.com URLs.
- Media keys: MPRIS over D-Bus (`godbus/dbus/v5`).
- Storage: JSON in the XDG data dir; config as TOML in the XDG config dir.
- Runtime deps: `mpv`, `yt-dlp`, `deno` (yt-dlp's JS runtime).

## Layout
- `cmd/ymtui/`: main, wiring only
- `internal/ytmusic`: InnerTube client and parsers (all YouTube response-shape knowledge lives here)
- `internal/player`: mpv process and IPC
- `internal/queue`: pure queue logic
- `internal/store`: persistence
- `internal/mpris`: D-Bus
- `internal/ui`: Bubble Tea models
- Dependency direction: `ui` → everything else; never the reverse.

## Conventions
- Run `gofmt`/`goimports`. Wrap errors with context: `fmt.Errorf("search %q: %w", q, err)`. Never panic on network or IPC errors.
- Pass `context.Context` to anything that does I/O.
- Bubble Tea: never block in `Update`. I/O goes in `tea.Cmd`s that return typed messages. mpv and MPRIS events reach the UI via `program.Send`.
- UI follows the layout in PLAN.md §4: sidebar + main pane + player bar + status line, an adaptive light/dark palette with one accent color, and no Nerd Font requirement.
- Tests: table-driven. Parsers are tested against recorded JSON in `internal/ytmusic/testdata/`. Tests never hit the network.
- Debug logging goes to a file (`tea.LogToFile`), never stdout.

## Commands
- `go run ./cmd/ymtui`: run
- `go build -o bin/ymtui ./cmd/ymtui`: build
- `go vet ./... && go test ./...`: check before every commit
- `golangci-lint run`: lint (if installed)
