// Command ymtui is a terminal YouTube Music player.
// Phase 1: a scripted demo that plays one track and drives mpv over IPC.
// Phase 2: -search prints song results from YouTube Music.
// Phase 3: with no flags, runs the TUI.
// Phase 6: playlists are stored under the XDG data dir.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mnprtstle/ymtui/internal/mpris"
	"github.com/mnprtstle/ymtui/internal/player"
	"github.com/mnprtstle/ymtui/internal/store"
	"github.com/mnprtstle/ymtui/internal/ui"
	"github.com/mnprtstle/ymtui/internal/ytmusic"
)

func main() {
	id := flag.String("id", "dQw4w9WgXcQ", "YouTube video ID to play")
	query := flag.String("search", "", "search YouTube Music for songs and print them")
	demo := flag.Bool("demo", false, "run the scripted mpv demo instead of the TUI")
	flag.Parse()

	var err error
	switch {
	case *query != "":
		err = search(*query)
	case *demo:
		err = run(*id)
	default:
		err = tui()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ymtui:", err)
		os.Exit(1)
	}
}

// tui starts mpv and runs the Bubble Tea program until the user quits.
func tui() error {
	// Debug output goes to a file: stdout belongs to the TUI.
	f, err := tea.LogToFile("debug.log", "ymtui")
	if err != nil {
		return err
	}
	defer f.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	p, err := player.Start(ctx)
	if err != nil {
		return err
	}
	defer p.Close()
	for i, prop := range ui.ObservedProperties {
		if err := p.ObserveProperty(ctx, i+1, prop); err != nil {
			return err
		}
	}

	path, err := store.DefaultPath()
	if err != nil {
		return err
	}
	playlists, err := store.Open(path)
	if err != nil {
		return err
	}

	model := ui.New(ytmusic.NewClient(), p, playlists)
	// The MPRIS callback needs the program, which needs the model, which
	// needs the server. An atomic pointer breaks the cycle safely: godbus
	// calls the callback on another goroutine, so a plain variable would be
	// a data race.
	var progRef atomic.Pointer[tea.Program]
	srv, err := mpris.Start(func(c mpris.Command) {
		if p := progRef.Load(); p != nil {
			p.Send(ui.MediaMsg{Cmd: c})
		}
	})
	if err != nil {
		// Media keys are optional (no session bus over SSH, say).
		log.Printf("mpris disabled: %v", err)
	} else {
		defer srv.Close()
		model = model.WithMedia(srv)
	}
	prog := tea.NewProgram(model, tea.WithAltScreen())
	progRef.Store(prog)
	go ui.Pump(prog, p.Events())
	_, err = prog.Run()
	return err
}

func search(query string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	songs, err := ytmusic.NewClient().Search(ctx, query)
	if err != nil {
		return err
	}
	for _, s := range songs {
		fmt.Printf("%-12s %s\n", s.VideoID, s)
	}
	return nil
}

func run(id string) error {
	// ctx is cancelled on Ctrl+C or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	p, err := player.Start(ctx)
	if err != nil {
		return err
	}
	defer p.Close()

	for i, prop := range []string{"media-title", "pause", "volume"} {
		if err := p.ObserveProperty(ctx, i+1, prop); err != nil {
			return err
		}
	}

	url := "https://music.youtube.com/watch?v=" + id
	fmt.Println("loading", url, "(yt-dlp takes a few seconds)")
	if err := p.Load(ctx, url); err != nil {
		return err
	}
	if err := watch(ctx, p.Events(), 30*time.Second, "file-loaded"); err != nil {
		return err
	}

	steps := []struct {
		label string
		do    func() error
		wait  time.Duration
	}{
		{"playing", func() error { return nil }, 5 * time.Second},
		{"pause", func() error { return p.SetPause(ctx, true) }, 2 * time.Second},
		{"resume", func() error { return p.SetPause(ctx, false) }, time.Second},
		{"seek +60s", func() error { return p.Seek(ctx, 60) }, 5 * time.Second},
		{"volume 50%", func() error { return p.SetVolume(ctx, 50) }, 3 * time.Second},
	}
	for _, s := range steps {
		if err := s.do(); err != nil {
			return err
		}
		var pos float64
		if err := p.GetProperty(ctx, "time-pos", &pos); err != nil {
			return err
		}
		fmt.Printf("» %-10s at %5.1fs\n", s.label, pos)
		if err := watch(ctx, p.Events(), s.wait, ""); err != nil {
			return err
		}
	}
	fmt.Println("done")
	return nil
}

// watch prints events for up to d. If until is non-empty, it returns as soon
// as that event arrives and fails if it doesn't arrive in time.
func watch(ctx context.Context, events <-chan player.Event, d time.Duration, until string) error {
	timeout := time.After(d)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return errors.New("mpv exited")
			}
			printEvent(ev)
			if ev.Name == "end-file" && ev.Reason == "error" {
				return fmt.Errorf("playback failed: %s", ev.FileError)
			}
			if ev.Name == until {
				return nil
			}
		case <-timeout:
			if until != "" {
				return fmt.Errorf("timed out waiting for %s", until)
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func printEvent(ev player.Event) {
	switch {
	case ev.Property != "":
		fmt.Printf("  event %s %s=%s\n", ev.Name, ev.Property, ev.Data)
	case ev.Reason != "":
		fmt.Printf("  event %s reason=%s\n", ev.Name, ev.Reason)
	default:
		fmt.Printf("  event %s\n", ev.Name)
	}
}
