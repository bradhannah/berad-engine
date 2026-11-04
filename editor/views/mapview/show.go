package mapview

import (
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/showdraw"
	"github.com/bradhannah/berad-engine/engine/art"
)

// Locating what the preview shows: a speaker's bubble and the box.
const (
	speechPart = "speech"
	boxPart    = "box"
)

// sameBox reports whether two boxes are the same, as sent.
func sameBox(sent, now *protocol.PreviewBox) bool {
	if sent == nil || now == nil {
		return sent == now
	}

	return *sent == *now
}

// shownState is the preview's state the map draws from: this window's, or
// another window's in this zone.
func (s *State) shownState() *protocol.PreviewState {
	switch {
	case s.preview != nil:
		state := s.previewState()

		return &state
	case s.remote != nil:
		return s.remote.state
	}

	return nil
}

// shakeBy is how far the world is moved now, in the map's pixels.
func (s *State) shakeBy(state *protocol.PreviewState) image.Point {
	if state == nil {
		return image.Pt(0, 0)
	}
	scale := float64(s.camera.cellSize()) / art.DefaultTileSize

	return image.Pt(int(math.Round(state.ShakeX*scale)), int(math.Round(state.ShakeY*scale)))
}

// speakers lists the NPCs saying a line on the shown floor whose place is in
// view: a speaker off screen still speaks, unseen.
func (s *State) speakers(grid image.Rectangle) []protocol.PreviewEntity {
	var shown []protocol.PreviewEntity
	for _, entity := range s.liveNPCs() {
		if entity.Saying != "" && entity.Floor == s.floor && s.entityRect(grid, entity.X, entity.Y).Overlaps(grid) {
			shown = append(shown, entity)
		}
	}

	return shown
}

// headOf is the top middle of an NPC's cell, where its bubble's tail
// points.
func (s *State) headOf(grid image.Rectangle, entity protocol.PreviewEntity) image.Point {
	cell := s.entityRect(grid, entity.X, entity.Y)

	return image.Pt((cell.Min.X+cell.Max.X)/ui.Half, cell.Min.Y)
}

// drawSpeech draws each speaker's line in a bubble over its head.
func (s *State) drawSpeech(area *ebiten.Image, grid image.Rectangle) {
	for _, entity := range s.speakers(grid) {
		showdraw.DrawBubble(area, s.theme, s.headOf(grid, entity), entity.Saying, entity.SayingLeft)
	}
}

// drawBox draws the box shown now at the foot of the map.
func (s *State) drawBox(area *ebiten.Image, grid image.Rectangle, box *protocol.PreviewBox) {
	showdraw.DrawBox(area, s.theme, grid, box)
}

// closeBox closes a box that waits for the player when they click the map,
// reporting whether it did.
func (s *State) closeBox(input *ui.Input) bool {
	if s.preview == nil || !s.preview.stage.Holding() || !input.Pressed || !input.Cursor.In(s.Grid()) {
		return false
	}
	s.preview.stage.Close()
	input.ClearPointer()

	return true
}

// locateShown finds a speaker's bubble, "speech:<NPC>", or the box, "box".
func (s *State) locateShown(kind, value string) (image.Point, bool) {
	state, grid := s.shownState(), s.Grid()
	switch kind {
	case speechPart:
		for _, entity := range s.speakers(grid) {
			if s.names.Name(entity.NPC, "") == value {
				rect, _ := showdraw.BubbleRect(s.theme, s.headOf(grid, entity), entity.Saying)

				return ui.Center(rect), true
			}
		}
	case boxPart:
		if state != nil && state.Box != nil {
			rect, _ := showdraw.BoxRect(s.theme, grid, state.Box)

			return ui.Center(rect), true
		}
	}

	return image.Pt(0, 0), false
}

// hasShownState answers "speaking:<NPC>=<line>", the line an NPC is saying,
// drawn or not; "box=<text>", the box shown now; "boxes=<count>", how many
// are queued; and "shaking".
func (s *State) hasShownState(state string) (bool, bool) {
	shown := s.shownState()
	switch {
	case strings.HasPrefix(state, "speaking:"):
		name, line, _ := strings.Cut(strings.TrimPrefix(state, "speaking:"), "=")
		for _, entity := range s.liveNPCs() {
			if s.names.Name(entity.NPC, "") == name {
				return entity.Saying == line, true
			}
		}

		return false, true
	case strings.HasPrefix(state, "box="):
		return shown != nil && shown.Box != nil && shown.Box.Text == strings.TrimPrefix(state, "box="), true
	case strings.HasPrefix(state, "boxes="):
		queued := 0
		if shown != nil && shown.Box != nil {
			queued = shown.Box.Queued
		}

		return strconv.Itoa(queued) == strings.TrimPrefix(state, "boxes="), true
	case state == "shaking":
		return shown != nil && (shown.ShakeX != 0 || shown.ShakeY != 0), true
	}

	return false, false
}
