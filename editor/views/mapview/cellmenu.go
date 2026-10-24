package mapview

import (
	"fmt"
	"image"
	"strconv"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// The right-click menu's choices.
const (
	menuWatch         = "watch-npc"
	menuSelectNPC     = "select-npc"
	menuOpenBehaviour = "open-behaviour"
	menuRemoveNPC     = "remove-npc"
	menuRenameAnchor  = "rename-anchor"
	menuDeleteAnchor  = "delete-anchor"
	menuSelectWalls   = "select-walls"
	menuAddAnchor     = "add-anchor"
	menuPlaceNPC      = "place-npc"
	menuClearTop      = "clear-top"
	menuPreviewHere   = "preview-here"
	menuExtendFloor   = "extend-floor"
)

// cellMenu is the right-click menu and the cell it was opened on.
type cellMenu struct {
	ui.Menu

	cell Cell
}

func menuItem(id, label string, disabled bool) ui.MenuItem {
	return ui.MenuItem{ID: id, Label: label, Icon: ui.IconNone, Disabled: disabled, Danger: false, Rule: false}
}

func dangerItem(id, label string) ui.MenuItem {
	item := menuItem(id, label, false)
	item.Danger = true

	return item
}

// openMenu opens the right-click menu for a cell at the cursor.
func (s *State) openMenu(floorData *floor.Floor, cell Cell, onFloor bool, at image.Point) {
	items := s.menuItems(floorData, cell, onFloor)
	s.menu = &cellMenu{Menu: ui.Menu{Items: items, Width: ui.MenuWidth(items, s.theme)}, cell: cell}
	s.menu.OpenAt(at, s.Grid())
}

// menuItems lists what can be done at a cell: first for what is there, then
// for the cell itself. Beyond the floor bounds, the floor can grow to it.
func (s *State) menuItems(floorData *floor.Floor, cell Cell, onFloor bool) []ui.MenuItem {
	if !onFloor {
		_, fits := extendTo(floorData.Bounds(), cell)

		return []ui.MenuItem{menuItem(menuExtendFloor, "Extend floor to here", !fits)}
	}
	live := s.preview != nil || s.remote != nil
	var items []ui.MenuItem
	if _, onNPC := s.liveNPCAt(cell); live && onNPC {
		items = append(items, menuItem(menuWatch, "Watch NPC", false))
	}
	_, placed := floorData.PlacementAt(cell.X, cell.Y)
	if placed && !live {
		items = append(items, menuItem(menuSelectNPC, "Select NPC", false),
			menuItem(menuOpenBehaviour, "Open behaviour", false), dangerItem(menuRemoveNPC, "Remove from map"))
	}
	_, anchored := floorData.AnchorAt(cell.X, cell.Y)
	if anchored {
		items = append(items, menuItem(menuRenameAnchor, "Rename anchor", false),
			dangerItem(menuDeleteAnchor, "Delete anchor"))
	}
	if isWallAt(floorData, cell) {
		items = append(items, menuItem(menuSelectWalls, "Select walls", false))
	}
	here := []ui.MenuItem{menuItem(menuAddAnchor, "Add anchor here", anchored)}
	if name, chosen := s.chosenNPC(); chosen {
		here = append(here, menuItem(menuPlaceNPC, "Place “"+name+"” here", placed))
	} else {
		here = append(here, menuItem(menuPlaceNPC, "Place an NPC here", true))
	}
	if top, _ := floorData.Top(cell.X, cell.Y); top != "" { //nolint:errcheck // The cell is on the floor.
		here = append(here, menuItem(menuClearTop, "Remove “"+tileName(top)+"”", false))
	}
	busy := live || s.choosing || s.starting
	here = append(here, menuItem(menuPreviewHere, "Preview from here", busy || !s.canStart(floorData, cell)))
	if len(items) > 0 {
		here[0].Rule = true
	}

	return append(items, here...)
}

// chosenNPC returns the name of the NPC chosen in the NPC list, if any.
func (s *State) chosenNPC() (string, bool) {
	npcID, err := ids.Parse(s.tile[ToolNPC])
	if err != nil {
		return "", false
	}
	name := s.names.Name(npcID, "")

	return name, name != ""
}

// extendTo returns the floor bounds grown to include a cell, and whether the
// floor may be that large.
func extendTo(bounds image.Rectangle, cell Cell) (image.Rectangle, bool) {
	next := bounds.Union(image.Rectangle{Min: cell, Max: cell.Add(image.Pt(1, 1))})

	return next, next.Dx() <= floor.MaxSide && next.Dy() <= floor.MaxSide
}

// updateMenu handles the open right-click menu and carries out a choice.
func (s *State) updateMenu(input *ui.Input, ctx *frame.Context) {
	menu := s.menu
	chosen := menu.Update(input)
	if !menu.IsOpen() {
		s.menu = nil
	}
	if chosen == "" {
		return
	}
	ctx.Docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			s.choose(ctx, floorDoc.Floor, chosen, menu.cell)
		}
	})
}

