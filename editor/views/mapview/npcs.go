package mapview

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// NPC tokens: a dark disc most of a cell across, with the NPC's initials.
const (
	tokenRadius    = 0.4
	minTokenRadius = 4
	tokenRing      = 2.5
	npcRows        = 8
	initialsWanted = 2
	workflowView   = "workflow"
	hexToken       = 0x1d2433
	hexBroken      = 0x9aa1ad
	// placementButtonsTop is where the placement inspector puts its buttons,
	// below the NPC and its behaviour.
	placementButtonsTop = ui.Inset + 10*ui.LineHeight
)

type placementDrag struct {
	id   ids.ULID
	from Cell
}

// npcRows lists the project's NPCs, for the Place NPC tool.
func (s *State) npcRows() []ui.Row {
	entries := s.names.OfFormat(npc.FormatName)
	rows := make([]ui.Row, 0, len(entries))
	for number, entry := range entries {
		detail := ""
		if number < numberedTiles {
			detail = strconv.Itoa(number + 1)
		}
		rows = append(rows, ui.Row{
			ID: entry.ID.String(), Label: entry.Name, Detail: detail, Indent: 0, Swatch: nil, Heading: false, Marked: false,
		})
	}

	return rows
}

// refreshNPCs copies the project index, and finds the selected placement's
// NPC and its behaviour, before anything looks inside the floor: a window's
// documents cannot be viewed from inside another view.
func (s *State) refreshNPCs(docs frame.Docs) {
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			s.names = project.NewIndex(index.Entries())
		}
	})
	s.placedNPC, s.behaviour = ids.ULID{}, ids.ULID{}
	if s.placement.IsZero() {
		return
	}
	docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			if placement, _, found := floorDoc.Floor.Placement(s.placement); found {
				s.placedNPC = placement.NPC
			}
		}
	})
	docs.View(s.placedNPC, func(document client.Document) {
		if npcDoc, ok := document.Model.(*npcplugin.Document); ok {
			s.behaviour = npcDoc.NPC.Behaviour
		}
	})
}

// subscribeNPC keeps the selected placement's NPC loaded, so its behaviour
// can be shown.
func (s *State) subscribeNPC(ctx *frame.Context) {
	if _, exists := s.names.Entry(s.placedNPC); exists {
		s.subscribe(s.placedNPC, ctx)
	}
}

// initials returns up to two letters or digits for an NPC's token: the first
// of each of its first two words.
func initials(name string) string {
	var letters []rune
	for _, word := range strings.Fields(name) {
		for _, letter := range word {
			if unicode.IsLetter(letter) || unicode.IsDigit(letter) {
				letters = append(letters, unicode.ToUpper(letter))

				break
			}
		}
		if len(letters) == initialsWanted {
			break
		}
	}

	return string(letters)
}

// updateNPCTool places the chosen NPC on an empty cell, selects a placement
// that is clicked, and moves one that is dragged.
func (s *State) updateNPCTool(input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool) {
	if input.Pressed && onFloor {
		if existing, found := floorData.PlacementAt(cell.X, cell.Y); found {
			s.selectPlacement(existing.ID)
			s.placing = &placementDrag{id: existing.ID, from: cell}

			return
		}
		s.placeNPC(ctx, floorData, cell)
	}
	if s.placing != nil && !input.Down {
		drag := s.placing
		s.placing = nil
		if !onFloor || cell == drag.from {
			return
		}
		if _, taken := floorData.PlacementAt(cell.X, cell.Y); taken {
			ctx.Frame.Status("Another NPC is already there")

			return
		}
		if moved, _, found := floorData.Placement(drag.id); found {
			moved.Col, moved.Row = cell.X, cell.Y
			update := tilefloor.UpdatePlacementArgs{Placement: moved}
			s.command(ctx, s.floor, tilefloor.UpdatePlacementEdit, "", update, nil)
		}
	}
}

// placeNPC puts the NPC chosen on the left on a cell.
func (s *State) placeNPC(ctx *frame.Context, floorData *floor.Floor, cell Cell) {
	npcID, err := ids.Parse(s.tile[ToolNPC])
	if err != nil {
		ctx.Frame.Status("Choose an NPC on the left first. New NPC in the project browser makes one.")

		return
	}
	name := s.names.Name(npcID, "the NPC")
	args := tilefloor.AddPlacementArgs{
		Placement: floor.Placement{ID: ids.ULID{}, NPC: npcID, Col: cell.X, Row: cell.Y},
		Index:     len(floorData.Placements()),
	}
	s.command(ctx, s.floor, tilefloor.AddPlacementEdit, "", args, func(result protocol.Result) {
		s.selectPlacement(result.Created)
		ctx.Frame.Status(fmt.Sprintf("Placed %s at %d, %d", name, cell.X, cell.Y))
	})
}

func (s *State) selectPlacement(placementID ids.ULID) {
	s.placement = placementID
	s.anchor, s.selected = ids.ULID{}, nil
	s.focus.Set(nil)
}

// syncPlacement drops the selection when the placement is gone.
func (s *State) syncPlacement(floorData *floor.Floor) {
	if s.placement.IsZero() {
		return
	}
	if _, _, ok := floorData.Placement(s.placement); !ok {
		s.placement = ids.ULID{}
	}
}

