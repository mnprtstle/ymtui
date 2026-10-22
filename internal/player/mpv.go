package player

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Player is a running mpv process plus an IPC connection to it.
// The embedded *Client makes its methods (Load, SetPause, ...) available
// directly on Player.
type Player struct {
	*Client
	cmd *exec.Cmd
	dir string // temp dir holding the socket
}

// Start launches mpv in idle mode and connects to its IPC socket.
// ctx bounds startup only; the process lives until Close.
func Start(ctx context.Context) (*Player, error) {
	dir, err := os.MkdirTemp("", "ymtui-")
	if err != nil {
		return nil, fmt.Errorf("create socket dir: %w", err)
	}
	sock := filepath.Join(dir, "mpv.sock")

	// Not exec.CommandContext: that would kill mpv as soon as the startup
	// ctx ends. We stop it ourselves in Close.
	cmd := exec.Command("mpv",
		"--idle",
		"--no-video",
		"--no-terminal", // mpv must not touch our stdin/stdout
		"--input-ipc-server="+sock,
		"--ytdl-format=bestaudio",
	)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("start mpv: %w", err)
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client, err := Dial(dialCtx, sock)
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		os.RemoveAll(dir)
		return nil, err
	}
	return &Player{Client: client, cmd: cmd, dir: dir}, nil
}

// Close asks mpv to quit, kills it if it doesn't exit within two seconds,
// and removes the socket.
func (p *Player) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	p.Command(ctx, "quit") // mpv may exit before replying; ignore the error
	p.Client.Close()

	exited := make(chan error, 1)
	go func() { exited <- p.cmd.Wait() }()
	var err error
	select {
	case err = <-exited:
	case <-time.After(2 * time.Second):
		p.cmd.Process.Kill()
		err = <-exited
	}
	os.RemoveAll(p.dir)
	if err != nil {
		return fmt.Errorf("mpv exit: %w", err)
	}
	return nil
}
