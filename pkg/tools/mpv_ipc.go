package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// mpvClient communicates with an mpv instance over a Unix socket using the
// JSON IPC protocol. Commands are JSON arrays, responses are JSON objects.
//
// If a command fails due to a stale connection, the client automatically
// reconnects to the same socket and retries once.
//
// Protocol reference: https://mpv.io/manual/master/#json-ipc
type mpvClient struct {
	conn       net.Conn
	reader     *bufio.Reader
	socketPath string // kept for reconnect
	mu         sync.Mutex
	requestID  atomic.Int64
	closed     atomic.Bool
}

// mpvCommand is the JSON structure sent to mpv.
type mpvCommand struct {
	Command   []any `json:"command"`
	RequestID int64 `json:"request_id"`
}

// mpvResponse is the JSON structure returned by mpv.
type mpvResponse struct {
	Error     string `json:"error"`
	RequestID int64  `json:"request_id"`
	Data      any    `json:"data,omitempty"`
}

// newMPVClient connects to an mpv instance via its Unix socket.
// Waits up to timeout for the socket file to appear and become connectable.
func newMPVClient(socketPath string, timeout time.Duration) (*mpvClient, error) {
	deadline := time.Now().Add(timeout)
	var conn net.Conn
	var lastErr error

	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(socketPath); os.IsNotExist(statErr) {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		conn, lastErr = net.DialTimeout("unix", socketPath, 500*time.Millisecond)
		if lastErr == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if conn == nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("socket file %s was never created", socketPath)
		}
		return nil, fmt.Errorf("failed to connect to mpv socket %s: %w", socketPath, lastErr)
	}

	c := &mpvClient{
		conn:       conn,
		reader:     bufio.NewReader(conn),
		socketPath: socketPath,
	}
	return c, nil
}

// reconnect closes the stale connection and dials a fresh one to the same socket.
// Must be called with c.mu held (or from sendCommand which holds it).
func (c *mpvClient) reconnect() error {
	// Close old connection (best effort)
	if c.conn != nil {
		c.conn.Close()
	}

	conn, err := net.DialTimeout("unix", c.socketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("reconnect failed: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	return nil
}

// sendCommand sends a JSON command to mpv and returns the response data.
// If the connection is stale, it reconnects and retries once.
func (c *mpvClient) sendCommand(args ...any) (any, error) {
	if c.closed.Load() {
		return nil, fmt.Errorf("mpv client is closed")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.sendLocked(args...)
}

// sendLocked is the internal implementation that does the actual send/receive.
// On failure it reconnects and retries once. Caller must hold c.mu.
func (c *mpvClient) sendLocked(args ...any) (any, error) {
	id := c.requestID.Add(1)
	cmd := mpvCommand{
		Command:   args,
		RequestID: id,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	// Write command
	c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.conn.Write(append(data, '\n')); err != nil {
		// Write failed — connection is dead, try reconnect + retry
		if reconnectErr := c.reconnect(); reconnectErr != nil {
			return nil, fmt.Errorf("command failed and reconnect failed: %w (original: %w)", reconnectErr, err)
		}
		return c.sendLocked(args...) // retry once on fresh connection
	}

	// Read response — mpv may send events interspersed with responses.
	// Use a short timeout so we don't block the agent for long.
	for {
		c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			// Read failed — connection is dead, try reconnect + retry
			if reconnectErr := c.reconnect(); reconnectErr != nil {
				return nil, fmt.Errorf("command failed and reconnect failed: %w (original: %w)", reconnectErr, err)
			}
			return c.sendLocked(args...) // retry once on fresh connection
		}

		var resp mpvResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // not JSON — probably an event, skip
		}

		if resp.RequestID == id {
			if resp.Error != "success" {
				return nil, fmt.Errorf("mpv error: %s", resp.Error)
			}
			return resp.Data, nil
		}
		// Different request_id — stale response, skip
	}
}

// setProperty sets an mpv property (e.g. "pause", "volume").
func (c *mpvClient) setProperty(property string, value any) error {
	_, err := c.sendCommand("set_property", property, value)
	return err
}

// getProperty gets an mpv property value.
func (c *mpvClient) getProperty(property string) (any, error) {
	return c.sendCommand("get_property", property)
}

// getMetadata returns the current track's metadata as a map.
func (c *mpvClient) getMetadata() (map[string]any, error) {
	data, err := c.getProperty("metadata")
	if err != nil {
		return nil, err
	}
	if m, ok := data.(map[string]any); ok {
		return m, nil
	}
	return nil, fmt.Errorf("unexpected metadata type: %T", data)
}

// getTimePos returns the current playback position in seconds.
func (c *mpvClient) getTimePos() (float64, error) {
	data, err := c.getProperty("time-pos")
	if err != nil {
		return 0, err
	}
	if f, ok := data.(float64); ok {
		return f, nil
	}
	return 0, fmt.Errorf("unexpected time-pos type: %T", data)
}

// getDuration returns the duration of the current track in seconds.
func (c *mpvClient) getDuration() (float64, error) {
	data, err := c.getProperty("duration")
	if err != nil {
		return 0, err
	}
	if f, ok := data.(float64); ok {
		return f, nil
	}
	return 0, fmt.Errorf("unexpected duration type: %T", data)
}

// isPaused returns whether playback is currently paused.
func (c *mpvClient) isPaused() (bool, error) {
	data, err := c.getProperty("pause")
	if err != nil {
		return false, err
	}
	if b, ok := data.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("unexpected pause type: %T", data)
}

// isEOF returns whether playback has reached the end.
func (c *mpvClient) isEOF() (bool, error) {
	data, err := c.getProperty("eof-reached")
	if err != nil {
		return false, err
	}
	if b, ok := data.(bool); ok {
		return b, nil
	}
	return false, nil
}

// quit sends the quit command to mpv to shut it down gracefully.
func (c *mpvClient) quit() error {
	if c.closed.CompareAndSwap(false, true) {
		c.sendCommand("quit") // best effort
		if c.conn != nil {
			return c.conn.Close()
		}
	}
	return nil
}

// close closes the IPC connection without sending quit.
func (c *mpvClient) close() error {
	if c.closed.CompareAndSwap(false, true) {
		if c.conn != nil {
			return c.conn.Close()
		}
	}
	return nil
}
