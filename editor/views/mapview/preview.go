package mapview

import (
	"fmt"
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/play"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

const (
	// streamTicks limits how often the preview's state goes to the zone's
	// other views: every second tick, 30 times a second (D7).
	streamTicks = 2
	// followMargin is how many cells the view keeps between a walking
	// player and its edge.
	followMargin = 3
	// The player is a white token with a blue ring and a blue dot towards
	// where they face.
	playerRadius = 0.36
	playerRing   = 2.5
	faceOffset   = 0.5
	faceRadius   = 0.3
	cellCentre   = 0.5
	// startAlpha is how faintly the player shows on the cell under the
	// cursor while the author chooses where the player starts.
	startAlpha = 0x80
	// previewLines is the height of the preview's part of the inspector.
	previewLines = 4
)

// previewRun is the preview this window runs (D15). The run is simulated in
// this process; its state goes to the zone's other views through the
// orchestrator.
type previewRun struct {
	id   ids.ULID
	run  *play.Run
	held play.Direction
	sent *protocol.PreviewEntity
}

// remotePreview is a preview another window runs in this zone.
type remotePreview struct {
	id     ids.ULID
	tick   uint64
	player *protocol.PreviewEntity
}

// docsZone gives a run the window's copies of the zone's floors.
type docsZone struct {
	floors []ids.ULID
	docs   frame.Docs
}

func (z docsZone) Floors() []ids.ULID { return z.floors }

func (z docsZone) Floor(id ids.ULID, fn func(*floor.Floor)) bool {
	found := false
	z.docs.View(id, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			found = true
			fn(floorDoc.Floor)
		}
	})

	return found
}

// arrows maps the arrow keys to the directions they walk.
func arrows() map[ui.Key]play.Direction {
	return map[ui.Key]play.Direction{
		ui.KeyUp: play.North, ui.KeyRight: play.East, ui.KeyDown: play.South, ui.KeyLeft: play.West,
	}
}

// togglePreview starts choosing where the player starts, or stops the
// preview.
func (s *State) togglePreview(ctx *frame.Context) {
	switch {
	case s.starting:
	case s.preview != nil:
		s.stopPreview(ctx, "Preview stopped")
	case s.choosing:
		s.choosing = false
		ctx.Frame.Status("Preview cancelled")
	case s.remote != nil:
		ctx.Frame.Status("A preview is already running in another view of this zone")
	default:
		s.choosing = true
		ctx.Frame.Status("Click the cell where the player starts. Escape cancels.")
	}
}

// updateChoosing starts a preview on the clicked cell, when the player can
// stand there.
func (s *State) updateChoosing(input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool) {
	if !input.Pressed || !onFloor {
		return
	}
	shown := play.Levels{{ID: s.floor, Floor: floorData}}
	if !play.CanStand(shown, s.floor, cell) {
		ctx.Frame.Status("The player cannot stand there. Choose a cell where nothing blocks.")

		return
	}
	s.choosing, s.starting = false, true
	floorID := s.floor
	ctx.Requests.PreviewStart(s.zone, func(result protocol.Result, err error) {
		s.starting = false
		if err != nil {
			ctx.Frame.Status("Preview not started: " + labelOr(result.Reason, err.Error()))

			return
		}
		s.preview = &previewRun{id: result.Created, run: play.Start(floorID, cell), held: 0, sent: nil}
		s.remote = nil
		s.subscribeFloors(ctx)
		ctx.Frame.Status("Preview running. Arrow keys walk; P stops.")
	})
}

// arrowHint explains what the arrow keys need when no preview is running in
// this window.
func (s *State) arrowHint(input *ui.Input, ctx *frame.Context) {
	if s.focus.Current() != nil {
		return
	}
	pressed := false
	for key := range arrows() {
		pressed = pressed || input.Has(key, ui.Plain())
	}
	switch {
	case !pressed:
	case s.choosing || s.starting:
		ctx.Frame.Status("First click the cell where the player starts; then the arrow keys walk.")
	case s.remote != nil:
		ctx.Frame.Status("The preview is running in another view of this zone; walk there.")
	default:
		ctx.Frame.Status("To walk, press P, then click the cell where the player starts.")
	}
}

// subscribeFloors subscribes to every floor of the zone, so the player can
// take stairs to any of them.
func (s *State) subscribeFloors(ctx *frame.Context) {
	for _, floorID := range s.floors {
		s.subscribe(floorID, ctx)
	}
}

