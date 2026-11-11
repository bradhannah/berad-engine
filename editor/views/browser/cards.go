package browser

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	tilefloorplugin "github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Cards (increment 32): each thing drawn small with what matters about it.
// An NPC shows its sprite, what it follows and where it lives; a zone, its
// ground floor as a small map, its floors, people and loop; a floor, its
// map; and a workflow is drawn small from where its nodes are, with how it
// runs and its problems.

// Sizes of a card, in pixels.
const (
	cardWidth   = 240
	cardHeight  = 96
	cardPicture = 72
	cardPad     = 12
	cardLine    = 20
	miniNodeW   = 10
	miniNodeH   = 6
	miniStroke  = 1
)

// cardTile is the size of a tile in Cards.
func cardTile() image.Point { return image.Pt(cardWidth, cardHeight) }

// cardText is a line of a card, and whether it warns.
type cardText struct {
	text string
	warn bool
}

// minimap is a floor drawn small, a pixel a cell, kept until it changes.
type minimap struct {
	revision uint64
	image    *ebiten.Image
}

// floorMap returns a floor drawn a pixel a cell, from its tiles' colours.
func (s *State) floorMap(docs frame.Docs, floorID ids.ULID) (*ebiten.Image, bool) {
	docs.View(floorID, func(document client.Document) {
		floorDoc, ok := document.Model.(*tilefloorplugin.Document)
		if cached := s.maps[floorID]; !ok || cached != nil && cached.revision == document.Revision {
			return
		}
		bounds := floorDoc.Floor.Bounds()
		drawn := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
			for col := bounds.Min.X; col < bounds.Max.X; col++ {
				drawn.SetRGBA(col-bounds.Min.X, row-bounds.Min.Y, ui.Hex(tiles.ColorOf(cellTile(floorDoc.Floor, col, row))))
			}
		}
		if cached := s.maps[floorID]; cached != nil {
			cached.image.Deallocate()
		}
		picture := ebiten.NewImage(bounds.Dx(), bounds.Dy())
		picture.WritePixels(drawn.Pix)
		s.maps[floorID] = &minimap{revision: document.Revision, image: picture}
	})
	cached := s.maps[floorID]
	if cached == nil {
		return nil, false
	}

	return cached.image, true
}

// cellTile returns the tile a floor's map shows at a cell: its top, or its
// ground where it has none.
func cellTile(level *floor.Floor, col, row int) string {
	if top, err := level.Top(col, row); err == nil && top != "" {
		return top
	}
	if ground, err := level.Ground(col, row); err == nil {
		return ground
	}

	return ""
}

// syncCards asks for the floors the cards draw: each zone's ground floor,
// and each floor shown.
func (s *State) syncCards(ctx *frame.Context) {
	if s.viewShown() != viewCards && s.viewShown() != viewPreview {
		return
	}
	index := s.index(ctx.Docs)
	for _, cell := range s.grid.cells {
		documentID, err := ids.Parse(cell.row.ID)
		if err != nil {
			continue
		}
		entry, found := index.Entry(documentID)
		switch {
		case !found:
		case entry.Format == zone.FormatName:
			for _, floorID := range s.floorsOf(ctx.Docs, documentID) {
				s.subscribe(floorID, ctx)
			}
		case entry.Format == floor.FormatName:
			s.subscribe(documentID, ctx)
		}
	}
}

// drawCards draws Cards' tiles and its groups' headings.
func (s *State) drawCards(screen *ebiten.Image, docs frame.Docs) {
	area, ok := screen.SubImage(s.grid.rect).(*ebiten.Image)
	if !ok {
		return
	}
	index := s.index(docs)
	for _, cell := range s.grid.cells {
		rect := s.grid.onScreen(cell)
		if !rect.Overlaps(s.grid.rect) {
			continue
		}
		if cell.row.Heading {
			s.theme.DrawLabel(area, cell.row.Label, rect.Min.X, rect.Min.Y+ui.Gap, ui.ColorMuted())

			continue
		}
		s.drawCard(area, docs, index, cell.row, rect)
	}
}

