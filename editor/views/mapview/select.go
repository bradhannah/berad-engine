package mapview

import (
	"fmt"
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/pen"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// Select is the one tool that selects what sits on a cell (increment 65):
// an anchor, an NPC, an ambient sound, a portal, the player start, or the
// walls. A click selects one thing, and the inspector shows it with its
// controls; where a cell holds several, a menu at the cell asks which.
// Dragging the selected thing moves it. A click with Select never adds or
// changes anything, and the tools that add never select.

// thingKind is a kind of thing that can be selected on a cell.
type thingKind string

// The kinds, in the order a cell's things are listed.
const (
	kindNPC    thingKind = "npc"
	kindAnchor thingKind = "anchor"
	kindPortal thingKind = "portal"
	kindSound  thingKind = "sound"
	kindStart  thingKind = "start"
	kindWalls  thingKind = "walls"
	// selectItem prefixes a kind in the menu that asks which of a cell's
	// things to select.
	selectItem = "select:"
)

// thing is one thing on a cell: its kind, its ID where it has one, the cell,
// and what the menu that asks which calls it.
type thing struct {
	kind  thingKind
	id    ids.ULID
	cell  Cell
	label string
}

// thingDrag is a press with Select on a cell that holds something: the thing
// a drag from there moves, if one is settled, and where the press was. ask
// says a click there, released where it was pressed, asks which of the
// cell's things to select.
type thingDrag struct {
	thing   thing
	settled bool
	from    Cell
	ask     bool
}

// thingsAt lists what can be selected on a cell, in a fixed order: the NPC,
// the anchor, the portal, the ambient sound, the player start, and the run
// of walls when the cell is a wall.
func (s *State) thingsAt(floorData *floor.Floor, cell Cell) []thing {
	var things []thing
	add := func(kind thingKind, id ids.ULID, label string) {
		things = append(things, thing{kind: kind, id: id, cell: cell, label: label})
	}
	if placement, found := floorData.PlacementAt(cell.X, cell.Y); found {
		add(kindNPC, placement.ID, "NPC: "+s.names.Name(placement.NPC, "its definition is gone"))
	}
	if anchor, found := floorData.AnchorAt(cell.X, cell.Y); found {
		add(kindAnchor, anchor.ID, "Anchor: "+anchor.Name)
	}
	if portal, found := floorData.PortalAt(cell.X, cell.Y); found {
		add(kindPortal, portal.ID, "Portal to "+s.portalNamed(portal.ID))
	}
	if sound, found := floorData.SoundAt(cell.X, cell.Y); found {
		add(kindSound, sound.ID, "Ambient sound: "+s.cueName(sound.Cue))
	}
	if s.startHere(cell) {
		add(kindStart, ids.ULID{}, "Player start")
	}
	if isWallAt(floorData, cell) {
		add(kindWalls, ids.ULID{}, "The run of walls")
	}

	return things
}

// portalNamed is the anchor a portal leads to, by name, or "nowhere".
func (s *State) portalNamed(portalID ids.ULID) string {
	leads := s.portals.leads[portalID]
	if leads.state != gameload.Held || leads.anchor == "" {
		return nowhereLabel
	}

	return leads.anchor
}

// clearSelection selects nothing. An unsaved anchor name is asked about
// before the inspector leaves it, as when another anchor is selected.
func (s *State) clearSelection() {
	s.selected, s.anchor, s.placement = nil, ids.ULID{}, ids.ULID{}
	s.sound, s.portal, s.startPicked = ids.ULID{}, ids.ULID{}, false
}

// anySelected reports whether anything is selected.
func (s *State) anySelected() bool {
	return !s.anchor.IsZero() || !s.placement.IsZero() || !s.sound.IsZero() || !s.portal.IsZero() || s.startPicked ||
		len(s.selected) > 0
}

// selectThing selects one thing, and nothing else.
func (s *State) selectThing(floorData *floor.Floor, chosen thing) {
	switch chosen.kind {
	case kindAnchor:
		s.selectAnchor(chosen.id)
	case kindNPC:
		s.selectPlacement(chosen.id)
	case kindSound:
		s.clearSelection()
		s.sound = chosen.id
	case kindPortal:
		s.clearSelection()
		s.portal = chosen.id
	case kindStart:
		s.clearSelection()
		s.startPicked = true
	case kindWalls:
		s.clearSelection()
		s.selected = WallRun(floorData, chosen.cell)
	}
	s.focus.Set(nil)
}

// isSelected reports whether a thing is the one selected.
func (s *State) isSelected(candidate thing) bool {
	switch candidate.kind {
	case kindAnchor:
		return s.anchor == candidate.id
	case kindNPC:
		return s.placement == candidate.id
	case kindSound:
		return s.sound == candidate.id
	case kindPortal:
		return s.portal == candidate.id
	case kindStart:
		return s.startPicked
	case kindWalls:
	}

	return false
}

// syncSelection drops a selected sound or portal that is gone from the
// floor, and the player start when it is not on the floor shown.
func (s *State) syncSelection(floorData *floor.Floor) {
	if _, _, found := floorData.Sound(s.sound); !found {
		s.sound = ids.ULID{}
	}
	if _, _, found := floorData.Portal(s.portal); !found {
		s.portal = ids.ULID{}
	}
	start := s.playerStart
	if start.IsZero() || start.Zone != s.zone || start.Floor != s.floor {
		s.startPicked = false
	}
}

// updateSelect handles the Select tool. A press on a cell that holds one
// thing selects it; on a cell that holds several, releasing where it was
// pressed asks which. A drag from the selected thing's cell, or from a cell
// with one thing, moves it. A wall alone selects its run, a double-click
// inside a room its walls, and a drag from an empty cell the walls in the
// area, as before; a click on nothing selects nothing.
func (s *State) updateSelect(input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool) {
	if input.Pressed && onFloor {
		s.pressSelect(input, ctx, floorData, cell)
	}
	if s.rect != nil && input.Down && onFloor {
		s.rect.to = cell
	}
	if s.rect != nil && !input.Down {
		s.selected = WallsIn(floorData, s.rect.from, s.rect.to)
		if s.rect.from != s.rect.to || len(s.selected) > 0 {
			ctx.Frame.Status(fmt.Sprintf("Selected %d walls", len(s.selected)))
		}
		s.rect = nil
	}
	if s.moving != nil && !input.Down {
		drag := s.moving
		s.moving = nil
		s.releaseSelect(ctx, floorData, *drag, cell, onFloor)
	}
}

// pressSelect takes a press with Select on a cell of the floor.
func (s *State) pressSelect(input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell) {
	things := s.thingsAt(floorData, cell)
	switch {
	case input.DoubleClick && len(things) == 0:
		walls, closed := RoomWalls(floorData, cell)
		if !closed {
			ctx.Frame.Refuse("That space is not closed in by walls and doors")

			return
		}
		s.clearSelection()
		s.selected = walls
		ctx.Frame.Status(fmt.Sprintf("Selected the %d walls around this room", len(walls)))
	case len(things) == 0:
		s.clearSelection()
		s.rect = &rectDrag{from: cell, to: cell}
	case len(things) == 1 && things[0].kind == kindWalls:
		s.selectThing(floorData, things[0])
		ctx.Frame.Status(fmt.Sprintf("Selected a run of %d walls", len(s.selected)))
	case len(things) == 1:
		s.selectThing(floorData, things[0])
		s.moving = &thingDrag{thing: things[0], settled: true, from: cell, ask: false}
	default:
		drag := &thingDrag{thing: things[0], settled: false, from: cell, ask: true}
		for _, each := range things {
			if s.isSelected(each) {
				drag.thing, drag.settled = each, true
			}
		}
		s.moving = drag
	}
}

// releaseSelect ends a press with Select: released where it was pressed on
// a cell with several things, it asks which to select; released on another
// cell, it moves the thing the drag settled on.
func (s *State) releaseSelect(ctx *frame.Context, floorData *floor.Floor, drag thingDrag, cell Cell, onFloor bool) {
	switch {
	case onFloor && cell == drag.from && drag.ask:
		s.askWhich(floorData, cell)
	case !onFloor || cell == drag.from || !drag.settled:
	default:
		s.moveThing(ctx, floorData, drag.thing, cell)
	}
}

// askWhich opens a menu at a cell that lists its things, to select one.
func (s *State) askWhich(floorData *floor.Floor, cell Cell) {
	things := s.thingsAt(floorData, cell)
	items := make([]ui.MenuItem, 0, len(things))
	for _, each := range things {
		items = append(items, menuItem(selectItem+string(each.kind), each.label, false))
	}
	at := pen.WindowRect(s.camera.cellRect(cell, s.stage())).Max
	s.menu = &cellMenu{Menu: ui.Menu{Items: items, Width: ui.MenuWidth(items, s.theme)}, cell: cell}
	s.menu.OpenAt(at, s.Grid())
}

// chooseWhich selects the thing chosen in the menu that asks which, and
// reports whether the choice was one of its.
func (s *State) chooseWhich(ctx *frame.Context, floorData *floor.Floor, chosen string, cell Cell) bool {
	kind, asked := strings.CutPrefix(chosen, selectItem)
	if !asked {
		return false
	}
	for _, each := range s.thingsAt(floorData, cell) {
		if string(each.kind) == kind {
			s.selectThing(floorData, each)
			ctx.Frame.Status("Selected " + lowerFirst(each.label) + ". Drag it to move it.")
		}
	}

	return true
}

// lowerFirst starts a label with a small letter, for the middle of a
// sentence.
func lowerFirst(label string) string {
	if label == "" || strings.HasPrefix(label, "NPC") {
		return label
	}

	return strings.ToLower(label[:1]) + label[1:]
}

// moveThing moves a thing to another cell of its floor: one edit, with
// undo. A cell that already holds one of its kind refuses.
func (s *State) moveThing(ctx *frame.Context, floorData *floor.Floor, moved thing, target Cell) {
	switch moved.kind {
	case kindAnchor:
		s.moveAnchor(ctx, floorData, moved.id, target)
	case kindNPC:
		s.movePlacement(ctx, floorData, moved.id, target)
	case kindSound:
		s.moveSound(ctx, floorData, moved.id, target)
	case kindPortal:
		s.movePortal(ctx, floorData, moved.id, target)
	case kindStart:
		s.moveStart(ctx, floorData, target)
	case kindWalls:
	}
}

func (s *State) moveAnchor(ctx *frame.Context, floorData *floor.Floor, anchorID ids.ULID, target Cell) {
	moved, _, found := floorData.Anchor(anchorID)
	_, taken := floorData.AnchorAt(target.X, target.Y)
	switch {
	case !found:
	case taken:
		ctx.Frame.Refuse("Another anchor is already there")
	default:
		moved.Col, moved.Row = target.X, target.Y
		says := fmt.Sprintf("Moved %s to %d, %d", anchorName(moved.Name), target.X, target.Y)
		s.commandSaying(ctx, s.floor, tilefloor.UpdateAnchorEdit, "", says, tilefloor.UpdateAnchorArgs{Anchor: moved}, nil)
	}
}

func (s *State) movePlacement(ctx *frame.Context, floorData *floor.Floor, placementID ids.ULID, target Cell) {
	moved, _, found := floorData.Placement(placementID)
	_, taken := floorData.PlacementAt(target.X, target.Y)
	switch {
	case !found:
	case taken:
		ctx.Frame.Refuse("Another NPC is already there")
	default:
		moved.Col, moved.Row = target.X, target.Y
		says := fmt.Sprintf("Moved %s to %d, %d", s.names.Name(moved.NPC, "an NPC"), target.X, target.Y)
		update := tilefloor.UpdatePlacementArgs{Placement: moved}
		s.commandSaying(ctx, s.floor, tilefloor.UpdatePlacementEdit, "", says, update, nil)
	}
}

func (s *State) moveSound(ctx *frame.Context, floorData *floor.Floor, soundID ids.ULID, target Cell) {
	moved, _, found := floorData.Sound(soundID)
	_, taken := floorData.SoundAt(target.X, target.Y)
	switch {
	case !found:
	case taken:
		ctx.Frame.Refuse("Another ambient sound is already there")
	default:
		moved.Col, moved.Row = target.X, target.Y
		s.command(ctx, s.floor, tilefloor.UpdateSoundEdit, "", tilefloor.UpdateSoundArgs{Sound: moved},
			func(protocol.Result) {
				ctx.Frame.Status(fmt.Sprintf("Moved the ambient sound to %d,%d", target.X, target.Y))
			})
	}
}

func (s *State) movePortal(ctx *frame.Context, floorData *floor.Floor, portalID ids.ULID, target Cell) {
	moved, _, found := floorData.Portal(portalID)
	_, taken := floorData.PortalAt(target.X, target.Y)
	switch {
	case !found:
	case taken:
		ctx.Frame.Refuse("Another portal is already there")
	default:
		moved.Col, moved.Row = target.X, target.Y
		s.command(ctx, s.floor, tilefloor.UpdatePortalEdit, "", tilefloor.UpdatePortalArgs{Portal: moved},
			func(protocol.Result) { ctx.Frame.Status(fmt.Sprintf("Moved the portal to %d,%d", target.X, target.Y)) })
	}
}

func (s *State) moveStart(ctx *frame.Context, floorData *floor.Floor, target Cell) {
	if !s.canStart(floorData, target) {
		ctx.Frame.Refuse("The player cannot start there: nothing can stand on that cell")

		return
	}
	start := game.Start{Zone: s.zone, Floor: s.floor, Col: target.X, Row: target.Y}
	s.command(ctx, game.SettingsID(), gameplugin.StartEdit, "", gameplugin.StartArgs{Start: start},
		func(protocol.Result) {
			ctx.Frame.Status(fmt.Sprintf("Moved the player start to %d,%d. Launch plays from here.", target.X, target.Y))
		})
}

// drawPicked outlines in blue the cell of the selected thing, and where a
// thing being dragged would land. An anchor and an NPC also show they are
// selected by their own rings, but one may stand over the other on a cell,
// so the cell is outlined for them too.
func (s *State) drawPicked(area *ebiten.Image, floorData *floor.Floor, grid image.Rectangle) {
	outline := func(cell Cell) {
		if rect := s.camera.cellRect(cell, grid); rect.Overlaps(grid) {
			ui.StrokeRect(area, rect, ui.Outline, ui.ColorSelect())
		}
	}
	if anchor, _, found := floorData.Anchor(s.anchor); found && !s.anchor.IsZero() {
		outline(Cell{X: anchor.Col, Y: anchor.Row})
	}
	if placement, _, found := floorData.Placement(s.placement); found && !s.placement.IsZero() {
		outline(Cell{X: placement.Col, Y: placement.Row})
	}
	if sound, _, found := floorData.Sound(s.sound); found && !s.sound.IsZero() {
		outline(Cell{X: sound.Col, Y: sound.Row})
	}
	if portal, _, found := floorData.Portal(s.portal); found && !s.portal.IsZero() {
		outline(Cell{X: portal.Col, Y: portal.Row})
	}
	if s.startPicked {
		outline(Cell{X: s.playerStart.Col, Y: s.playerStart.Row})
	}
	if s.moving != nil && s.moving.settled && s.hovering && s.hover != s.moving.from {
		outline(s.hover)
	}
}

// hasSelected answers "selected:none", and "selected:<kind>:C,R" when the
// thing of that kind on that cell is the one selected, for input scripts.
func (s *State) hasSelected(value string, floorData *floor.Floor) bool {
	if value == "none" {
		return !s.anySelected()
	}
	kind, where, _ := strings.Cut(value, ":")
	var cell Cell
	if _, err := fmt.Sscanf(where, "%d,%d", &cell.X, &cell.Y); err != nil {
		return false
	}
	for _, each := range s.thingsAt(floorData, cell) {
		if string(each.kind) == kind {
			return s.isSelected(each)
		}
	}

	return false
}
