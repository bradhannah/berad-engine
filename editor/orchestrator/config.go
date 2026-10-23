package orchestrator

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/bradhannah/berad-engine/editor/plugin"
)

const (
	defaultUndoLimit        = 100
	defaultFlushInterval    = 2 * time.Second
	defaultSyncInterval     = 200 * time.Millisecond
	defaultHandshakeTimeout = 5 * time.Second
	defaultShutdownTimeout  = 5 * time.Second
	defaultOutboundQueue    = 4096
	defaultRecentMessages   = 64
	socketHashChars         = 12
)

// ErrConfig reports an incomplete configuration.
var ErrConfig = errors.New("invalid orchestrator configuration")

// Config configures an orchestrator. Zero values take the documented defaults.
type Config struct {
	// ProjectDir is the project folder. Required.
	ProjectDir string
	// Registry is the explicit plug-in list. Required.
	Registry *plugin.Registry
	// Build identifies the program build; windows of another build are refused.
	Build string
	// SocketPath is where windows connect. Defaults to a path in the temporary
	// folder derived from the project folder.
	SocketPath string
	// UndoLimit is the number of undo steps kept per undo group. Default 100.
	UndoLimit int
	// FlushInterval is how often changed documents are saved to their files.
	FlushInterval time.Duration
	// SyncInterval is how often the journal is synced to disk.
	SyncInterval time.Duration
	// HandshakeTimeout bounds how long a new connection may take to say hello.
	HandshakeTimeout time.Duration
	// ShutdownTimeout bounds how long windows get to close on shutdown.
	ShutdownTimeout time.Duration
	// OutboundQueue is the per-window queue length; a window that falls this
	// far behind is disconnected.
	OutboundQueue int
	// RecentMessages is how many recent messages per window are kept for
	// failure reports.
	RecentMessages int
	// Clock returns the current time. Defaults to time.Now.
	Clock func() time.Time
	// Random supplies ID randomness. Defaults to crypto/rand.
	Random io.Reader
	// Logger receives diagnostics. Defaults to text on standard error.
	Logger *slog.Logger
	// Tap, when set, receives every message as a JSON line (the wire tap).
	Tap io.Writer
	// Launcher starts window processes. Without one, Open requests fail.
	Launcher Launcher
	// QuitWhenLastWindowCloses stops the orchestrator once every window it
	// launched has closed, so no headless orchestrator is left running.
	QuitWhenLastWindowCloses bool
}

func (c Config) withDefaults() (Config, error) {
	if c.ProjectDir == "" || c.Registry == nil {
		return c, fmt.Errorf("%w: ProjectDir and Registry are required", ErrConfig)
	}
	absolute, err := filepath.Abs(c.ProjectDir)
	if err != nil {
		return c, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	c.ProjectDir = absolute
	if c.SocketPath == "" {
		sum := sha256.Sum256([]byte(absolute))
		c.SocketPath = filepath.Join(os.TempDir(), "berad-"+hex.EncodeToString(sum[:])[:socketHashChars]+".sock")
	}
	c.UndoLimit = orDefault(c.UndoLimit, defaultUndoLimit)
	c.FlushInterval = orDefault(c.FlushInterval, defaultFlushInterval)
	c.SyncInterval = orDefault(c.SyncInterval, defaultSyncInterval)
	c.HandshakeTimeout = orDefault(c.HandshakeTimeout, defaultHandshakeTimeout)
	c.ShutdownTimeout = orDefault(c.ShutdownTimeout, defaultShutdownTimeout)
	c.OutboundQueue = orDefault(c.OutboundQueue, defaultOutboundQueue)
	c.RecentMessages = orDefault(c.RecentMessages, defaultRecentMessages)
	if c.Clock == nil {
		c.Clock = time.Now
	}
	if c.Random == nil {
		c.Random = rand.Reader
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	return c, nil
}

//nolint:ireturn // T is a concrete numeric type chosen by the caller.
func orDefault[T int | time.Duration](value, fallback T) T {
	if value <= 0 {
		return fallback
	}

	return value
}
