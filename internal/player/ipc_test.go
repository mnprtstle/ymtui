package player

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

// fakeMPV answers each request on conn using respond, which gets the
// command array and returns the reply's error string and data.
// Before replying to a command named "emit", it sends an event.
func fakeMPV(t *testing.T, conn net.Conn, respond func(cmd []any) (string, any)) {
	t.Helper()
	go func() {
		sc := bufio.NewScanner(conn)
		for sc.Scan() {
			var req request
			if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
				t.Errorf("fake mpv: bad request %q: %v", sc.Text(), err)
				return
			}
			if req.Command[0] == "emit" {
				fmt.Fprintln(conn, `{"event":"property-change","id":1,"name":"pause","data":true}`)
			}
			errStr, data := respond(req.Command)
			line, _ := json.Marshal(map[string]any{
				"request_id": req.RequestID, "error": errStr, "data": data,
			})
			fmt.Fprintf(conn, "%s\n", line)
		}
	}()
}

func TestCommand(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		data     any
		wantData string
		wantErr  bool
	}{
		{name: "success with data", errStr: "success", data: 42.5, wantData: "42.5"},
		{name: "success no data", errStr: "success", data: nil, wantData: "null"},
		{name: "mpv error", errStr: "property not found", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			fakeMPV(t, server, func([]any) (string, any) { return tt.errStr, tt.data })
			c := newClient(client)
			defer c.Close()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got, err := c.Command(ctx, "get_property", "time-pos")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && string(got) != tt.wantData {
				t.Errorf("data = %s, want %s", got, tt.wantData)
			}
		})
	}
}

func TestEventsAndReplies(t *testing.T) {
	client, server := net.Pipe()
	fakeMPV(t, server, func([]any) (string, any) { return "success", nil })
	c := newClient(client)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := c.Command(ctx, "emit"); err != nil {
		t.Fatalf("Command: %v", err)
	}
	select {
	case ev := <-c.Events():
		if ev.Name != "property-change" || ev.Property != "pause" || string(ev.Data) != "true" {
			t.Errorf("event = %+v", ev)
		}
	case <-ctx.Done():
		t.Fatal("no event received")
	}
}

func TestCommandAfterClose(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	c := newClient(client)
	c.Close()

	_, err := c.Command(context.Background(), "get_property", "pause")
	if err == nil {
		t.Fatal("want error after Close")
	}
	if !errors.Is(err, ErrClosed) {
		t.Errorf("err = %v, want ErrClosed", err)
	}
}
