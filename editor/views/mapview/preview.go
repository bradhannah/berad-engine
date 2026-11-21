package mapview

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"image"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/pen"
	"github.com/bradhannah/berad-engine/editor/views/docsource"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/play"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/worldstate"
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
)

// previewRun is the preview this window runs (D15): the engine's game
// runtime, playing the zone from the cell the author clicked, as Launch and a
// game play it (increment 63). Its state goes to the zone's other views
// through the orchestrator. NPCs can be paused, or stepped until the watched
// NPC changes node; step is the changes it made in the latest step.
type previewRun struct {
	id   ids.ULID
	game *game.Game
	// from is where the player started, for gathering the project again
	// after an edit; read is the documents the last gathering asked for, and
	// stamp what they were as it read them.
	from     game.Start
	read     []ids.ULID
	stamp    uint64
	held     play.Direction
	sent     *protocol.PreviewState
	watched  ids.ULID
	paused   bool
	stepping bool
	// stepTicks counts the ticks the step under way has run.
	stepTicks int
	step      []protocol.PreviewChange
	// heard counts the times each cue has played in this preview.
	heard map[string]int
	// anchors names the anchors of the zone's floors, for the watched
	// NPC's place values.
	anchors map[ids.ULID]string
	// lines are the latest lines the Logs wrote, at most
	// protocol.RecentLines, and written counts every line so far.
	lines   []protocol.PreviewLine
	written uint64
	// trail is the watched NPC's latest changes of node, at most
	// protocol.TrailLength, and passed counts every one so far.
	trail  []protocol.PreviewTrail
	passed uint64
	// named is the signals the project declares, to name them.
	named []worldstate.Signal
}

// remotePreview is a preview another window runs in this zone, and its
// latest state.
type remotePreview struct {
	id     ids.ULID
	tick   uint64
	player *protocol.PreviewEntity
	state  *protocol.PreviewState
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
		ctx.Frame.Refuse("A preview is already running in another view of this zone")
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
	if !s.canStart(floorData, cell) {
		ctx.Frame.Refuse("The player cannot stand there. Choose a cell where nothing blocks.")

		return
	}
	s.startPreviewAt(ctx, cell)
}

// canStart reports whether the player can start a preview on a cell.
func (s *State) canStart(floorData *floor.Floor, cell Cell) bool {
	return play.CanStand(play.Levels{{ID: s.floor, Floor: floorData}}, s.floor, cell)
}

// startPreviewAt starts a preview with the player on a cell of the current
// floor.
func (s *State) startPreviewAt(ctx *frame.Context, cell Cell) {
	s.choosing, s.starting = false, true
	s.askAudio(ctx)
	from := game.Start{Zone: s.zone, Floor: s.floor, Col: cell.X, Row: cell.Y}
	ctx.Requests.PreviewStart(s.zone, func(result protocol.Result, err error) {
		s.starting = false
		if err != nil {
			ctx.Frame.Status("Preview not started: " + labelOr(result.Reason, err.Error()))

			return
		}
		loaded, read, stamp := s.gatherPreview(ctx, from)
		loaded.Project.Seconds, loaded.Project.Seed = s.start, cmp.Or(ctx.Seed, seedOf(result.Created))
		played, err := game.New(loaded.Project)
		if err != nil {
			ctx.Requests.PreviewStop(protocol.PreviewStop{Run: result.Created, Reason: "the game could not start"})
			ctx.Frame.Status("Preview not started: " + err.Error())

			return
		}
		played.KnowCues(s.cueTicks)
		s.preview = &previewRun{
			id: result.Created, game: played, from: from, read: read, stamp: stamp,
			held: 0, sent: nil, watched: ids.ULID{}, paused: false, stepping: false, stepTicks: 0, step: nil,
			heard: map[string]int{}, anchors: map[ids.ULID]string{}, lines: nil, written: 0, trail: nil, passed: 0,
			named: loaded.Project.Signals,
		}
		if len(loaded.Cues) > 0 {
			ctx.Sounds.Prepare(loaded.Cues)
		}
		s.remote = nil
		s.subscribeFloors(ctx)
		ctx.Frame.Status("Preview running from " + clock.Format(played.Clock().Seconds()) +
			". Arrow keys walk; click an NPC to watch it; P stops.")
	})
}

