package browser

import (
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Preview (increment 32): one thing large, the rest in a strip beneath, which
// the arrow keys walk: a sprite, a palette or a template as a picture, a
// zone's floors as a map with a tab for each, a workflow drawn from where its
// nodes are, an NPC as its sprite, and a sound as its waveform.

// Sizes of Preview, in pixels.
const (
	stripHeight = 140
	stripTile   = 72
	previewPad  = 16
	factsWidth  = 240
	floorTabW   = 120
	floorTabTop = 8
	// floorTabPrefix starts a floor tab's button, as "floor-tab-0".
	floorTabPrefix = "floor-tab-"
)

// previewState is what Preview shows: where its large picture and its
// facts are, the floor tabs of a zone previewed, and which floor each zone
// shows.
type previewState struct {
	big, facts image.Rectangle
	floorTabs  []*ui.Button
	floorShown map[ids.ULID]int
}

func newPreviewState() *previewState {
	return &previewState{big: ui.Zero(), facts: ui.Zero(), floorTabs: nil, floorShown: map[ids.ULID]int{}}
}

// previewTile is the size of a tile in Preview's strip.
func previewTile() image.Point { return image.Pt(stripTile+gridPad, stripTile+pictureLabels) }

// previewed returns the row Preview shows large: the selected thing, or
// the strip's first.
func (s *State) previewed() (ui.Row, bool) {
	for _, cell := range s.grid.cells {
		if cell.row.ID == s.list.Selected {
			return cell.row, true
		}
	}
	if len(s.grid.cells) > 0 {
		return s.grid.cells[0].row, true
	}

	return ui.NewRow("", ""), false
}

// layoutPreview places the strip along the bottom of the main pane, the
// large picture over it, its facts beside it, and a zone's floor tabs.
func (s *State) layoutPreview(docs frame.Docs, area image.Rectangle) {
	strip := image.Rect(area.Min.X, area.Max.Y-stripHeight, area.Max.X, area.Max.Y)
	s.grid.layoutStrip(s.list.Rows, strip, previewTile())
	top := area.Min.Y + previewPad
	s.preview.facts = image.Rect(area.Max.X-factsWidth-previewPad, top, area.Max.X-previewPad, strip.Min.Y-previewPad)
	s.preview.big = image.Rect(area.Min.X+previewPad, top, s.preview.facts.Min.X-previewPad, strip.Min.Y-previewPad)
	s.preview.floorTabs = nil
	row, shown := s.previewed()
	zoneID, err := ids.Parse(row.ID)
	if !shown || err != nil {
		return
	}
	index := s.index(docs)
	if entry, found := index.Entry(zoneID); !found || entry.Format != zone.FormatName {
		return
	}
	floors := s.floorsOf(docs, zoneID)
	for position, floorID := range floors {
		button := ui.NewButton(floorTabPrefix+strconv.Itoa(position), index.Name(floorID, "a floor"))
		left := s.preview.big.Min.X + position*(floorTabW+ui.Gap)
		button.Rect = image.Rect(left, top, left+floorTabW, top+ui.ButtonH)
		button.On = position == s.preview.floorShown[zoneID]
		s.preview.floorTabs = append(s.preview.floorTabs, button)
	}
	if len(floors) > 0 {
		s.preview.big.Min.Y += ui.ButtonH + floorTabTop
	}
}

// updatePreview handles Preview: the strip as tiles, and a zone's floor
// tabs, each showing its floor.
func (s *State) updatePreview(input *ui.Input, ctx *frame.Context) {
	row, shown := s.previewed()
	for position, button := range s.preview.floorTabs {
		if zoneID, err := ids.Parse(row.ID); shown && err == nil && button.Update(input) {
			s.preview.floorShown[zoneID] = position
		}
	}
	s.updateGrid(input, ctx)
}

// stepPreview moves Preview along its strip, as the arrow keys do.
func (s *State) stepPreview(delta int) {
	row, shown := s.previewed()
	if !shown {
		return
	}
	for position, cell := range s.grid.cells {
		if cell.row.ID == row.ID {
			next := min(max(position+delta, 0), len(s.grid.cells)-1)
			s.list.Selected = s.grid.cells[next].row.ID
			s.grid.reveal(s.list.Selected)
			s.focus.Set(nil)

			return
		}
	}
}

// drawPreview draws the strip, the thing previewed large with its floor
// tabs, and its facts.
func (s *State) drawPreview(screen *ebiten.Image, docs frame.Docs) {
	index := s.index(docs)
	ui.Divider(screen, image.Pt(s.grid.rect.Min.X, s.grid.rect.Min.Y), image.Pt(s.grid.rect.Max.X, s.grid.rect.Min.Y))
	if area, ok := screen.SubImage(s.grid.rect).(*ebiten.Image); ok {
		for _, cell := range s.grid.cells {
			rect := s.grid.onScreen(cell)
			if !rect.Overlaps(s.grid.rect) {
				continue
			}
			s.drawTileFrame(area, cell.row, rect)
			box := image.Rect(rect.Min.X+gridPad/ui.Half, rect.Min.Y+pictureTop, rect.Min.X+gridPad/ui.Half+stripTile,
				rect.Min.Y+pictureTop+stripTile)
			s.drawThing(area, docs, index, cell.row, box)
			s.drawTileText(area, cell.row, image.Rect(rect.Min.X, box.Max.Y, rect.Max.X, rect.Max.Y))
		}
	}
	row, shown := s.previewed()
	if !shown || s.preview.big.Empty() {
		return
	}
	for _, button := range s.preview.floorTabs {
		button.Draw(screen, s.theme)
	}
	s.drawThing(screen, docs, index, row, s.preview.big)
	s.drawFacts(screen, docs, index, row)
}

// drawThing draws what any thing looks like in a box: art as its picture, a
// zone as the floor its tab shows, a floor as its map, an NPC as its sprite,
// a workflow drawn small, and a sound as its waveform.
func (s *State) drawThing(
	screen *ebiten.Image, docs frame.Docs, index *project.Index, row ui.Row, box image.Rectangle,
) {
	ui.FillRound(screen, box, pictureRadius, ui.ColorPage())
	if cue, isCue := s.cueOf(row.ID); isCue {
		recording, heard := heardRecording(cue)
		s.drawWave(screen, box.Inset(ui.Inset), recording, heard)

		return
	}
	documentID, err := ids.Parse(row.ID)
	entry, found := index.Entry(documentID)
	switch {
	case err != nil || !found:
		s.drawPicture(screen, docs, index, row, box)
	case entry.Format == zone.FormatName:
		floors := s.floorsOf(docs, entry.ID)
		if shown := s.preview.floorShown[entry.ID]; shown < len(floors) {
			if picture, ok := s.floorMap(docs, floors[shown]); ok {
				drawScaled(screen, picture, box)
			}
		}
	case entry.Format == floor.FormatName:
		if picture, ok := s.floorMap(docs, entry.ID); ok {
			drawScaled(screen, picture, box)
		}
	case entry.Format == npc.FormatName:
		s.drawPerson(screen, docs, entry.ID, box)
	case entry.Format == flow.FormatName:
		docs.View(entry.ID, func(document client.Document) {
			if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
				drawMiniature(screen, workflowDoc.Workflow, box.Inset(ui.Inset))
			}
		})
	default:
		s.drawPicture(screen, docs, index, row, box)
	}
}

