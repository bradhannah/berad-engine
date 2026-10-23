// Package window runs one editor window process: it connects to the
// orchestrator, subscribes to its target document and runs the Ebitengine
// window for its view.
package window

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const (
	connectTimeout = 10 * time.Second
	windowWidth    = 1200
	windowHeight   = 800
	textSize       = 13
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
	if args.View != "map" {
		return fmt.Errorf("%w: %q", ErrUnknownView, args.View)
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	conn, err := client.Dial(ctx, args.Socket, protocol.Hello{
		Protocol:  protocol.Version,
		Build:     args.Build,
		Window:    args.Window,
		View:      args.View,
		Target:    args.Target,
		UndoGroup: "map:" + args.Target.String(),
	}, registry)
	if err != nil {
		return fmt.Errorf("connect window: %w", err)
	}
	if err := conn.Subscribe(ctx, args.Target); err != nil {
		return errors.Join(fmt.Errorf("open document: %w", err), conn.Close())
	}
	face, err := newFace()
	if err != nil {
		return errors.Join(err, conn.Close())
	}
	game := newMapGame(conn, args.Target, face)
	game.script, err = loadScript()
	if err != nil {
		return errors.Join(err, conn.Close())
	}
	ebiten.SetWindowTitle(game.title())
	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	runErr := ebiten.RunGame(game)
	if errors.Is(runErr, ebiten.Termination) {
		runErr = nil
	}

	return errors.Join(runErr, conn.Close())
}

func newFace() (*text.GoTextFace, error) {
	source, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, fmt.Errorf("load font: %w", err)
	}
	var face text.GoTextFace
	face.Source = source
	face.Size = textSize

	return &face, nil
}
