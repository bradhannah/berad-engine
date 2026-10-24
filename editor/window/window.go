// Package window runs one editor window process: it connects to the
// orchestrator, subscribes to its target document and runs the Ebitengine
// window for its view (the project browser, a Map, Text, Workflow, Watch or
// Clock window).
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
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/clockview"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
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

// Args identifies the window to run.
type Args struct {
	Socket string
	View   string
	Target ids.ULID
	Window ids.ULID
	Build  string
}

// Run connects, opens the window and blocks until it closes. It must be called
// from the main goroutine, as Ebitengine requires.
func Run(args Args, registry *plugin.Registry) error {
	switch args.View {
	case mapview.ViewName, browser.ViewName, textview.ViewName, workflowview.ViewName, watchview.ViewName,
		clockview.ViewName:
	default:
		return fmt.Errorf("%w: %q", ErrUnknownView, args.View)
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	// Views of one map, or of one workflow, share an undo list (D3).
	group := ""
	switch args.View {
	case mapview.ViewName:
		group = "map:" + args.Target.String()
	case workflowview.ViewName:
		group = "workflow:" + args.Target.String()
	}
	conn, err := client.Dial(ctx, args.Socket, protocol.Hello{
		Protocol: protocol.Version, Build: args.Build, Window: args.Window, View: args.View,
		Target: args.Target, UndoGroup: group,
	}, registry)
	if err != nil {
		return fmt.Errorf("connect window: %w", err)
	}
	if err := conn.Subscribe(ctx, args.Target); err != nil {
		return errors.Join(fmt.Errorf("open document: %w", err), conn.Close())
	}
	theme, err := ui.NewTheme()
	if err != nil {
		return errors.Join(err, conn.Close())
	}
	defer theme.Close()
	game := newHost(conn, newView(args, conn.Window(), theme), args.View, theme)
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
	// The close button asks the view first, so unsaved changes are not lost.
	ebiten.SetWindowClosingHandled(true)
	// A window driven by an input script needs no keyboard, so it opens
	// without taking focus from whatever the person at the computer is using.
	// So does every other window opened while scripts drive the editor, as
	// in the real-window tests, even one left without a script, and the
	// Clock window, which opens by itself when a preview starts.
	var options ebiten.RunGameOptions
	options.InitUnfocused = game.script != nil || scripted() || args.View == clockview.ViewName
	runErr := ebiten.RunGameWithOptions(game, &options)
	if errors.Is(runErr, ebiten.Termination) {
		runErr = nil
	}

	return errors.Join(runErr, conn.Close())
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
	case clockview.ViewName:
		return clockview.New(theme)
	case textview.ViewName:
		return textview.New(window, args.Target, theme)
	case workflowview.ViewName:
		return workflowview.New(window, args.Target, theme)
	case watchview.ViewName:
		return watchview.New(window, args.Target, theme)
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
