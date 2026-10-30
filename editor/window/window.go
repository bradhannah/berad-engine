// Package window runs the editor window: one Ebitengine window whose window
// bar has a tab for each view open, each view running in a pane with its own
// connection to the orchestrator (the project browser, a Map, Text,
// Workflow, Watch, Audio, Pixel editor or Palette Editor, and the pinned
// clock).
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

// Args identifies a window to connect: the editor window's first pane, or a
// pane the orchestrator asked it to show, which connects as Window.
type Args struct {
	// Connect opens the pane's connection to the orchestrator.
	Connect client.Dialer
	View    string
	Target  ids.ULID
	Window  ids.ULID
}

// Run opens the editor window with its first pane, normally the project
// browser, and blocks until it closes. It must be called from the main
// goroutine, as Ebitengine requires.
func Run(args Args, registry *plugin.Registry) error {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	conn, err := connect(ctx, args, registry, true)
	if err != nil {
		return err
	}
	theme, err := ui.NewTheme()
	if err != nil {
		return errors.Join(err, conn.Close())
	}
	defer theme.Close()
	shown := newShell(args, registry, theme)
	project, err := shown.addPane(conn, args)
	if err != nil {
		return errors.Join(err, conn.Close())
	}
	project.host.closeQuits = true
	project.host.hosts = shown.host
	// The pinned clock is a Clock window, always running.
	project.host.Open(clockview.ViewName, args.Target, false)
	ebiten.SetWindowTitle(project.host.view.Title(conn))
	ebiten.SetWindowSize(windowWidth, windowHeight+rowHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// Closing the editor window quits the editor, asking first.
	ebiten.SetWindowClosingHandled(true)
	// While input scripts drive the editor, as in the real-window tests, the
	// window opens without taking focus from whatever the person at the
	// computer is using.
	var options ebiten.RunGameOptions
	options.InitUnfocused = scripted()
	runErr := ebiten.RunGameWithOptions(timed(shown, shown.stats), &options)
	if errors.Is(runErr, ebiten.Termination) {
		runErr = nil
	}

	return errors.Join(runErr, shown.closeAll(), shown.stats.write())
}

// connect connects a window to the orchestrator and loads its target.
// hosts is set for the editor window's own connection, which the
// orchestrator asks to show the windows it opens.
func connect(ctx context.Context, args Args, registry *plugin.Registry, hosts bool) (*client.Client, error) {
	switch args.View {
	case mapview.ViewName, browser.ViewName, textview.ViewName, workflowview.ViewName, watchview.ViewName,
		clockview.ViewName, audioview.ViewName, pixelview.ViewName, paletteview.ViewName:
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownView, args.View)
	}
	conn, err := client.Dial(ctx, args.Connect, protocol.Hello{
		Protocol: protocol.Version, Window: args.Window, View: args.View,
		Target: args.Target, UndoGroup: undoGroup(args), Hosts: hosts,
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