// drawCard draws a card: its picture at the left, its name and what
// matters about it beside.
func (s *State) drawCard(
	screen *ebiten.Image, docs frame.Docs, index *project.Index, row ui.Row, rect image.Rectangle,
) {
	ui.FillRound(screen, rect, ui.Radius, ui.ColorField())
	ui.StrokeRound(screen, rect, ui.Radius, ui.Outline, ui.ColorRule())
	s.drawTileFrame(screen, row, rect)
	box := image.Rect(rect.Min.X+cardPad, rect.Min.Y+cardPad, rect.Min.X+cardPad+cardPicture,
		rect.Min.Y+cardPad+cardPicture)
	ui.FillRound(screen, box, pictureRadius, ui.ColorPage())
	lines := s.drawCardPicture(screen, docs, index, row, box)
	left, width := box.Max.X+cardPad, rect.Max.X-box.Max.X-ui.Half*cardPad
	s.theme.DrawStrong(screen, s.theme.FitText(row.Label, width), left, rect.Min.Y+cardPad, ui.ColorInk())
	for position, line := range lines {
		ink := ui.ColorSubdued()
		if line.warn {
			ink = ui.ColorWarn()
		}
		s.theme.DrawText(screen, s.theme.FitText(line.text, width), left, rect.Min.Y+cardPad+(position+1)*cardLine, ink)
	}
}

// drawCardPicture draws what a card shows of a thing in its box, and
// returns the lines it says about it.
func (s *State) drawCardPicture(
	screen *ebiten.Image, docs frame.Docs, index *project.Index, row ui.Row, box image.Rectangle,
) []cardText {
	documentID, err := ids.Parse(row.ID)
	entry, found := index.Entry(documentID)
	if err != nil || !found {
		ui.DrawIcon(screen, row.Icon, ui.Center(box).Sub(image.Pt(ui.IconSize/ui.Half, ui.IconSize/ui.Half)))

		return []cardText{{text: row.Detail, warn: false}}
	}
	switch entry.Format {
	case npc.FormatName:
		return s.npcCard(screen, docs, index, entry, box)
	case zone.FormatName:
		return s.zoneCard(screen, docs, index, entry, box)
	case floor.FormatName:
		if picture, ok := s.floorMap(docs, entry.ID); ok {
			drawScaled(screen, picture, box)
		}

		return []cardText{{text: "Floor of " + s.zoneName(docs, index, entry.ID), warn: false}}
	case flow.FormatName:
		return s.workflowCard(screen, docs, entry, box)
	}
	ui.DrawIcon(screen, row.Icon, ui.Center(box).Sub(image.Pt(ui.IconSize/ui.Half, ui.IconSize/ui.Half)))

	return []cardText{{text: row.Detail, warn: false}}
}

// zoneName names the zone holding a floor.
func (s *State) zoneName(docs frame.Docs, index *project.Index, floorID ids.ULID) string {
	if zoneID, inZone := s.zoneOf(docs, floorID); inZone {
		return index.Name(zoneID, "a zone")
	}

	return "no zone"
}

// npcCard shows an NPC's sprite, what it follows and where it lives.
func (s *State) npcCard(
	screen *ebiten.Image, docs frame.Docs, index *project.Index, entry project.Entry, box image.Rectangle,
) []cardText {
	person, loaded := s.npcOf(docs, entry.ID)
	if picture, ok := s.spritePicture(docs, person.Sprite); loaded && !person.Sprite.IsZero() && ok {
		drawScaled(screen, picture, box)
	} else {
		ui.DrawIcon(screen, ui.IconNPC, ui.Center(box).Sub(image.Pt(ui.IconSize/ui.Half, ui.IconSize/ui.Half)))
	}
	follows := cardText{text: "follows nothing", warn: true}
	if loaded && !person.Behaviour.IsZero() {
		follows = cardText{text: "follows " + index.Name(person.Behaviour, "a deleted workflow"), warn: false}
	}
	lives := cardText{text: "lives nowhere yet", warn: false}
	if loaded && !person.Zone.IsZero() {
		lives.text = "lives in " + index.Name(person.Zone, "a deleted zone")
	}

	return []cardText{follows, lives}
}

