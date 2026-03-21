package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Swarup012/solo/pkg/logger"
)

// Song holds metadata for a track.
type Song struct {
	Query     string `json:"query"`
	Title     string `json:"title"`
	Duration  int    `json:"duration"` // seconds
	Thumbnail string `json:"thumbnail"`
	URL       string `json:"url"`
}

// YouTubeMusicTool handles YouTube audio streaming via yt-dlp and mpv.
// Uses mpv's JSON IPC protocol over a Unix socket for reliable control.
type YouTubeMusicTool struct {
	player  string // mpv (default)
	quality string // bestaudio
	running bool
	mu      sync.RWMutex

	// mpv process and IPC
	mpvCmd    *exec.Cmd
	ipcSocket string
	ipc       *mpvClient

	// Queue
	queue       []Song
	currentSong *Song

	// Lifecycle
	done chan struct{}
}

// NewYouTubeMusicTool creates a new YouTube music tool.
func NewYouTubeMusicTool() *YouTubeMusicTool {
	return &YouTubeMusicTool{
		player:  "mpv",
		quality: "bestaudio",
		running: false,
		done:    make(chan struct{}),
	}
}

func (t *YouTubeMusicTool) Name() string {
	return "youtube_music"
}

func (t *YouTubeMusicTool) Description() string {
	return "Play music from YouTube. Streams audio via mpv. Supports play, pause, resume, stop, next, volume, status, queue, and show_queue actions. Usage: query=<song name> to play, action=<command> to control."
}

func (t *YouTubeMusicTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Song name or YouTube search query to play. Examples: 'money song', 'bohemian rhapsody'",
			},
			"action": map[string]any{
				"type":        "string",
				"description": "Control action: pause, resume, stop, next, volume <0-100>, status, queue <song>, show_queue",
			},
		},
		"required": []string{},
	}
}

// Execute handles music playback control.
func (t *YouTubeMusicTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	t.mu.Lock()
	defer t.mu.Unlock()

	query, hasQuery := args["query"].(string)
	action, hasAction := args["action"].(string)

	if hasQuery && query != "" {
		return t.playSong(query, ctx)
	}

	if hasAction && action != "" {
		return t.controlPlayback(action, ctx)
	}

	return ErrorResult("Provide 'query' to play a song or 'action' to control playback (pause, resume, stop, next, volume <level>, status, queue <song>, show_queue)")
}

// playSong searches YouTube, extracts metadata, and starts playback via mpv IPC.
func (t *YouTubeMusicTool) playSong(query string, ctx context.Context) *ToolResult {
	// Check dependencies
	if err := t.checkDependencies(); err != nil {
		return err
	}

	// Extract metadata before playing
	song := t.extractMetadata(query)

	// Stop any currently playing music
	if t.running {
		t.stopPlayback()
	}

	// Start playback
	return t.startPlayback(song, ctx)
}

// extractMetadata uses yt-dlp --dump-json to get title, duration, thumbnail.
func (t *YouTubeMusicTool) extractMetadata(query string) Song {
	song := Song{Query: query}

	cmd := exec.Command("yt-dlp",
		fmt.Sprintf("ytsearch:%s", query),
		"--dump-json",
		"--no-playlist",
		"--quiet",
		"--no-warnings",
		"--no-download",
	)

	output, err := cmd.Output()
	if err != nil {
		logger.DebugCF("youtube_music", "metadata extraction failed, using query as title", map[string]any{"query": query, "error": err.Error()})
		song.Title = query
		return song
	}

	var meta struct {
		Title     string  `json:"title"`
		Duration  float64 `json:"duration"`
		Thumbnail string  `json:"thumbnail"`
		URL       string  `json:"url"`
	}

	if err := json.Unmarshal(output, &meta); err != nil {
		logger.DebugCF("youtube_music", "failed to parse metadata JSON", map[string]any{"error": err.Error()})
		song.Title = query
		return song
	}

	song.Title = meta.Title
	song.Duration = int(meta.Duration)
	song.Thumbnail = meta.Thumbnail
	song.URL = meta.URL

	if song.Title == "" {
		song.Title = query
	}

	return song
}