// drawFacts draws what is known of the thing previewed: its name, where it
// lives and what is said of it.
func (s *State) drawFacts(screen *ebiten.Image, docs frame.Docs, index *project.Index, row ui.Row) {
	facts := s.preview.facts
	width := facts.Dx()
	top := facts.Min.Y
	s.theme.DrawStrong(screen, s.theme.FitText(row.Label, width), facts.Min.X, top, ui.ColorInk())
	lines := []string{row.Detail}
	if documentID, err := ids.Parse(row.ID); err == nil {
		if entry, found := index.Entry(documentID); found {
			lines = append([]string{s.wherePath(docs, index, entry)}, lines...)
		}
	}
	if cue, isCue := s.cueOf(row.ID); isCue {
		if recording, heard := heardRecording(cue); heard {
			said, _, _, _ := s.loudnessOf(recording)
			lines = append(lines, lengthText(recording.LengthMs), said)
		}
	}
	for _, line := range lines {
		if line == "" {
			continue
		}
		top += cardLine
		s.theme.DrawText(screen, s.theme.FitText(line, width), facts.Min.X, top, ui.ColorSubdued())
	}
}

// drawPerson draws an NPC as its sprite, or its icon when it shows none.
func (s *State) drawPerson(screen *ebiten.Image, docs frame.Docs, npcID ids.ULID, box image.Rectangle) {
	if person, loaded := s.npcOf(docs, npcID); loaded && !person.Sprite.IsZero() {
		if picture, ok := s.spritePicture(docs, person.Sprite); ok {
			drawScaled(screen, picture, box)

			return
		}
	}
	ui.DrawIcon(screen, ui.IconNPC, ui.Center(box).Sub(image.Pt(ui.IconSize/ui.Half, ui.IconSize/ui.Half)))
}
