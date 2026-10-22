// Package player runs mpv as a child process and controls it over mpv's
// JSON IPC protocol: one JSON object per line over a Unix socket.
package player

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// ErrClosed is returned by Command when the connection to mpv is gone.
var ErrClosed = errors.New("mpv connection closed")

// Event is an unsolicited message from mpv, e.g. "file-loaded",
// "end-file" or "property-change".
type Event struct {
	Name      string          // "event" field
	Property  string          // property-change: property name
	Data      json.RawMessage // property-change: new value (raw JSON)
	Reason    string          // end-file: "eof", "stop", "error", ...
	FileError string          // end-file with reason "error"
}

// request is what we send. mpv takes the command as a JSON array.
type request struct {
	Command   []any `json:"command"`
	RequestID int64 `json:"request_id"`
}

// incoming covers both replies (with request_id) and events (with event).
type incoming struct {
	Event     string          `json:"event"`
	RequestID int64           `json:"request_id"`
	Error     string          `json:"error"`
	Data      json.RawMessage `json:"data"`
	Name      string          `json:"name"`
	Reason    string          `json:"reason"`
	FileError string          `json:"file_error"`
}

type reply struct {
	err  string
	data json.RawMessage
}

// Client is a connection to one mpv IPC socket. It is safe for concurrent use.
type Client struct {
	conn    net.Conn
	events  chan Event
	done    chan struct{} // closed when the read loop exits
	closing chan struct{} // closed by Close
	once    sync.Once

	writeMu sync.Mutex // serializes writes to conn

	mu      sync.Mutex // guards nextID and pending
	nextID  int64
	pending map[int64]chan reply
}

// Dial connects to the mpv socket at path, retrying until ctx is done.
// mpv creates the socket shortly after it starts, so the first attempts
// usually fail.
func Dial(ctx context.Context, path string) (*Client, error) {
	var d net.Dialer
	for {
		conn, err := d.DialContext(ctx, "unix", path)
		if err == nil {
			return newClient(conn), nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("dial mpv %s: %w", path, err)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func newClient(conn net.Conn) *Client {
	c := &Client{
		conn:    conn,
		events:  make(chan Event, 64),
		done:    make(chan struct{}),
		closing: make(chan struct{}),
		pending: make(map[int64]chan reply),
	}
	go c.readLoop()
	return c
}

// Events returns the channel of mpv events. It is closed when the
// connection ends. Callers must keep draining it: if the buffer fills up,
// the read loop blocks and command replies stall too.
func (c *Client) Events() <-chan Event { return c.events }

// Close closes the connection. Pending and future commands fail with ErrClosed.
func (c *Client) Close() error {
	var err error
	c.once.Do(func() {
		close(c.closing)
		err = c.conn.Close()
	})
	<-c.done
	return err
}

func (c *Client) readLoop() {
	defer close(c.events)
	defer close(c.done)

	sc := bufio.NewScanner(c.conn)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024) // some properties are large
	for sc.Scan() {
		var msg incoming
		if err := json.Unmarshal(sc.Bytes(), &msg); err != nil {
			continue // not something we understand; skip the line
		}
		if msg.Event != "" {
			ev := Event{
				Name:      msg.Event,
				Property:  msg.Name,
				Data:      msg.Data,
				Reason:    msg.Reason,
				FileError: msg.FileError,
			}
			select {
			case c.events <- ev:
			case <-c.closing:
				return
			}
			continue
		}
		c.mu.Lock()
		ch, ok := c.pending[msg.RequestID]
		c.mu.Unlock()
		if ok {
			ch <- reply{err: msg.Error, data: msg.Data} // buffered, never blocks
		}
	}
}

// Command sends an mpv command and waits for its reply, returning the
// reply's data field. See https://mpv.io/manual/master/#list-of-input-commands.
func (c *Client) Command(ctx context.Context, name string, args ...any) (json.RawMessage, error) {
	select {
	case <-c.done:
		return nil, fmt.Errorf("mpv %s: %w", name, ErrClosed)
	default:
	}

	ch := make(chan reply, 1)
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	line, err := json.Marshal(request{Command: append([]any{name}, args...), RequestID: id})
	if err != nil {
		return nil, fmt.Errorf("encode mpv %s: %w", name, err)
	}
	c.writeMu.Lock()
	_, err = c.conn.Write(append(line, '\n'))
	c.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("send mpv %s: %w", name, err)
	}

	select {
	case r := <-ch:
		if r.err != "success" {
			return nil, fmt.Errorf("mpv %s: %s", name, r.err)
		}
		return r.data, nil
	case <-c.done:
		return nil, fmt.Errorf("mpv %s: %w", name, ErrClosed)
	case <-ctx.Done():
		return nil, fmt.Errorf("mpv %s: %w", name, ctx.Err())
	}
}

// Load replaces the current track with url. mpv's ytdl_hook resolves
// YouTube URLs through yt-dlp.
func (c *Client) Load(ctx context.Context, url string) error {
	_, err := c.Command(ctx, "loadfile", url, "replace")
	return err
}

// SetPause pauses or resumes playback.
func (c *Client) SetPause(ctx context.Context, paused bool) error {
	return c.SetProperty(ctx, "pause", paused)
}

// Seek moves the playback position by secs (negative seeks back).
func (c *Client) Seek(ctx context.Context, secs float64) error {
	_, err := c.Command(ctx, "seek", secs, "relative")
	return err
}

// SetVolume sets the volume in percent (0–100; mpv allows up to 130).
func (c *Client) SetVolume(ctx context.Context, percent float64) error {
	return c.SetProperty(ctx, "volume", percent)
}

// SetProperty sets an mpv property.
func (c *Client) SetProperty(ctx context.Context, name string, value any) error {
	_, err := c.Command(ctx, "set_property", name, value)
	return err
}

// GetProperty reads an mpv property and decodes it into out.
func (c *Client) GetProperty(ctx context.Context, name string, out any) error {
	data, err := c.Command(ctx, "get_property", name)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode mpv property %s: %w", name, err)
	}
	return nil
}

// ObserveProperty asks mpv to send a "property-change" event whenever
// the property changes (and once right away).
func (c *Client) ObserveProperty(ctx context.Context, id int, name string) error {
	_, err := c.Command(ctx, "observe_property", id, name)
	return err
}
