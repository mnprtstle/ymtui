package mpris

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

const (
	busName    = "org.mpris.MediaPlayer2.ymtui"
	objectPath = "/org/mpris/MediaPlayer2"
	rootIface  = "org.mpris.MediaPlayer2"
	playIface  = "org.mpris.MediaPlayer2.Player"
)

// Server owns the bus name and the exported object.
type Server struct {
	conn   *dbus.Conn
	props  *prop.Properties
	handle func(Command)

	mu   sync.Mutex // guards last
	last State
}

// root implements org.mpris.MediaPlayer2. Go exports a value's methods to
// D-Bus: godbus matches exported method names to the D-Bus method names, and
// a trailing *dbus.Error return reports failures to the caller.
type root struct{ s *Server }

func (root) Raise() *dbus.Error { return nil }
func (root) Quit() *dbus.Error  { return nil } // CanQuit is false

// player implements org.mpris.MediaPlayer2.Player.
type player struct{ s *Server }

func (p player) Play() *dbus.Error      { return p.s.send(Command{Kind: Play}) }
func (p player) Pause() *dbus.Error     { return p.s.send(Command{Kind: Pause}) }
func (p player) PlayPause() *dbus.Error { return p.s.send(Command{Kind: PlayPause}) }
func (p player) Stop() *dbus.Error      { return p.s.send(Command{Kind: Stop}) }
func (p player) Next() *dbus.Error      { return p.s.send(Command{Kind: Next}) }
func (p player) Previous() *dbus.Error  { return p.s.send(Command{Kind: Previous}) }

// SeekBy is exported to D-Bus as "Seek" (see Start). It can't be named Seek
// in Go because go vet reserves that name for io.Seeker's signature.
// The argument is a relative offset in microseconds.
func (p player) SeekBy(offset int64) *dbus.Error {
	return p.s.send(Command{Kind: SeekBy, Offset: time.Duration(offset) * time.Microsecond})
}

// SetPosition is absolute; we turn it into a relative seek from the last
// position we were told about.
func (p player) SetPosition(track dbus.ObjectPath, pos int64) *dbus.Error {
	p.s.mu.Lock()
	cur := p.s.last
	p.s.mu.Unlock()
	if !cur.HasTrack || track != trackPath(cur.VideoID) {
		return nil // the spec says to ignore a stale track ID
	}
	return p.s.send(Command{Kind: SeekBy, Offset: time.Duration(pos)*time.Microsecond - cur.Position})
}

func (player) OpenUri(string) *dbus.Error { return nil }

// send hands a command to the callback. The callback must not block: godbus
// calls our methods on its own goroutine, and the UI side just does
// program.Send.
func (s *Server) send(c Command) *dbus.Error {
	s.handle(c)
	return nil
}

// Start claims the MPRIS name on the session bus. handle is called from a
// godbus goroutine for every remote command.
func Start(handle func(Command)) (*Server, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect to session bus: %w", err)
	}
	s := &Server{conn: conn, handle: handle}

	// Properties: the map is interface -> name -> value. EmitTrue sends a
	// PropertiesChanged signal on change; Position must not (MPRIS says
	// clients poll it).
	ro := func(v any, emit prop.EmitType) *prop.Prop {
		return &prop.Prop{Value: v, Writable: false, Emit: emit}
	}
	s.props, err = prop.Export(conn, objectPath, map[string]map[string]*prop.Prop{
		rootIface: {
			"CanQuit":             ro(false, prop.EmitFalse),
			"CanRaise":            ro(false, prop.EmitFalse),
			"HasTrackList":        ro(false, prop.EmitFalse),
			"Identity":            ro("ymtui", prop.EmitFalse),
			"SupportedUriSchemes": ro([]string{}, prop.EmitFalse),
			"SupportedMimeTypes":  ro([]string{}, prop.EmitFalse),
		},
		playIface: {
			"PlaybackStatus": ro("Stopped", prop.EmitTrue),
			"LoopStatus":     ro("None", prop.EmitFalse),
			"Rate":           ro(1.0, prop.EmitFalse),
			"MinimumRate":    ro(1.0, prop.EmitFalse),
			"MaximumRate":    ro(1.0, prop.EmitFalse),
			"Shuffle":        ro(false, prop.EmitFalse),
			"Metadata":       ro(State{}.metadata(), prop.EmitTrue),
			"Volume": {
				Value:    1.0,
				Writable: true,
				Emit:     prop.EmitTrue,
				Callback: func(c *prop.Change) *dbus.Error {
					if v, ok := c.Value.(float64); ok {
						s.handle(Command{Kind: SetVolume, Volume: min(max(v, 0), 1)})
					}
					return nil // the UI reports the applied volume via SetState
				},
			},
			"Position":      ro(int64(0), prop.EmitFalse),
			"CanGoNext":     ro(false, prop.EmitTrue),
			"CanGoPrevious": ro(false, prop.EmitTrue),
			"CanPlay":       ro(true, prop.EmitFalse),
			"CanPause":      ro(true, prop.EmitFalse),
			"CanSeek":       ro(true, prop.EmitFalse),
			"CanControl":    ro(true, prop.EmitFalse),
		},
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("export properties: %w", err)
	}

	// Methods, plus the introspection XML that tools like busctl read.
	if err := conn.Export(root{s}, objectPath, rootIface); err != nil {
		conn.Close()
		return nil, fmt.Errorf("export %s: %w", rootIface, err)
	}
	if err := conn.ExportWithMap(player{s}, map[string]string{"SeekBy": "Seek"}, objectPath, playIface); err != nil {
		conn.Close()
		return nil, fmt.Errorf("export %s: %w", playIface, err)
	}
	playMethods := introspect.Methods(player{})
	for i := range playMethods {
		if playMethods[i].Name == "SeekBy" {
			playMethods[i].Name = "Seek"
		}
	}
	node := &introspect.Node{
		Name: objectPath,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{Name: rootIface, Methods: introspect.Methods(root{}), Properties: s.props.Introspection(rootIface)},
			{Name: playIface, Methods: playMethods, Properties: s.props.Introspection(playIface)},
		},
	}
	if err := conn.Export(introspect.NewIntrospectable(node), objectPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("export introspection: %w", err)
	}

	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("request name %s: %w", busName, err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		conn.Close()
		return nil, fmt.Errorf("name %s is already taken (is ymtui running twice?)", busName)
	}
	return s, nil
}

// SetState publishes a new state. Only properties that changed are written,
// so clients get PropertiesChanged signals just when something is new.
// It is cheap (no waiting on clients) and safe to call from any goroutine.
func (s *Server) SetState(st State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.last
	s.last = st

	set := func(name string, v any) { s.props.SetMust(playIface, name, v) }
	if st.status() != old.status() {
		set("PlaybackStatus", st.status())
	}
	if !reflect.DeepEqual(st.metadata(), old.metadata()) {
		set("Metadata", st.metadata())
	}
	if st.Volume != old.Volume {
		set("Volume", st.Volume)
	}
	if st.CanNext != old.CanNext {
		set("CanGoNext", st.CanNext)
	}
	if st.CanPrev != old.CanPrev {
		set("CanGoPrevious", st.CanPrev)
	}
	if st.Position != old.Position {
		set("Position", st.Position.Microseconds())
	}
}

// Close releases the bus name and the connection.
func (s *Server) Close() error { return s.conn.Close() }
