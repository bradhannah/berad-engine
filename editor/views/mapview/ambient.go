package mapview

import (
	"cmp"
	"fmt"
	"image"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// Ambient sounds are ambient cues placed on cells (increment 14), heard in a
// game within their cue's distance, louder closer. The right-click menu
// places one, changes its cue, moves it or removes it; the Map draws each as
// a marker with a faint ring at its distance.

const (
	menuPlaceSound  = "place-sound"
	menuChangeSound = "change-sound"
	menuMoveSound   = "move-sound"
	menuRemoveSound = "remove-sound"
	// soundCueItem prefixes a cue's ID in the menu that chooses one.
	soundCueItem = "sound-cue:"
	hexSound     = 0x7a3fb0
	soundRadius  = 0.22
	ringAlpha    = 90
)

// ambientSounds is the Map's part for ambient sounds: the audio library as
// last listed, and the sound being moved or given another cue.
type ambientSounds struct {
	library  *protocol.AudioLibrary
	asking   bool
	moving   ids.ULID
	changing ids.ULID
}

// askAudio asks for the project's audio library, unless a question is out.
func (s *State) askAudio(ctx *frame.Context) {
	if s.ambient.asking {
		return
	}
	s.ambient.asking = true
	ctx.Requests.AudioLibrary(func(library protocol.AudioLibrary, err error) {
		s.ambient.asking = false
		if err == nil {
			s.ambient.library = &library
		}
	})
}

// cueTicks is how many ticks a cue plays for, from the library as last
// listed; none until it is.
func (s *State) cueTicks(cue string) int {
	if s.ambient.library == nil {
		return 0
	}

	return s.ambient.library.CueTicks(cue)
}

// ambientCues returns the library's ambient cues by name.
func (s *State) ambientCues() []protocol.AudioCue {
	if s.ambient.library == nil {
		return nil
	}
	var cues []protocol.AudioCue
	for _, cue := range s.ambient.library.Cues {
		if cue.Category == protocol.CategoryAmbient {
			cues = append(cues, cue)
		}
	}
	slices.SortFunc(cues, func(a, b protocol.AudioCue) int { return cmp.Compare(a.Name, b.Name) })

	return cues
}

// cueOf returns a cue from the library as last listed.
func (s *State) cueOf(cueID string) (protocol.AudioCue, bool) {
	if s.ambient.library != nil {
		for _, cue := range s.ambient.library.Cues {
			if cue.ID == cueID {
				return cue, true
			}
		}
	}

	return protocol.AudioCue{}, false //nolint:exhaustruct_v5 // No cue.
}

// cueName names a cue by its name in the library, or its ID.
func (s *State) cueName(cueID string) string {
	if cue, ok := s.cueOf(cueID); ok && cue.Name != "" {
		return cue.Name
	}

	return cueID
}

// soundItems are the cell menu's choices for an ambient sound on the cell,
// or for placing one.
func (s *State) soundItems(floorData *floor.Floor, cell Cell) []ui.MenuItem {
	if sound, found := floorData.SoundAt(cell.X, cell.Y); found {
		name := "“" + s.cueName(sound.Cue) + "”"

		return []ui.MenuItem{
			menuItem(menuChangeSound, "Change ambient sound "+name+"…", len(s.ambientCues()) == 0),
			menuItem(menuMoveSound, "Move ambient sound", false),
			dangerItem(menuRemoveSound, "Remove ambient sound "+name),
		}
	}

	return []ui.MenuItem{menuItem(menuPlaceSound, "Place ambient sound here…", len(s.ambientCues()) == 0)}
}

// chooseSound carries out an ambient sound choice from the cell menu.
func (s *State) chooseSound(ctx *frame.Context, floorData *floor.Floor, chosen string, cell Cell) {
	sound, found := floorData.SoundAt(cell.X, cell.Y)
	switch {
	case chosen == menuPlaceSound:
		s.ambient.changing = ids.ULID{}
		s.openCueMenu(cell, "")
	case chosen == menuChangeSound && found:
		s.ambient.changing = sound.ID
		s.openCueMenu(cell, sound.Cue)
	case chosen == menuMoveSound && found:
		s.ambient.moving = sound.ID
		ctx.Frame.Status("Click the cell to move the ambient sound to. Escape cancels.")
	case chosen == menuRemoveSound && found:
		s.command(ctx, s.floor, tilefloor.RemoveSoundEdit, "", tilefloor.RemoveSoundArgs{ID: sound.ID},
			func(protocol.Result) { ctx.Frame.Status("Removed the ambient sound") })
	case strings.HasPrefix(chosen, soundCueItem):
		s.placeSound(ctx, floorData, cell, strings.TrimPrefix(chosen, soundCueItem))
	}
}

// openCueMenu offers the library's ambient cues for a cell, the current one
// ticked off.
func (s *State) openCueMenu(cell Cell, current string) {
	cues := s.ambientCues()
	items := make([]ui.MenuItem, 0, len(cues))
	for _, cue := range cues {
		items = append(items, menuItem(soundCueItem+cue.ID, cue.Name, cue.ID == current))
	}
	at := s.camera.cellRect(cell, s.Grid()).Max
	s.menu = &cellMenu{Menu: ui.Menu{Items: items, Width: ui.MenuWidth(items, s.theme)}, cell: cell}
	s.menu.OpenAt(at, s.Grid())
}

// placeSound places an ambient cue on a cell, or gives the sound being
// changed that cue.
func (s *State) placeSound(ctx *frame.Context, floorData *floor.Floor, cell Cell, cueID string) {
	name := s.cueName(cueID)
	if sound, _, found := floorData.Sound(s.ambient.changing); found {
		sound.Cue = cueID
		s.command(ctx, s.floor, tilefloor.UpdateSoundEdit, "", tilefloor.UpdateSoundArgs{Sound: sound},
			func(protocol.Result) { ctx.Frame.Status("The ambient sound is now “" + name + "”") })

		return
	}
	sound := floor.Sound{ID: ids.ULID{}, Cue: cueID, Col: cell.X, Row: cell.Y}
	args := tilefloor.AddSoundArgs{Sound: sound, Index: len(floorData.Sounds())}
	s.command(ctx, s.floor, tilefloor.AddSoundEdit, "", args,
		func(protocol.Result) {
			ctx.Frame.Status(fmt.Sprintf("Placed “%s” at %d,%d. It is heard %s.", name, cell.X, cell.Y,
				s.heardWithin(cueID)))
		})
}

// heardWithin says how far a cue is heard.
func (s *State) heardWithin(cueID string) string {
	cue, ok := s.cueOf(cueID)
	if !ok || cue.Ambient == nil || cue.Ambient.RadiusTiles <= 0 {
		return "everywhere on the floor"
	}

	return fmt.Sprintf("within %g cells", cue.Ambient.RadiusTiles)
}

// updateMovingSound moves the sound being moved to the clicked cell, or
// stops on Escape.
func (s *State) updateMovingSound(
	input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool,
) {
	if input.Consume(ui.KeyEscape, ui.Plain()) {
		s.ambient.moving = ids.ULID{}
		ctx.Frame.Status("Move cancelled")

		return
	}
	if !input.Pressed || !onFloor {
		return
	}
	sound, _, found := floorData.Sound(s.ambient.moving)
	s.ambient.moving = ids.ULID{}
	if !found {
		return
	}
	if _, taken := floorData.SoundAt(cell.X, cell.Y); taken {
		ctx.Frame.Refuse("Another ambient sound is already there")

		return
	}
	sound.Col, sound.Row = cell.X, cell.Y
	s.command(ctx, s.floor, tilefloor.UpdateSoundEdit, "", tilefloor.UpdateSoundArgs{Sound: sound},
		func(protocol.Result) {
			ctx.Frame.Status(fmt.Sprintf("Moved the ambient sound to %d,%d", cell.X, cell.Y))
		})
}

// drawSounds draws each ambient sound on the floor: a purple marker with its
// cue's name beside it when there is room, and a faint ring at the distance
// it is heard within.
func (s *State) drawSounds(area *ebiten.Image, floorData *floor.Floor, grid image.Rectangle) {
	size := float32(s.camera.cellSize())
	purple := ui.Hex(hexSound)
	for _, sound := range floorData.Sounds() {
		rect := s.camera.cellRect(Cell{X: sound.Col, Y: sound.Row}, grid)
		centreX, centreY := float32(rect.Min.X)+size/ui.Half, float32(rect.Min.Y)+size/ui.Half
		if cue, ok := s.cueOf(sound.Cue); ok && cue.Ambient != nil && cue.Ambient.RadiusTiles > 0 {
			ring := float32(cue.Ambient.RadiusTiles) * size
			vector.StrokeCircle(area, centreX, centreY, ring, ui.Hairline, ui.HexAlpha(hexSound, ringAlpha), true)
		}
		if !rect.Overlaps(grid) {
			continue
		}
		radius := max(size*soundRadius, minAnchorRadius)
		vector.FillCircle(area, centreX, centreY, radius, purple, true)
		vector.StrokeCircle(area, centreX, centreY, radius+tokenRing, tokenRing, ui.ColorReverse(), true)
		if rect.Dx() < minLabel {
			continue
		}
		name := s.cueName(sound.Cue)
		labelX, labelY := rect.Max.X+ui.Half, rect.Max.Y-ui.LabelSize-ui.Gap
		backing := image.Rect(labelX-1, labelY, labelX+s.theme.SmallWidth(name)+ui.Half, labelY+ui.LabelSize+ui.Gap+ui.Half)
		ui.FillRect(area, backing, ui.HexAlpha(hexWhite, labelBacking))
		s.theme.DrawSmall(area, name, labelX, labelY, purple)
	}
}

// hasSound answers "ambient:C,R:<cue>" for input scripts: an ambient sound of
// that cue is on that cell of the shown floor.
func (s *State) hasSound(value string, docs frame.Docs) bool {
	where, cueID, _ := strings.Cut(value, ":")
	var col, row int
	if _, err := fmt.Sscanf(where, "%d,%d", &col, &row); err != nil {
		return false
	}
	found := false
	docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			sound, placed := floorDoc.Floor.SoundAt(col, row)
			found = placed && sound.Cue == cueID
		}
	})

	return found
}