// stepPreview runs one tick of this window's preview: the arrow keys walk
// the player, the view follows and switches floor with them, and the state
// goes to the other views. It must not run while a document is being read.
func (s *State) stepPreview(input *ui.Input, ctx *frame.Context) {
	var held play.Direction
	for key, direction := range arrows() {
		if s.focus.Current() == nil && input.IsDown(key) {
			held |= direction
		}
		for input.Consume(key, ui.Plain()) {
			continue
		}
	}
	pressed := held &^ s.preview.held
	s.preview.held = held
	zone := docsZone{floors: s.floors, docs: ctx.Docs}
	outcome := s.preview.run.Step(zone, play.Input{Held: held, Pressed: pressed})
	player := s.preview.run.Player()
	switch outcome {
	case play.Climbed:
		s.switchFloor(player.Floor, ctx)
		ctx.Frame.Status("Took the stairs to " + s.floorName(ctx.Docs, player.Floor))
	case play.NoFloor:
		ctx.Frame.Status("These stairs lead to a floor this zone does not have")
	case play.StairsBlocked:
		ctx.Frame.Status("The other end of these stairs is blocked or outside that floor")
	case play.Lost:
		s.stopPreview(ctx, "Preview stopped: the player's floor is no longer in the zone")

		return
	case play.Walking:
	}
	if player.Moving() || outcome == play.Climbed {
		s.follow(player)
	}
	s.sendState(ctx)
}

func (s *State) floorName(docs frame.Docs, floorID ids.ULID) string {
	name := "another floor"
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(floorID, name)
		}
	})

	return name
}

// follow pans the view to keep a walking player away from its edge.
func (s *State) follow(player play.Player) {
	size := s.camera.cellSize()
	grid := s.Grid()
	x, y := player.At()
	centre := grid.Min.Add(s.camera.offset).Add(image.Pt(
		int(math.Round((x+cellCentre)*float64(size))), int(math.Round((y+cellCentre)*float64(size))),
	))
	inner := grid.Inset(followMargin*size + size/ui.Half)
	delta := image.Pt(0, 0)
	switch {
	case centre.X < inner.Min.X:
		delta.X = inner.Min.X - centre.X
	case centre.X > inner.Max.X:
		delta.X = inner.Max.X - centre.X
	}
	switch {
	case centre.Y < inner.Min.Y:
		delta.Y = inner.Min.Y - centre.Y
	case centre.Y > inner.Max.Y:
		delta.Y = inner.Max.Y - centre.Y
	}
	s.camera.pan(delta)
}

// sendState sends the player's position to the zone's other views, at most
// every streamTicks ticks and only when it has changed.
func (s *State) sendState(ctx *frame.Context) {
	run := s.preview.run
	if run.Tick()%streamTicks != 0 {
		return
	}
	entity := entityOf(run.Player())
	if s.preview.sent != nil && *s.preview.sent == entity {
		return
	}
	s.preview.sent = &entity
	ctx.Requests.PreviewState(protocol.PreviewState{
		Run: s.preview.id, Tick: run.Tick(), Entities: []protocol.PreviewEntity{entity},
	})
}

func entityOf(player play.Player) protocol.PreviewEntity {
	x, y := player.At()

	return protocol.PreviewEntity{
		ID: protocol.EntityPlayer, Floor: player.Floor, X: x, Y: y, Face: [2]int{player.Facing.X, player.Facing.Y},
	}
}

func (s *State) stopPreview(ctx *frame.Context, message string) {
	ctx.Requests.PreviewStop(protocol.PreviewStop{Run: s.preview.id, Reason: "stopped in the window running it"})
	s.preview = nil
	ctx.Frame.Status(message)
}

// previewChanged follows a preview another window runs in this zone.
func (s *State) previewChanged(update client.Update, ctx *frame.Context) {
	switch {
	case update.PreviewStart != nil:
		s.remote = &remotePreview{id: update.PreviewStart.Run, tick: 0, player: nil}
		ctx.Frame.Status("A preview started in another view of this zone")
	case s.remote == nil:
	case update.PreviewState != nil && update.PreviewState.Run == s.remote.id:
		state := update.PreviewState
		if state.Tick <= s.remote.tick {
			return
		}
		s.remote.tick = state.Tick
		for index := range state.Entities {
			if state.Entities[index].ID == protocol.EntityPlayer {
				s.remote.player = &state.Entities[index]
			}
		}
	case update.PreviewStop != nil && update.PreviewStop.Run == s.remote.id:
		s.remote = nil
		ctx.Frame.Status("The preview in another view stopped")
	}
}