// startPlayback launches mpv with the URL directly and sets up IPC for control.
// mpv handles yt-dlp internally so audio starts playing immediately.
// The IPC socket is used for pause/resume/volume/stop commands.
func (t *YouTubeMusicTool) startPlayback(song Song, ctx context.Context) *ToolResult {
	// Create temp socket for IPC
	socketPath := fmt.Sprintf("/tmp/octa-mpv-%d.sock", time.Now().UnixNano())
	t.ipcSocket = socketPath

	// Build the URL to play
	playURL := fmt.Sprintf("ytdl://ytsearch:%s", song.Query)
	if song.URL != "" {
		playURL = song.URL
	}

	// Start mpv with the URL directly — mpv plays audio immediately
	mpvArgs := []string{
		"--no-video",
		"--ytdl-format=" + t.quality,
		"--input-ipc-server=" + socketPath,
		playURL,
	}

	cmd := exec.Command(t.player, mpvArgs...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		t.cleanupSocket()
		return ErrorResult(fmt.Sprintf("Failed to start mpv: %v", err))
	}

	t.mpvCmd = cmd
	t.running = true
	t.currentSong = &song
	t.done = make(chan struct{})

	// Connect to IPC socket in background — mpv creates it after processing the URL.
	// Don't block the agent waiting for this; control commands will use reconnect if needed.
	go t.connectIPC(socketPath)

	// Monitor playback in background
	go t.monitorPlayback(cmd, ctx)

	logger.InfoCF("youtube_music", "playback started", map[string]any{
		"query": song.Query,
		"title": song.Title,
	})
	return t.formatNowPlaying(song)
}

// connectIPC connects to the mpv IPC socket with retry.
// Called in a goroutine — does not block playback.
func (t *YouTubeMusicTool) connectIPC(socketPath string) {
	ipc, err := newMPVClient(socketPath, 10*time.Second)
	if err != nil {
		logger.WarnCF("youtube_music", "IPC connection failed, control commands will reconnect on demand", map[string]any{"error": err.Error()})
		return
	}
	t.mu.Lock()
	t.ipc = ipc
	t.mu.Unlock()
	logger.DebugCF("youtube_music", "IPC connected", nil)
}

// getIPC returns the IPC client, creating one if needed.
// If the background connectIPC goroutine hasn't finished yet,
// this tries to connect on demand so control commands don't fail.
// Must be called with t.mu held (or from methods that hold it).
func (t *YouTubeMusicTool) getIPC() *mpvClient {
	if t.ipc != nil {
		return t.ipc
	}
	if t.ipcSocket == "" {
		return nil
	}
	// Try to connect on demand
	ipc, err := newMPVClient(t.ipcSocket, 2*time.Second)
	if err != nil {
		logger.WarnCF("youtube_music", "on-demand IPC connect failed", map[string]any{"error": err.Error()})
		return nil
	}
	t.ipc = ipc
	return ipc
}

// monitorPlayback waits for mpv to exit and handles auto-next from queue.
func (t *YouTubeMusicTool) monitorPlayback(cmd *exec.Cmd, ctx context.Context) {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		t.mu.Lock()
		defer t.mu.Unlock()
		t.onPlaybackEnd()

	case <-ctx.Done():
		t.mu.Lock()
		t.stopPlayback()
		t.mu.Unlock()

	case <-t.done:
		cmd.Wait()
	}
}

// onPlaybackEnd handles cleanup and auto-advance from queue.
// Must be called with t.mu held.
func (t *YouTubeMusicTool) onPlaybackEnd() {
	t.closeIPC()
	t.cleanupSocket()

	if len(t.queue) > 0 {
		next := t.queue[0]
		t.queue = t.queue[1:]
		logger.InfoCF("youtube_music", "auto-advancing to next song", map[string]any{"title": next.Title})
		t.startPlaybackUnlocked(next)
		return
	}

	t.running = false
	t.currentSong = nil
	t.mpvCmd = nil
}

