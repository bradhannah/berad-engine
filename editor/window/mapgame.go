package window

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const (
	requestTimeout  = 5 * time.Second
	statusDuration  = 4 * time.Second
	latencySamples  = 120
	resultQueue     = 16
	maxUpdates      = 1024
	screenshotEnv   = "BERAD_SCREENSHOT"
	screenshotFrame = 90
)

// mapGame is the Map window: Ebitengine input and drawing around mapview.
type mapGame struct {
	conn        *client.Client
	target      ids.ULID
	state       *mapview.State
	face        *text.GoTextFace
	layout      mapview.Layout
	latencies   []time.Duration
	status      string
	statusUntil time.Time
	notice      string
	results     chan string
	screenshot  string
	frames      int
	script      *inputScript
}

func newMapGame(conn *client.Client, target ids.ULID, face *text.GoTextFace) *mapGame {
	return &mapGame{
		conn:        conn,
		target:      target,
		state:       mapview.New(conn.Window().String()),
		face:        face,
		layout:      mapview.Layout{Width: windowWidth, Height: windowHeight},
		latencies:   nil,
		status:      "",
		statusUntil: time.Time{},
		notice:      "",
		results:     make(chan string, resultQueue),
		screenshot:  os.Getenv(screenshotEnv),
		frames:      0,
		script:      nil,
	}
}

func (g *mapGame) title() string {
	name := "Map"
	g.conn.View(g.target, func(document client.Document) { name = "Map: " + document.Name })

	return name + "  (view " + g.conn.Window().Short(shortID) + ")"
}

// Update runs once per tick: apply what the orchestrator sent, then turn input
// into commands.
func (g *mapGame) Update() error {
	if g.drainUpdates() {
		return ebiten.Termination
	}
	select {
	case message := <-g.results:
		g.setStatus(message)
	default:
	}
	input := readInput()
	if g.script != nil {
		input = g.script.next(g.state, g.layout)
	}
	actions := g.state.Update(input, g.layout)
	if actions.SimulateCrash {
		panic("simulated window crash (debug key)")
	}
	if actions.Quit {
		return ebiten.Termination
	}
	g.sendPaints(actions.Paints)
	if actions.Undo {
		g.async("Undone", g.conn.Undo)
	}
	if actions.Redo {
		g.async("Redone", g.conn.Redo)
	}
	if actions.NewView {
		g.async("Opened a new view", func(ctx context.Context) (protocol.Result, error) {
			return g.conn.Open(ctx, "map", g.target)
		})
	}

	return nil
}

// drainUpdates applies queued updates and reports whether the window must close.
func (g *mapGame) drainUpdates() bool {
	for range maxUpdates {
		select {
		case update, open := <-g.conn.Updates():
			if !open || update.Shutdown != nil {
				return true
			}
			g.onUpdate(update)
		default:
			return false
		}
	}

	return false
}

func (g *mapGame) onUpdate(update client.Update) {
	if update.Latency > 0 {
		g.latencies = append(g.latencies, update.Latency)
		if len(g.latencies) > latencySamples {
			g.latencies = g.latencies[len(g.latencies)-latencySamples:]
		}
	}
	if update.Result != nil && !update.Result.OK {
		g.setStatus(update.Result.Reason)
	}
	if update.Session != nil && update.Session.Topic == protocol.TopicNotice {
		var notice protocol.Notice
		if err := json.Unmarshal(update.Session.Value, &notice); err == nil {
			g.notice = notice.Text
		}
	}
}

func (g *mapGame) sendPaints(paints []mapview.Paint) {
	for _, paint := range paints {
		cells := []tilefloor.Cell{{Col: paint.Col, Row: paint.Row, Tile: paint.Tile}}
		args, err := json.Marshal(tilefloor.PaintArgs{Cells: cells})
		if err != nil {
			g.setStatus(err.Error())

			continue
		}
		command := protocol.Command{Document: g.target, Edit: tilefloor.PaintEdit, Gesture: paint.Gesture, Args: args}
		if _, err := g.conn.Send(command); err != nil {
			g.setStatus(err.Error())
		}
	}
}

// async runs a request off the frame loop and reports its outcome.
func (g *mapGame) async(success string, request func(context.Context) (protocol.Result, error)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		result, err := request(ctx)
		message := success
		if err != nil {
			message = err.Error()
			if result.Reason != "" {
				message = result.Reason
			}
		}
		select {
		case g.results <- message:
		default:
		}
	}()
}

func (g *mapGame) setStatus(message string) {
	g.status = message
	g.statusUntil = time.Now().Add(statusDuration)
}

// Layout uses the window's real size, one screen pixel per pixel.
func (g *mapGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	g.layout = mapview.Layout{Width: outsideWidth, Height: outsideHeight}

	return outsideWidth, outsideHeight
}

func (g *mapGame) medianLatency() time.Duration {
	if len(g.latencies) == 0 {
		return 0
	}
	sorted := slices.Clone(g.latencies)
	slices.Sort(sorted)

	return sorted[len(sorted)/2]
}

func readInput() mapview.Input {
	cursorX, cursorY := ebiten.CursorPosition()
	modifier := ebiten.IsKeyPressed(ebiten.KeyMeta) || ebiten.IsKeyPressed(ebiten.KeyControl)
	shift := ebiten.IsKeyPressed(ebiten.KeyShift)
	zPressed := inpututil.IsKeyJustPressed(ebiten.KeyZ)

	return mapview.Input{
		CursorX:       cursorX,
		CursorY:       cursorY,
		Down:          ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft),
		Pressed:       inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft),
		Released:      inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft),
		Undo:          modifier && !shift && zPressed,
		Redo:          modifier && shift && zPressed,
		NewView:       modifier && inpututil.IsKeyJustPressed(ebiten.KeyN),
		Quit:          modifier && inpututil.IsKeyJustPressed(ebiten.KeyW),
		SimulateCrash: modifier && shift && inpututil.IsKeyJustPressed(ebiten.KeyK),
		TileKey:       tileKey(),
	}
}

func tileKey() int {
	keys := []ebiten.Key{
		ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3,
		ebiten.KeyDigit4, ebiten.KeyDigit5, ebiten.KeyDigit6,
	}
	for index, key := range keys {
		if inpututil.IsKeyJustPressed(key) {
			return index + 1
		}
	}

	return 0
}

func formatLatency(latency time.Duration) string {
	if latency == 0 {
		return "-"
	}

	return fmt.Sprintf("%.0f us", float64(latency)/float64(time.Microsecond))
}