// shownPlayer returns the player of this window's preview or of another
// window's preview of the zone.
func (s *State) shownPlayer() (protocol.PreviewEntity, bool) {
	switch {
	case s.preview != nil:
		return entityOf(s.preview.run.Player()), true
	case s.remote != nil && s.remote.player != nil:
		return *s.remote.player, true
	}

	return protocol.PreviewEntity{}, false //nolint:exhaustruct // No player.
}

// drawPlayer draws the player on the shown floor: a disc with a dot towards
// where they face.
func (s *State) drawPlayer(area *ebiten.Image, grid image.Rectangle) {
	player, ok := s.shownPlayer()
	if !ok || player.Floor != s.floor {
		return
	}
	s.drawDisc(area, grid, player.X, player.Y, player.Face, fullAlpha)
}

// drawStartCell shows the player faintly on the cell under the cursor while
// the author chooses where the player starts, on cells where they can stand.
func (s *State) drawStartCell(area *ebiten.Image, floorData *floor.Floor, grid image.Rectangle) {
	shown := play.Levels{{ID: s.floor, Floor: floorData}}
	if !s.choosing || !s.hovering || !play.CanStand(shown, s.floor, s.hover) {
		return
	}
	s.drawDisc(area, grid, float64(s.hover.X), float64(s.hover.Y), [2]int{0, 1}, startAlpha)
}

// drawDisc draws the player at a position in cells: the wireframes' player
// token, a white disc with a blue ring, and a dot towards where they face.
func (s *State) drawDisc(area *ebiten.Image, grid image.Rectangle, x, y float64, face [2]int, alpha uint8) {
	size := float64(s.camera.cellSize())
	origin := grid.Min.Add(s.camera.offset)
	centreX := float64(origin.X) + (x+cellCentre)*size
	centreY := float64(origin.Y) + (y+cellCentre)*size
	radius := size * playerRadius
	blue := ui.HexAlpha(hexBlue, alpha)
	vector.FillCircle(area, float32(centreX), float32(centreY), float32(radius), ui.HexAlpha(hexWhite, alpha), true)
	vector.StrokeCircle(area, float32(centreX), float32(centreY), float32(radius), playerRing, blue, true)
	faceX, faceY := float64(face[0]), float64(face[1])
	if length := math.Hypot(faceX, faceY); length > 0 {
		faceX, faceY = faceX/length*radius*faceOffset, faceY/length*radius*faceOffset
	}
	vector.FillCircle(area, float32(centreX+faceX), float32(centreY+faceY), float32(radius*faceRadius), blue, true)
}

// drawPreviewInspector describes the preview above the keys.
func (s *State) drawPreviewInspector(screen *ebiten.Image, docs frame.Docs, left, top int) {
	lines := []string{}
	switch {
	case s.choosing || s.starting:
		lines = append(lines, "Click the cell where the player starts.", "Escape cancels.")
	case s.preview != nil:
		player := s.preview.run.Player()
		lines = append(lines, fmt.Sprintf("Player on %s, cell %d, %d", s.floorName(docs, player.Floor),
			player.Cell.X, player.Cell.Y), "Arrow keys walk.  P stops.")
	case s.remote != nil:
		lines = append(lines, "Running in another view of this zone.")
	default:
		return
	}
	s.theme.DrawLabel(screen, "PREVIEW", left, top, ui.ColorLive())
	for index, text := range lines {
		s.theme.DrawText(screen, text, left, top+(index+1)*ui.LineHeight, ui.ColorInk())
	}
}

// hasPlayer answers "player:C,R" for this window's preview and
// "remote-player:C,R" for another window's: the player stands still on that
// cell of any floor.
func (s *State) hasPlayer(value string, remote bool) bool {
	var col, row int
	if _, err := fmt.Sscanf(value, "%d,%d", &col, &row); err != nil {
		return false
	}
	if remote {
		player := s.remotePlayer()

		return player != nil && player.X == float64(col) && player.Y == float64(row)
	}
	if s.preview == nil {
		return false
	}
	player := s.preview.run.Player()

	return !player.Moving() && player.Cell == image.Pt(col, row)
}

func (s *State) remotePlayer() *protocol.PreviewEntity {
	if s.remote == nil {
		return nil
	}

	return s.remote.player
}

// locatePlayer finds the player on screen for input scripts.
func (s *State) locatePlayer() (image.Point, bool) {
	player, ok := s.shownPlayer()
	if !ok {
		return image.Pt(0, 0), false
	}
	size := float64(s.camera.cellSize())
	origin := s.Grid().Min.Add(s.camera.offset)

	return image.Pt(
		origin.X+int(math.Round((player.X+cellCentre)*size)), origin.Y+int(math.Round((player.Y+cellCentre)*size)),
	), true
}