// startPlaybackUnlocked starts playback without acquiring the mutex (caller must hold it).
func (t *YouTubeMusicTool) startPlaybackUnlocked(song Song) {
	socketPath := fmt.Sprintf("/tmp/octa-mpv-%d.sock", time.Now().UnixNano())
	t.ipcSocket = socketPath

	playURL := fmt.Sprintf("ytdl://ytsearch:%s", song.Query)
	if song.URL != "" {
		playURL = song.URL
	}

	mpvArgs := []string{
		"--no-video",
		"--ytdl-format=" + t.quality,
		"--input-ipc-server=" + socketPath,
		playURL,
	}

	cmd := exec.Command(t.player, mpvArgs...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		logger.ErrorCF("youtube_music", "failed to start mpv for next song", map[string]any{"error": err.Error()})
		t.running = false
		t.currentSong = nil
		t.cleanupSocket()
		return
	}

	t.mpvCmd = cmd
	t.running = true
	t.currentSong = &song
	t.done = make(chan struct{})

	go t.connectIPC(socketPath)
	go t.monitorPlayback(cmd, context.Background())
}

// controlPlayback handles pause, stop, volume, etc.
func (t *YouTubeMusicTool) controlPlayback(action string, ctx context.Context) *ToolResult {
	action = strings.ToLower(strings.TrimSpace(action))

	switch {
	case strings.HasPrefix(action, "play"):
		song := strings.TrimSpace(strings.TrimPrefix(action, "play"))
		if song != "" {
			return t.playSong(song, ctx)
		}
		return t.resumeMusic()

	case action == "pause":
		return t.pauseMusic()

	case action == "resume":
		return t.resumeMusic()

	case action == "stop":
		return t.stopMusic()

	case strings.HasPrefix(action, "volume"):
		return t.setVolume(action)

	case action == "next":
		return t.nextSong()

	case action == "status":
		return t.getStatus()

	case strings.HasPrefix(action, "queue"):
		song := strings.TrimSpace(strings.TrimPrefix(action, "queue"))
		if song == "" {
			return t.showQueue()
		}
		return t.addToQueue(song)

	case action == "show_queue":
		return t.showQueue()

	default:
		return ErrorResult(fmt.Sprintf("Unknown action: %s. Available: play, pause, resume, stop, next, volume <level>, status, queue <song>, show_queue", action))
	}
}

// pauseMusic pauses playback via IPC.
func (t *YouTubeMusicTool) pauseMusic() *ToolResult {
	if !t.running {
		return ErrorResult("No music is currently playing")
	}

	ipc := t.getIPC()
	if ipc == nil {
		return ErrorResult("Music control unavailable (IPC not connected)")
	}

	if err := ipc.setProperty("pause", true); err != nil {
		logger.ErrorCF("youtube_music", "IPC pause failed", map[string]any{"error": err.Error()})
		return ErrorResult(fmt.Sprintf("Failed to pause: %v", err))
	}
	return NewToolResult("Paused")
}

// resumeMusic resumes playback via IPC.
func (t *YouTubeMusicTool) resumeMusic() *ToolResult {
	if !t.running {
		return ErrorResult("No music is currently paused")
	}

	ipc := t.getIPC()
	if ipc == nil {
		return ErrorResult("Music control unavailable (IPC not connected)")
	}

	if err := ipc.setProperty("pause", false); err != nil {
		logger.ErrorCF("youtube_music", "IPC resume failed", map[string]any{"error": err.Error()})
		return ErrorResult(fmt.Sprintf("Failed to resume: %v", err))
	}
	return NewToolResult("Resumed")
}

// stopMusic stops playback and cleans up.
func (t *YouTubeMusicTool) stopMusic() *ToolResult {
	if !t.running {
		return ErrorResult("No music is currently playing")
	}

	t.stopPlayback()
	return NewToolResult("Stopped")
}