// zoneCard shows a zone's ground floor small, its floors and people, and
// its loop.
func (s *State) zoneCard(
	screen *ebiten.Image, docs frame.Docs, index *project.Index, entry project.Entry, box image.Rectangle,
) []cardText {
	floors := s.floorsOf(docs, entry.ID)
	if len(floors) > 0 {
		if picture, ok := s.floorMap(docs, floors[0]); ok {
			drawScaled(screen, picture, box)
		}
	}
	people := 0
	for _, person := range index.OfFormat(npc.FormatName) {
		if found, loaded := s.npcOf(docs, person.ID); loaded && found.Zone == entry.ID {
			people++
		}
	}
	loop := cardText{text: "no zone loop", warn: false}
	if loopID := s.loopOf(docs, entry.ID); !loopID.IsZero() {
		loop.text = "runs " + index.Name(loopID, "a deleted workflow")
	}

	return []cardText{{text: strconv.Itoa(len(floors)) + " " + plural(len(floors), "floor", "floors") + " · " +
		strconv.Itoa(people) + " " + plural(people, "person", "people"), warn: false}, loop}
}

// workflowCard draws a workflow small and says how it runs and what is
// wrong with it.
func (s *State) workflowCard(
	screen *ebiten.Image, docs frame.Docs, entry project.Entry, box image.Rectangle,
) []cardText {
	docs.View(entry.ID, func(document client.Document) {
		if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
			drawMiniature(screen, workflowDoc.Workflow, box.Inset(ui.Gap))
		}
	})
	purpose := flow.Purpose(entry.Kind)
	if !purpose.Valid() {
		purpose = flow.PurposeBehaviour
	}
	lines := []cardText{{text: purpose.Label(), warn: false}}
	if check, checked := s.checks[entry.ID]; checked {
		if families := check.Families(); len(families) > 0 {
			lines = append(lines, cardText{text: strings.Join(families, ", "), warn: true})
		} else if used := check.UsedBy(); used != "" {
			lines = append(lines, cardText{text: "used by " + used, warn: false})
		}
	}

	return lines
}

// drawMiniature draws a workflow's nodes where they are, joined by
// straight lines, scaled to fit a box and centred in it.
func drawMiniature(screen *ebiten.Image, workflow *flow.Workflow, box image.Rectangle) {
	boxes := workflow.Boxes()
	if len(boxes) == 0 {
		return
	}
	var bounds image.Rectangle
	for _, placed := range boxes {
		bounds = bounds.Union(image.Rect(placed.X, placed.Y, placed.X+1, placed.Y+1))
	}
	scale := min(float32(box.Dx()-miniNodeW)/float32(max(bounds.Dx(), 1)),
		float32(box.Dy()-miniNodeH)/float32(max(bounds.Dy(), 1)), 1)
	offsetX := (float32(box.Dx()) - float32(bounds.Dx())*scale - miniNodeW) / ui.Half
	offsetY := (float32(box.Dy()) - float32(bounds.Dy())*scale - miniNodeH) / ui.Half
	place := func(placed flow.Box) (float32, float32) {
		return float32(box.Min.X) + offsetX + float32(placed.X-bounds.Min.X)*scale,
			float32(box.Min.Y) + offsetY + float32(placed.Y-bounds.Min.Y)*scale
	}
	ink := ui.ColorWorkflow()
	for _, node := range workflow.Nodes() {
		from, known := boxes[node.ID]
		if !known {
			continue
		}
		fromX, fromY := place(from)
		for _, next := range node.Next {
			if to, ok := boxes[next]; ok {
				toX, toY := place(to)
				vector.StrokeLine(screen, fromX+miniNodeW/ui.Half, fromY+miniNodeH/ui.Half, toX+miniNodeW/ui.Half,
					toY+miniNodeH/ui.Half, miniStroke, ui.ColorSubdued(), true)
			}
		}
	}
	for _, placed := range boxes {
		left, top := place(placed)
		vector.FillRect(screen, left, top, miniNodeW, miniNodeH, ink, true)
	}
}
