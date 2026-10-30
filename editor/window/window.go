// Package window runs one editor window process: it connects to the
// orchestrator, subscribes to its target document and runs the Ebitengine
// window for its view (the project browser, a Map, Text, Workflow, Watch,
// Clock, Audio or Pixel editor window).
package window

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/audioview"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/clockview"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/editor/views/paletteview"
	"github.com/bradhannah/berad-engine/editor/views/pixelview"
	"github.com/bradhannah/berad-engine/editor/views/textview"
	"github.com/bradhannah/berad-engine/editor/views/watchview"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const (
	connectTimeout = 10 * time.Second
	windowWidth    = 1200
	windowHeight   = 800
)

// ErrUnknownView reports a view this program does not provide.
var ErrUnknownView = errors.New("unknown window view")

// Args identifies the window to run. Shell runs it as a shell, which shows
// every window the orchestrator opens after it as one of its panes
// (increment 10).
type Args struct {
	Socket string
	View   string
	Target ids.ULID
	Window ids.ULID
	Build  string
	Shell  bool
}

// Run connects, opens the window and blocks until it closes. It must be called
// from the main goroutine, as Ebitengine requires.
func Run(args Args, registry *plugin.Registry) error {
	if args.Shell {
		return runShell(args, registry)
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	conn, err := connect(ctx, args, registry)
	if err != nil {
		return err
	}
	theme, err := ui.NewTheme()
	if err != nil {
		return errors.Join(err, conn.Close())
	}
	defer theme.Close()
	game := newHost(conn, newView(args, conn.Window(), theme), args.View, theme, newOwnWindow())
	game.script, err = loadScript(args.View)
	if err != nil {
		return errors.Join(err, conn.Close())
	}
	ebiten.SetWindowTitle(game.view.Title(conn))
	ebiten.SetWindowSize(windowSize(args.View))
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// The Clock window stays on top of the others until its author turns
	// that off.
	game.frame.OnTop.On = args.View == clockview.ViewName
	game.floating = game.frame.OnTop.On
	ebiten.SetWindowFloating(game.floating)
	// The close button asks the view first, so unsaved changes are not lost.
	ebiten.SetWindowClosingHandled(true)
	// A window driven by an input script needs no keyboard, so it opens
	// without taking focus from whatever the person at the computer is using.
	// So does every other window opened while scripts drive the editor, as
	// in the real-window tests, even one left without a script, and the
	// Clock window, which opens by itself when a preview starts.
	var options ebiten.RunGameOptions
	options.InitUnfocused = game.script != nil || scripted() || args.View == clockview.ViewName
	stats := newFrameStats(args.View)
	runErr := ebiten.RunGameWithOptions(timed(game, stats), &options)
	if errors.Is(runErr, ebiten.Termination) {
		runErr = nil
	}
	stats.addLatencies(game.frame.Latencies())

	return errors.Join(runErr, game.sounds.Close(), conn.Close(), stats.write())
}

// connect connects a window to the orchestrator and loads its target.
func connect(ctx context.Context, args Args, registry *plugin.Registry) (*client.Client, error) {
	switch args.View {
	case mapview.ViewName, browser.ViewName, textview.ViewName, workflowview.ViewName, watchview.ViewName,
		clockview.ViewName, audioview.ViewName, pixelview.ViewName, paletteview.ViewName:
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownView, args.View)
	}
	conn, err := client.Dial(ctx, args.Socket, protocol.Hello{
		Protocol: protocol.Version, Build: args.Build, Window: args.Window, View: args.View,
		Target: args.Target, UndoGroup: undoGroup(args), Hosts: args.Shell,
	}, registry)
	if err != nil {
		return nil, fmt.Errorf("connect window: %w", err)
	}
	if err := conn.Subscribe(ctx, args.Target); err != nil {
		return nil, errors.Join(fmt.Errorf("open document: %w", err), conn.Close())
	}

	return conn, nil
}

// undoGroup is the undo list a window shares: views of one map, of one
// workflow, or of one sprite share one (D3); the Audio window, of which there
// is one, has its own.
func undoGroup(args Args) string {
	switch args.View {
	case mapview.ViewName:
		return "map:" + args.Target.String()
	case workflowview.ViewName:
		return "workflow:" + args.Target.String()
	case audioview.ViewName:
		return audioview.UndoGroup
	case pixelview.ViewName:
		return "sprite:" + args.Target.String()
	case paletteview.ViewName:
		return paletteview.UndoGroup(args.Target)
	}

	return ""
}

// windowSize is the size a window opens at: small for the Clock window.
func windowSize(view string) (int, int) {
	if view == clockview.ViewName {
		return clockview.Width, clockview.Height
	}

	return windowWidth, windowHeight
}

//nolint:ireturn // Each window runs one of several views.
func newView(args Args, window ids.ULID, theme *ui.Theme) frame.View {
	switch args.View {
	case browser.ViewName:
		return browser.New(window, theme)
	case audioview.ViewName:
		return audioview.New(window, theme)
	case clockview.ViewName:
		return clockview.New(theme)
	case textview.ViewName:
		return textview.New(window, args.Target, theme)
	case workflowview.ViewName:
		return workflowview.New(window, args.Target, theme)
	case watchview.ViewName:
		return watchview.New(window, args.Target, theme)
	case pixelview.ViewName:
		return pixelview.New(window, args.Target, theme)
	case paletteview.ViewName:
		return paletteview.New(window, args.Target, theme)
	default:
		return mapview.New(window, args.Target, theme)
	}
}

// saveScreenshot writes the window's pixels to a PNG, for automated visual
// checks. It is enabled by the BERAD_SCREENSHOT environment variable.
func saveScreenshot(screen *ebiten.Image, path string) error {
	bounds := screen.Bounds()
	pixels := image.NewRGBA(bounds)
	screen.ReadPixels(pixels.Pix)
	file, err := os.Create(path) //nolint:gosec // The path comes from the developer's own environment.
	if err != nil {
		return fmt.Errorf("create screenshot: %w", err)
	}
	encodeErr := png.Encode(file, pixels)
	closeErr := file.Close()
	if err := errors.Join(encodeErr, closeErr); err != nil {
		return fmt.Errorf("write screenshot: %w", err)
	}

	return nil
}
