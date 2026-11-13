package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mnprtstle/ymtui/internal/mpris"
)

type fakeMedia struct{ last mpris.State }

func (f *fakeMedia) SetState(s mpris.State) { f.last = s }

func TestMediaCommands(t *testing.T) {
	tests := []struct {
		name     string
		cmd      mpris.Command
		wantLoad []string // everything fakePlayer has loaded, including the first song
		wantVol  float64
	}{
		{"next plays the second song", mpris.Command{Kind: mpris.Next}, []string{"a", "b"}, 100},
		{"previous on the first song restarts it", mpris.Command{Kind: mpris.Previous}, []string{"a", "a"}, 100},
		{"volume is 0..1 scaled to percent", mpris.Command{Kind: mpris.SetVolume, Volume: 0.4}, []string{"a"}, 40},
		{"pause does not load", mpris.Command{Kind: mpris.PlayPause}, []string{"a"}, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, fp := loaded(t)
			fm := &fakeMedia{}
			m = m.WithMedia(fm)

			// Enter plays the first row and queues the whole list.
			next, cmd := m.Update(key("enter"))
			m = next.(Model)
			exec(cmd)

			next, cmd = m.Update(MediaMsg{tt.cmd})
			m = next.(Model)
			exec(cmd)

			got := make([]string, 0, len(fp.loaded))
			for _, u := range fp.loaded {
				got = append(got, u[len(u)-1:]) // last char of ...?v=<id>
			}
			if len(got) != len(tt.wantLoad) {
				t.Fatalf("loaded %v, want %v", got, tt.wantLoad)
			}
			for i := range got {
				if got[i] != tt.wantLoad[i] {
					t.Errorf("loaded %v, want %v", got, tt.wantLoad)
				}
			}
			if m.volume != tt.wantVol {
				t.Errorf("volume = %v, want %v", m.volume, tt.wantVol)
			}
			if !fm.last.HasTrack {
				t.Error("state not published after a song started")
			}
		})
	}
}

func TestMediaIgnoredWithoutTrack(t *testing.T) {
	m, fp := loaded(t)
	for _, k := range []mpris.CommandKind{mpris.Play, mpris.Pause, mpris.PlayPause, mpris.SeekBy} {
		_, cmd := m.Update(MediaMsg{mpris.Command{Kind: k, Offset: time.Second}})
		if cmd != nil {
			t.Errorf("command %v with nothing loaded returned a Cmd", k)
		}
	}
	if len(fp.loaded) != 0 {
		t.Errorf("loaded %v, want none", fp.loaded)
	}
}

func TestMediaStateFollowsModel(t *testing.T) {
	m, _ := loaded(t)
	fm := &fakeMedia{}
	m = m.WithMedia(fm)
	_, cmd := m.Update(key("enter"))
	exec(cmd)
	if s := fm.last; !s.HasTrack || s.Title != "A" || s.VideoID != "a" || !s.CanNext {
		t.Errorf("state = %+v", s)
	}
}

// exec runs a Cmd synchronously, which is fine for tests with fake I/O.
func exec(cmd tea.Cmd) {
	if cmd != nil {
		cmd()
	}
}