// removePlacement takes the selected NPC off the map; undo puts it back.
func (s *State) removePlacement(ctx *frame.Context) {
	removal := tilefloor.RemovePlacementArgs{ID: s.placement}
	s.command(ctx, s.floor, tilefloor.RemovePlacementEdit, "", removal, func(protocol.Result) {
		ctx.Frame.Status("NPC removed from the map. Undo brings it back.")
	})
	s.placement = ids.ULID{}
}

// openBehaviour opens the selected NPC's behaviour in a Workflow window.
func (s *State) openBehaviour(ctx *frame.Context) {
	if s.behaviour.IsZero() {
		ctx.Frame.Status("This NPC has no behaviour yet. Choose one in the project browser.")

		return
	}
	ctx.Requests.Open(workflowView, s.behaviour, false)
}

// drawPlacements draws each placed NPC as a dark token with its initials,
// and its name beside it when there is room. The selected NPC has a blue
// ring; an NPC whose definition was deleted is grey with a question mark.
func (s *State) drawPlacements(area *ebiten.Image, floorData *floor.Floor, grid image.Rectangle) {
	size := s.camera.cellSize()
	for _, placement := range floorData.Placements() {
		rect := s.camera.cellRect(Cell{X: placement.Col, Y: placement.Row}, grid)
		if !rect.Overlaps(grid) {
			continue
		}
		centreX, centreY := float32(rect.Min.X)+float32(size)/ui.Half, float32(rect.Min.Y)+float32(size)/ui.Half
		radius := max(float32(size)*tokenRadius, minTokenRadius)
		name := s.names.Name(placement.NPC, "")
		label, fill := initials(name), ui.Hex(hexToken)
		if name == "" {
			name, label, fill = "Deleted NPC", "?", ui.Hex(hexBroken)
		}
		vector.FillCircle(area, centreX, centreY, radius, fill, true)
		if placement.ID == s.placement {
			vector.StrokeCircle(area, centreX, centreY, radius+tokenRing, tokenRing, ui.ColorSelect(), true)
		}
		if size < minLabel {
			continue
		}
		width := s.theme.SmallWidth(label)
		s.theme.DrawSmall(area, label, int(centreX)-width/ui.Half, int(centreY)-ui.LabelSize/ui.Half-ui.Half,
			ui.ColorBackground())
		labelX, labelY := rect.Max.X+ui.Half, rect.Min.Y+(size-ui.LabelSize)/ui.Half-ui.Half
		backing := image.Rect(labelX-1, labelY, labelX+s.theme.SmallWidth(name)+ui.Half, labelY+ui.LabelSize+ui.Gap+ui.Half)
		ui.FillRect(area, backing, ui.HexAlpha(hexWhite, labelBacking))
		s.theme.DrawSmall(area, name, labelX, labelY, ui.ColorInk())
	}
}

// drawPlacementInspector describes the selected NPC: its name, its
// behaviour and where it stands.
func (s *State) drawPlacementInspector(screen *ebiten.Image, floorData *floor.Floor, textX int, textY *int) {
	placement, _, ok := floorData.Placement(s.placement)
	if !ok {
		return
	}
	s.theme.Heading(screen, "NPC", textX, *textY)
	*textY += ui.LineHeight
	name := s.names.Name(placement.NPC, "")
	if name == "" {
		s.theme.DrawText(screen, "Deleted NPC: its definition is gone", textX, *textY, ui.ColorWarn())
	} else {
		s.theme.DrawStrong(screen, name, textX, *textY, ui.ColorInk())
	}
	*textY += ui.LineHeight
	s.theme.DrawText(screen, fmt.Sprintf("at %d, %d", placement.Col, placement.Row), textX, *textY, ui.ColorInk())
	*textY += ui.LineHeight + ui.LineHeight/ui.Half
	s.theme.Heading(screen, "BEHAVIOUR", textX, *textY)
	*textY += ui.LineHeight
	has := !s.behaviour.IsZero()
	switch {
	case has:
		s.theme.DrawText(screen, s.names.Name(s.behaviour, "Broken: its workflow was deleted"), textX, *textY,
			ui.ColorInk())
	default:
		s.theme.DrawText(screen, "None yet. Choose one in the project browser.", textX, *textY, ui.ColorMuted())
	}
	*textY = placementButtonsTop
	s.openBn.Disabled = !has
	s.openBn.Draw(screen, s.theme)
	s.removeBn.Draw(screen, s.theme)
	*textY += ui.ButtonH + ui.LineHeight
	s.theme.DrawText(screen, "Drag the token to move it.", textX, *textY, ui.ColorMuted())
	*textY += ui.LineHeight
}

// locateNPC finds an NPC by name in the Place NPC tool's list.
func (s *State) locateNPC(name string) (image.Point, bool) {
	for _, row := range s.tileRows() {
		if row.Label == name {
			return s.tileList.Locate(row.ID)
		}
	}

	return image.Pt(0, 0), false
}

// hasPlacement reports whether an NPC with the given name stands on a cell
// ("C,R=Name") of the current floor, for input scripts.
func (s *State) hasPlacement(value string, docs frame.Docs) bool {
	where, name, _ := strings.Cut(value, "=")
	var col, row int
	if _, err := fmt.Sscanf(where, "%d,%d", &col, &row); err != nil {
		return false
	}
	found := false
	docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			if placement, placed := floorDoc.Floor.PlacementAt(col, row); placed {
				found = name == "" || s.names.Name(placement.NPC, "") == name
			}
		}
	})

	return found
}