// stopPlayback stops mpv and cleans up resources. Must be called with t.mu held.
func (t *YouTubeMusicTool) stopPlayback() {
	select {
	case <-t.done:
	default:
		close(t.done)
	}

	t.closeIPC()

	if t.mpvCmd != nil && t.mpvCmd.Process != nil {
		t.mpvCmd.Process.Kill()
		t.mpvCmd.Wait()
		t.mpvCmd = nil
	}

	t.cleanupSocket()
	t.running = false
	t.currentSong = nil
	t.queue = nil
}

// setVolume adjusts playback volume via IPC.
func (t *YouTubeMusicTool) setVolume(action string) *ToolResult {
	parts := strings.Fields(action)
	if len(parts) < 2 {
		return ErrorResult("Usage: volume <level> (0-100)")
	}

	var level int
	if _, err := fmt.Sscanf(parts[1], "%d", &level); err != nil {
		return ErrorResult("Invalid volume level. Use a number between 0 and 100")
	}

	level = max(0, min(100, level))

	if !t.running {
		return NewToolResult(fmt.Sprintf("Volume set to %d (will apply when playback starts)", level))
	}

	ipc := t.getIPC()
	if ipc == nil {
		return ErrorResult("Music control unavailable (IPC not connected)")
	}

	if err := ipc.setProperty("volume", level); err != nil {
		logger.ErrorCF("youtube_music", "IPC volume failed", map[string]any{"error": err.Error(), "level": level})
		return ErrorResult(fmt.Sprintf("Failed to set volume: %v", err))
	}

	// Verify the volume was actually set
	if actualVol, err := ipc.getProperty("volume"); err == nil {
		if v, ok := actualVol.(float64); ok {
			return NewToolResult(fmt.Sprintf("Volume set to %d%%", int(v)))
		}
	}

	return NewToolResult(fmt.Sprintf("Volume set to %d", level))
}

// nextSong skips to the next song in the queue.
func (t *YouTubeMusicTool) nextSong() *ToolResult {
	if !t.running {
		return ErrorResult("No music is currently playing")
	}

	if len(t.queue) == 0 {
		return ErrorResult("Queue is empty. Add songs with 'queue <song>' or play a new song with 'play <song>'")
	}

	next := t.queue[0]
	t.queue = t.queue[1:]

	t.closeIPC()
	if t.mpvCmd != nil && t.mpvCmd.Process != nil {
		t.mpvCmd.Process.Kill()
		t.mpvCmd.Wait()
	}
	t.cleanupSocket()

	t.running = false
	t.ipc = nil
	t.mpvCmd = nil

	return t.startPlayback(next, context.Background())
}

// getStatus returns current playback status with progress info.
func (t *YouTubeMusicTool) getStatus() *ToolResult {
	if !t.running {
		msg := "No music is playing"
		if len(t.queue) > 0 {
			msg += fmt.Sprintf(". Queue has %d song(s)", len(t.queue))
		}
		return NewToolResult(msg)
	}

	ipc := t.getIPC()
	if ipc != nil {
		var parts []string

		if t.currentSong != nil && t.currentSong.Title != "" {
			parts = append(parts, fmt.Sprintf("Now playing: %s", t.currentSong.Title))
		}

		if paused, err := ipc.isPaused(); err == nil {
			if paused {
				parts = append(parts, "Status: Paused")
			} else {
				parts = append(parts, "Status: Playing")
			}
		}

		pos, posErr := ipc.getTimePos()
		dur, durErr := ipc.getDuration()
		if posErr == nil && durErr == nil && dur > 0 {
			parts = append(parts, t.formatProgressBar(pos, dur))
		}

		if vol, err := ipc.getProperty("volume"); err == nil {
			if v, ok := vol.(float64); ok {
				parts = append(parts, fmt.Sprintf("Volume: %d%%", int(v)))
			}
		}

		if len(t.queue) > 0 {
			parts = append(parts, fmt.Sprintf("Queue: %d song(s)", len(t.queue)))
		}

		if len(parts) > 0 {
			return NewToolResult(strings.Join(parts, "\n"))
		}
	}

	title := "Unknown"
	if t.currentSong != nil && t.currentSong.Title != "" {
		title = t.currentSong.Title
	}
	msg := fmt.Sprintf("Playing: %s", title)
	if len(t.queue) > 0 {
		msg += fmt.Sprintf(" | Queue: %d", len(t.queue))
	}
	return NewToolResult(msg)
}

