package mapview

import (
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
	menuWatch        = "watch-npc"
	menuRemoveNPC    = "remove-npc"
	menuDeleteAnchor = "delete-anchor"
	menuAddAnchor    = "add-anchor"
	menuPlaceNPC     = "place-npc"
	menuClearTop     = "clear-top"
	menuPreviewHere  = "preview-here"
	menuExtendFloor  = "extend-floor"
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

// openMenu opens the right-click menu for a cell at the cursor, and lists
// the audio library again for its ambient cues.
func (s *State) openMenu(ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool, at image.Point) {
	s.askAudio(ctx)
	items := s.menuItems(floorData, cell, onFloor)
	s.menu = &cellMenu{Menu: ui.Menu{Items: items, Width: ui.MenuWidth(items, s.theme)}, cell: cell}
	s.menu.OpenAt(at, s.Grid())
}

// menuItems lists what can be done at a cell: first removing what is
// there, then adding to the cell and trying the game from it. The menu adds
// and removes only (increment 65): how a thing is set is in the inspector,
// once Select has selected it. Beyond the floor bounds, the floor can grow
// to the cell.
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
		items = append(items, dangerItem(menuRemoveNPC, "Remove NPC from map"))
	}
	_, anchored := floorData.AnchorAt(cell.X, cell.Y)
	if anchored {
		items = append(items, dangerItem(menuDeleteAnchor, "Delete anchor"))
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
	remove, add := s.markItems(floorData, cell)
	items = append(items, remove...)
	here = append(here, add...)
	here = append(here, s.gameItems(floorData, cell, live)...)
	if len(items) > 0 {
		here[0].Rule = true
	}

	return append(items, here...)
}

// gameItems are the menu's items for trying the game from a cell: Preview
// from here, Play from here, Move the player here while there is a game
// whose player can be moved, and the player start.
func (s *State) gameItems(floorData *floor.Floor, cell Cell, live bool) []ui.MenuItem {
	busy := live || s.choosing || s.starting
	standable := s.canStart(floorData, cell)
	items := []ui.MenuItem{
		menuItem(menuPreviewHere, "Preview from here", busy || !standable),
		menuItem(menuPlayHere, "Play from here", !standable),
	}
	if s.canMovePlayer() {
		items = append(items, menuItem(menuMoveHere, "Move the player here", !standable))
	}

	return append(items, s.startItem(standable, cell))
}

// chooseGame carries out one of the items for trying the game from a cell.
func (s *State) chooseGame(ctx *frame.Context, chosen string, cell Cell) {
	switch chosen {
	case menuPreviewHere:
		s.startPreviewAt(ctx, cell)
	case menuPlayHere:
		s.playFrom(ctx, cell)
	case menuMoveHere:
		s.movePlayer(ctx, cell)
	}
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
	case menuRemoveNPC:
		if placement, found := floorData.PlacementAt(cell.X, cell.Y); found {
			s.selectPlacement(placement.ID)
			s.removePlacement(ctx)
		}
	case menuDeleteAnchor:
		if anchor, found := floorData.AnchorAt(cell.X, cell.Y); found {
			s.selectAnchor(anchor.ID)
			s.deleteAnchor(ctx)
		}
	case menuAddAnchor:
		s.addAnchor(ctx, floorData, cell)
	case menuPlaceNPC:
		s.placeNPC(ctx, floorData, cell)
	case menuClearTop:
		s.stroke.count++
		gesture := s.window.String() + "-clear-" + strconv.Itoa(s.stroke.count)
		cleared := tilefloor.Cell{Col: cell.X, Row: cell.Y, Layer: tilefloor.LayerTop, Tile: ""}
		s.command(ctx, s.floor, tilefloor.PaintEdit, gesture, tilefloor.PaintArgs{Cells: []tilefloor.Cell{cleared}}, nil)
	case menuPreviewHere, menuPlayHere, menuMoveHere:
		s.chooseGame(ctx, chosen, cell)
	case menuSetStart:
		s.setStart(ctx, cell)
	case menuExtendFloor:
		if next, fits := extendTo(floorData.Bounds(), cell); fits {
			s.requestResize(ctx, floorData, next)
		}
	default:
		s.chooseMark(ctx, floorData, chosen, cell)
	}
}