// gatherPreview gathers what a preview plays from the window's documents,
// as a game gathers it, subscribing to each document it asks for. It lists
// those documents, with a stamp of what they were as it read them, so a
// change to one, or its arrival, is seen. What has not arrived yet is left
// out until it does.
func (s *State) gatherPreview(ctx *frame.Context, from game.Start) (gameload.Loaded, []ids.ULID, uint64) {
	asking := docsource.New(ctx.Docs).Asking(func(documentID ids.ULID) { s.subscribe(documentID, ctx) })
	loaded, _ := gameload.Load(asking, &from)
	loaded.Project.Names = places.NewNames(ctx.Docs)
	if loaded.Project.Values == nil {
		// Until the world state arrives the preview keeps values that
		// declare nothing, so its value nodes find none.
		loaded.Project.Values = worldstate.NewCatalog(worldstate.NewDeclarations(worldstate.HoldsWorldState), nil)
	}

	read, stamp := asking.Seen()

	return loaded, read, stamp
}

// refreshPreview gives the running preview the project as it is now, when a
// document it was gathered from has changed or arrived: an edit made while
// it runs is walked, followed and heard at once, and an NPC whose workflow
// changed starts it again.
func (s *State) refreshPreview(ctx *frame.Context) {
	docs := docsource.New(ctx.Docs)
	if docs.Stamp(s.preview.read) == s.preview.stamp {
		return
	}
	loaded, read, stamp := s.gatherPreview(ctx, s.preview.from)
	s.preview.game.Update(loaded.Project)
	s.preview.read, s.preview.stamp, s.preview.named = read, stamp, loaded.Project.Signals
	if len(loaded.Cues) > 0 {
		ctx.Sounds.Prepare(loaded.Cues)
	}
}