// choose carries out a right-click menu choice, selecting what it acts on
// first, as a click would. Each change is an ordinary command with undo.
func (s *State) choose(ctx *frame.Context, floorData *floor.Floor, chosen string, cell Cell) {
	switch chosen {
	case menuWatch:
		if placement, onNPC := s.liveNPCAt(cell); onNPC {
			s.watch(placement, ctx)
		}
	case menuSelectNPC, menuOpenBehaviour, menuRemoveNPC:
		s.choosePlacement(ctx, floorData, chosen, cell)
	case menuRenameAnchor, menuDeleteAnchor:
		s.chooseAnchor(ctx, floorData, chosen, cell)
	case menuSelectWalls:
		s.setTool(ToolSelect)
		s.anchor, s.placement = ids.ULID{}, ids.ULID{}
		s.selected = WallRun(floorData, cell)
		ctx.Frame.Status(fmt.Sprintf("Selected a run of %d walls. Pick a family on the left to change them.",
			len(s.selected)))
	case menuAddAnchor:
		s.addAnchor(ctx, floorData, cell)
	case menuPlaceNPC:
		s.placeNPC(ctx, floorData, cell)
	case menuClearTop:
		s.stroke.count++
		gesture := s.window.String() + "-clear-" + strconv.Itoa(s.stroke.count)
		cleared := tilefloor.Cell{Col: cell.X, Row: cell.Y, Layer: tilefloor.LayerTop, Tile: ""}
		s.command(ctx, s.floor, tilefloor.PaintEdit, gesture, tilefloor.PaintArgs{Cells: []tilefloor.Cell{cleared}}, nil)
	case menuPreviewHere:
		s.startPreviewAt(ctx, cell)
	case menuExtendFloor:
		if next, fits := extendTo(floorData.Bounds(), cell); fits {
			s.requestResize(ctx, floorData, next)
		}
	}
}

// choosePlacement selects the NPC placement on a cell, then opens its
// behaviour or removes it.
func (s *State) choosePlacement(ctx *frame.Context, floorData *floor.Floor, chosen string, cell Cell) {
	placement, found := floorData.PlacementAt(cell.X, cell.Y)
	if !found {
		return
	}
	s.selectPlacement(placement.ID)
	switch chosen {
	case menuOpenBehaviour:
		s.openLoaded = placement.ID
	case menuRemoveNPC:
		s.removePlacement(ctx)
	}
}

// openWhenLoaded opens the behaviour asked for from the right-click menu
// once the placement's NPC has loaded, as its behaviour is read from it. It
// gives up if the placement is no longer selected.
func (s *State) openWhenLoaded(ctx *frame.Context) {
	if s.openLoaded.IsZero() {
		return
	}
	if s.openLoaded != s.placement {
		s.openLoaded = ids.ULID{}

		return
	}
	if ctx.Docs.View(s.placedNPC, func(client.Document) {}) {
		s.openLoaded = ids.ULID{}
		s.openBehaviour(ctx)
	}
}

// chooseAnchor selects the anchor on a cell, then focuses its name or
// deletes it. An unsaved name of another anchor is asked about first.
func (s *State) chooseAnchor(ctx *frame.Context, floorData *floor.Floor, chosen string, cell Cell) {
	anchor, found := floorData.AnchorAt(cell.X, cell.Y)
	if !found {
		return
	}
	switch {
	case chosen == menuDeleteAnchor:
		s.selectAnchor(anchor.ID)
		s.deleteAnchor(ctx)
	case s.nameField.Field.Unsaved() && s.named != anchor.ID:
		s.selectAnchor(anchor.ID)
	default:
		s.selectAnchor(anchor.ID)
		s.named = anchor.ID
		s.nameField.Field.SetSaved(anchor.Name)
		s.focus.Set(s.nameField)
		if buffer := s.nameField.Field.Buffer(); buffer != nil {
			buffer.SelectAll()
		}
	}
}
