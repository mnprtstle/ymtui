package mpris

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	busName    = "org.mpris.MediaPlayer2"
	objectPath = "/org/mpris/MediaPlayer2"
	rootIface  = "org.mpris.MediaPlayer2"
	playIface  = "org.mpris.MediaPlayer2.Player"
	propsIface = "org.freedesktop.DBus.Properties"
)

const ownName = busName + ".ymtui"

// Server owns the bus name and the exported object.
type Server struct {
	conn   *dbus.Conn
	handle func(Command)

	mu    sync.Mutex // guards props and last
	props map[string]map[string]dbus.Variant
	last  State
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

// props implements org.freedesktop.DBus.Properties. godbus ships a helper
// for this (the prop package), but it merges new values into the stored one
// in place, which leaves stale keys in the Metadata map and races with a
// client reading it. Here every value is an immutable dbus.Variant that is
// replaced wholesale under the mutex.
type props struct{ s *Server }

func (p props) Get(iface, name string) (dbus.Variant, *dbus.Error) {
	p.s.mu.Lock()
	defer p.s.mu.Unlock()
	v, ok := p.s.props[iface][name]
	if !ok {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("no such property %s.%s", iface, name))
	}
	return v, nil
}

func (p props) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	p.s.mu.Lock()
	defer p.s.mu.Unlock()
	out := make(map[string]dbus.Variant, len(p.s.props[iface]))
	for k, v := range p.s.props[iface] {
		out[k] = v
	}
	return out, nil
}

// Set only accepts Player.Volume; everything else is read-only.
func (p props) Set(iface, name string, v dbus.Variant) *dbus.Error {
	vol, ok := v.Value().(float64)
	if iface != playIface || name != "Volume" || !ok {
		return dbus.MakeFailedError(fmt.Errorf("%s.%s is not writable", iface, name))
	}
	p.s.handle(Command{Kind: SetVolume, Volume: min(max(vol, 0), 1)}) // the UI reports the applied value via SetState
	return nil
}

// send hands a command to the callback. The callback must not block: godbus
// calls our methods on its own goroutine, and the UI side just does
// program.Send.
func (s *Server) send(c Command) *dbus.Error {
	s.handle(c)
	return nil
}

// initialProps are the values before the first SetState.
func initialProps() map[string]map[string]dbus.Variant {
	v := dbus.MakeVariant
	st := State{}
	return map[string]map[string]dbus.Variant{
		rootIface: {
			"CanQuit":             v(false),
			"CanRaise":            v(false),
			"HasTrackList":        v(false),
			"Identity":            v("ymtui"),
			"SupportedUriSchemes": v([]string{}),
			"SupportedMimeTypes":  v([]string{}),
		},
		playIface: {
			"PlaybackStatus": v(st.status()),
			"LoopStatus":     v("None"),
			"Rate":           v(1.0),
			"MinimumRate":    v(1.0),
			"MaximumRate":    v(1.0),
			"Shuffle":        v(false),
			"Metadata":       v(st.metadata()),
			"Volume":         v(1.0),
			"Position":       v(int64(0)),
			"CanGoNext":      v(false),
			"CanGoPrevious":  v(false),
			"CanPlay":        v(true),
			"CanPause":       v(true),
			"CanSeek":        v(true),
			"CanControl":     v(true),
		},
	}
}

// introspectProps lists an interface's properties for the introspection XML.
func introspectProps(m map[string]dbus.Variant, writable ...string) []introspect.Property {
	var out []introspect.Property
	for name, v := range m {
		access := "read"
		for _, w := range writable {
			if w == name {
				access = "readwrite"
			}
		}
		out = append(out, introspect.Property{Name: name, Type: v.Signature().String(), Access: access})
	}
	return out
}

// Start claims the MPRIS name on the session bus. handle is called from a
// godbus goroutine for every remote command.
func Start(handle func(Command)) (*Server, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect to session bus: %w", err)
	}
	s := &Server{conn: conn, handle: handle, props: initialProps()}

	fail := func(format string, args ...any) (*Server, error) {
		conn.Close()
		return nil, fmt.Errorf(format, args...)
	}
	if err := conn.Export(root{s}, objectPath, rootIface); err != nil {
		return fail("export %s: %w", rootIface, err)
	}
	// ExportWithMap lets the Go method SeekBy answer to the D-Bus name Seek.
	if err := conn.ExportWithMap(player{s}, map[string]string{"SeekBy": "Seek"}, objectPath, playIface); err != nil {
		return fail("export %s: %w", playIface, err)
	}
	if err := conn.Export(props{s}, objectPath, propsIface); err != nil {
		return fail("export %s: %w", propsIface, err)
	}

	// Introspection XML, which busctl and friends read.
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
			{Name: propsIface, Methods: introspect.Methods(props{}), Signals: []introspect.Signal{{
				Name: "PropertiesChanged",
				Args: []introspect.Arg{
					{Name: "interface_name", Type: "s"},
					{Name: "changed_properties", Type: "a{sv}"},
					{Name: "invalidated_properties", Type: "as"},
				},
			}}},
			{Name: rootIface, Methods: introspect.Methods(root{}), Properties: introspectProps(s.props[rootIface])},
			{Name: playIface, Methods: playMethods, Properties: introspectProps(s.props[playIface], "Volume")},
		},
	}
	if err := conn.Export(introspect.NewIntrospectable(node), objectPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		return fail("export introspection: %w", err)
	}

	reply, err := conn.RequestName(ownName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fail("request name %s: %w", ownName, err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fail("name %s is already taken (is ymtui running twice?)", ownName)
	}
	return s, nil
}

// SetState publishes a new state. Only properties that changed are written,
// and a PropertiesChanged signal goes out for them (Position is never
// signalled: MPRIS says clients poll it). It is safe to call from any
// goroutine and does not wait for clients.
func (s *Server) SetState(st State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last = st

	changed := map[string]dbus.Variant{}
	set := func(name string, v any, signal bool) {
		nv := dbus.MakeVariant(v)
		if reflect.DeepEqual(s.props[playIface][name].Value(), v) {
			return
		}
		s.props[playIface][name] = nv
		if signal {
			changed[name] = nv
		}
	}
	set("PlaybackStatus", st.status(), true)
	set("Metadata", st.metadata(), true)
	set("Volume", st.Volume, true)
	set("CanGoNext", st.CanNext, true)
	set("CanGoPrevious", st.CanPrev, true)
	set("Position", st.Position.Microseconds(), false)

	if len(changed) > 0 {
		// Emit only queues the message, so holding the mutex is fine.
		_ = s.conn.Emit(objectPath, propsIface+".PropertiesChanged", playIface, changed, []string{})
	}
}

// Close releases the bus name and the connection.
func (s *Server) Close() error { return s.conn.Close() }