// seedOf is a preview's seed: its run's, so each preview differs and a run
// repeats.
func seedOf(run ids.ULID) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write(run[:])

	return hash.Sum64()
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
	stage := s.preview.game.Stage()
	if stage.Holding() {
		held = 0
		if input.Consume(ui.KeyEnter, ui.Plain()) || input.Consume(ui.KeySpace, ui.Plain()) {
			stage.Close()
		}
	}
	pressed := held &^ s.preview.held
	s.preview.held = held
	s.refreshPreview(ctx)
	outcome := s.preview.game.Walk(play.Input{Held: held, Pressed: pressed})
	player := s.preview.game.Player()
	switch outcome {
	case play.Climbed:
		s.switchFloor(player.Floor, ctx)
		ctx.Frame.Status("Took the stairs to " + s.floorName(ctx.Docs, player.Floor))
	case play.NoFloor:
		ctx.Frame.Refuse("These stairs lead to a floor this zone does not have")
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
	s.stepNPCs(ctx)
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
	grid := s.stage()
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

// sameState reports whether a preview's state is as it was sent.
func sameState(sent, state protocol.PreviewState) bool {
	return slices.EqualFunc(sent.Entities, state.Entities, sameEntity) && sent.Clock == state.Clock &&
		sent.Rate == state.Rate && sent.ClockStopped == state.ClockStopped && sent.NPCsPaused == state.NPCsPaused &&
		sent.Watched == state.Watched && sent.Stepping == state.Stepping && slices.Equal(sent.Step, state.Step) &&
		sameBox(sent.Box, state.Box) && sent.ShakeX == state.ShakeX && sent.ShakeY == state.ShakeY &&
		sameLog(sent.Log, state.Log)
}

// sameLog reports whether a preview's lines end where they did when sent.
func sameLog(sent, now []protocol.PreviewLine) bool {
	return len(sent) == len(now) && (len(now) == 0 || sent[len(sent)-1] == now[len(now)-1])
}

// sameEntity reports whether an entity is as it was sent.
func sameEntity(sent, now protocol.PreviewEntity) bool {
	return slices.Equal(sent.Calls, now.Calls) && sent.ID == now.ID && sent.Floor == now.Floor && sent.X == now.X &&
		sent.Y == now.Y && sent.Face == now.Face && sent.NPC == now.NPC && sent.Workflow == now.Workflow &&
		sent.Node == now.Node && sent.Activity == now.Activity && sent.Status == now.Status && sent.Last == now.Last &&
		sameDoing(sent, now)
}

// sameDoing reports whether what an entity is doing, beside its node, is as
// it was sent: its changes, its line, its lanes and its signals.
func sameDoing(sent, now protocol.PreviewEntity) bool {
	return sent.Changes == now.Changes && sent.Saying == now.Saying && slices.Equal(sent.Lanes, now.Lanes) &&
		sent.Handling == now.Handling && sent.Resumes == now.Resumes && slices.Equal(sent.Held, now.Held)
}

// sendState sends the preview's state, the player, the NPCs and the clock,
// to the zone's other views, at most every streamTicks ticks and only when
// it has changed.
func (s *State) sendState(ctx *frame.Context) {
	if s.preview.game.Ticks()%streamTicks != 0 {
		return
	}
	state := s.previewState()
	if sent := s.preview.sent; sent != nil && sameState(*sent, state) {
		return
	}
	s.preview.sent = &state
	ctx.Requests.PreviewState(state)
}

func entityOf(player play.Player) protocol.PreviewEntity {
	x, y := player.At()

	return protocol.PreviewEntity{
		ID: protocol.EntityPlayer, Floor: player.Floor, X: x, Y: y, Face: [2]int{player.Facing.X, player.Facing.Y},
		NPC: ids.ULID{}, Workflow: ids.ULID{}, Node: ids.ULID{}, Activity: "", Status: "",
		Last:    protocol.PreviewChange{From: ids.ULID{}, Output: "", To: ids.ULID{}, Tick: 0, Status: "", Lane: "", Why: ""},
		Changes: 0, Calls: nil, Way: nil, Loop: ids.ULID{}, Sprite: ids.ULID{}, Saying: "", SayingLeft: 0, Lanes: nil,
		Handling: "", HandlingFrom: "", Resumes: "", Held: nil,
	}
}

func (s *State) stopPreview(ctx *frame.Context, message string) {
	ctx.Requests.PreviewStop(protocol.PreviewStop{Run: s.preview.id, Reason: "stopped in the Map running it"})
	ctx.Sounds.Stop()
	s.preview = nil
	ctx.Frame.Status(message)
}

// previewChanged follows a preview another window runs in this zone.
func (s *State) previewChanged(update client.Update, ctx *frame.Context) {
	switch {
	case update.PreviewStart != nil:
		s.remote = &remotePreview{id: update.PreviewStart.Run, tick: 0, player: nil, state: nil}
		ctx.Frame.Status("A preview started in another view of this zone")
	case s.remote == nil:
	case update.PreviewState != nil && update.PreviewState.Run == s.remote.id:
		state := update.PreviewState
		if state.Tick <= s.remote.tick {
			return
		}
		s.remote.tick, s.remote.state = state.Tick, state
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
		return entityOf(s.preview.game.Player()), true
	case s.remote != nil && s.remote.player != nil:
		return *s.remote.player, true
	}

	return protocol.PreviewEntity{}, false //nolint:exhaustruct_v5 // No player.
}

// drawPlayer draws the player on the shown floor: their sprite, or a disc
// with a dot towards where they face.
func (s *State) drawPlayer(area *ebiten.Image, grid image.Rectangle) {
	player, ok := s.shownPlayer()
	if !ok || player.Floor != s.floor {
		return
	}
	if picture, shown := s.sprites.image(art.Player); shown {
		drawSprite(area, picture, s.entityRect(grid, player.X, player.Y), fullAlpha)

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
	pen.FillCircle(area, float32(centreX), float32(centreY), float32(radius), ui.HexAlpha(hexWhite, alpha), true)
	pen.StrokeCircle(area, float32(centreX), float32(centreY), float32(radius), playerRing, blue, true)
	faceX, faceY := float64(face[0]), float64(face[1])
	if length := math.Hypot(faceX, faceY); length > 0 {
		faceX, faceY = faceX/length*radius*faceOffset, faceY/length*radius*faceOffset
	}
	pen.FillCircle(area, float32(centreX+faceX), float32(centreY+faceY), float32(radius*faceRadius), blue, true)
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
	player := s.preview.game.Player()

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
	origin := s.stage().Min.Add(s.camera.offset)

	return pen.WindowPoint(image.Pt(
		origin.X+int(math.Round((player.X+cellCentre)*size)), origin.Y+int(math.Round((player.Y+cellCentre)*size)),
	)), true
}