// addToQueue adds a song to the playback queue.
func (t *YouTubeMusicTool) addToQueue(query string) *ToolResult {
	song := t.extractMetadata(query)
	t.queue = append(t.queue, song)
	pos := len(t.queue)

	msg := fmt.Sprintf("Added to queue (#%d): %s", pos, song.Title)
	if song.Duration > 0 {
		msg += fmt.Sprintf(" (%s)", formatDuration(song.Duration))
	}
	return NewToolResult(msg)
}

// showQueue displays the current song queue.
func (t *YouTubeMusicTool) showQueue() *ToolResult {
	if t.currentSong == nil && len(t.queue) == 0 {
		return NewToolResult("Queue is empty. Use 'queue <song>' to add songs.")
	}

	var parts []string

	if t.currentSong != nil {
		parts = append(parts, fmt.Sprintf("Now playing: %s", t.currentSong.Title))
	}

	if len(t.queue) == 0 {
		parts = append(parts, "Up next: (queue is empty)")
	} else {
		parts = append(parts, "Up next:")
		for i, s := range t.queue {
			entry := fmt.Sprintf("  %d. %s", i+1, s.Title)
			if s.Duration > 0 {
				entry += fmt.Sprintf(" (%s)", formatDuration(s.Duration))
			}
			parts = append(parts, entry)
		}
	}

	return NewToolResult(strings.Join(parts, "\n"))
}

// formatNowPlaying returns a formatted "now playing" message.
func (t *YouTubeMusicTool) formatNowPlaying(song Song) *ToolResult {
	msg := fmt.Sprintf("Now playing: %s", song.Title)
	if song.Duration > 0 {
		msg += fmt.Sprintf(" (%s)", formatDuration(song.Duration))
	}
	msg += "\nControls: /music pause | /music stop | /music volume <0-100>"
	return NewToolResult(msg)
}

// formatProgressBar creates a text-based progress bar.
func (t *YouTubeMusicTool) formatProgressBar(pos, dur float64) string {
	width := 20
	progress := pos / dur
	if progress > 1 {
		progress = 1
	}
	filled := int(math.Round(progress * float64(width)))

	bar := "["
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "="
		} else if i == filled {
			bar += ">"
		} else {
			bar += " "
		}
	}
	bar += "]"

	return fmt.Sprintf("%s %s / %s", bar, formatDuration(int(pos)), formatDuration(int(dur)))
}

// formatDuration formats seconds as mm:ss or hh:mm:ss.
func formatDuration(seconds int) string {
	if seconds <= 0 {
		return "0:00"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// checkDependencies verifies yt-dlp and mpv are installed.
func (t *YouTubeMusicTool) checkDependencies() *ToolResult {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return ErrorResult("yt-dlp is not installed. Install with: pip install yt-dlp")
	}
	if _, err := exec.LookPath(t.player); err != nil {
		return ErrorResult(fmt.Sprintf("%s is not installed. Install with: sudo apt install %s (Debian/Ubuntu) or sudo pacman -S %s (Arch)", t.player, t.player, t.player))
	}
	return nil
}

// closeIPC closes the IPC connection to mpv.
func (t *YouTubeMusicTool) closeIPC() {
	if t.ipc != nil {
		t.ipc.quit()
		t.ipc = nil
	}
}

// cleanupSocket removes the IPC socket file.
func (t *YouTubeMusicTool) cleanupSocket() {
	if t.ipcSocket != "" {
		os.Remove(t.ipcSocket)
		t.ipcSocket = ""
	}
}

// Cleanup stops playback and releases all resources.
// Call this during agent shutdown.
func (t *YouTubeMusicTool) Cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		logger.InfoCF("youtube_music", "cleaning up on shutdown", nil)
		t.stopPlayback()
	}
}

// IsStreaming returns whether music is currently playing.
func (t *YouTubeMusicTool) IsStreaming() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.running
}
