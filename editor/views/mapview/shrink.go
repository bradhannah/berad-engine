package mapview

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

// A floor that shrinks lists all it gives up (increment 58): besides its
// anchors and their uses, its NPCs, its ambient sounds, its stairs, and the
// stairs on the floors above and below that lead into the cells it gives
// up. A shrink that would leave the player start off the floor is refused
// until the start is moved.

// shrinking is what a resize gives up, besides anchors.
type shrinking struct {
	npcs    []string
	sounds  []string
	stairs  []image.Point
	leading []string
}

// empty reports whether the resize gives up none of it.
func (g shrinking) empty() bool {
	return len(g.npcs) == 0 && len(g.sounds) == 0 && len(g.stairs) == 0 && len(g.leading) == 0
}

// startOff reports whether the player start is on this floor and would be
// off it at next.
func (s *State) startOff(next image.Rectangle) bool {
	start := s.playerStart

	return !start.IsZero() && start.Floor == s.floor && !image.Pt(start.Col, start.Row).In(next)
}

// refuseStartOff refuses a resize that would leave the player start off the
// floor, saying where it is.
func (s *State) refuseStartOff(ctx *frame.Context) {
	ctx.Frame.Refuse(fmt.Sprintf("The player start, %d,%d, would be off the floor. Move it first, then resize.",
		s.playerStart.Col, s.playerStart.Row))
}

// goesHere lists what a resize to next gives up on the floor itself. It is
// read while the floor is, so it names NPCs from the Map's copy of the
// project index rather than reading the index too.
func (s *State) goesHere(floorData *floor.Floor, next image.Rectangle) shrinking {
	var going shrinking
	for _, placement := range floorData.Placements() {
		if !image.Pt(placement.Col, placement.Row).In(next) {
			going.npcs = append(going.npcs, s.names.Name(placement.NPC, "an NPC"))
		}
	}
	for _, sound := range floorData.Sounds() {
		if !image.Pt(sound.Col, sound.Row).In(next) {
			going.sounds = append(going.sounds, s.cueName(sound.Cue))
		}
	}
	bounds := floorData.Bounds()
	eachCell(bounds, func(cell image.Point) {
		if top, err := floorData.Top(cell.X, cell.Y); err == nil && !cell.In(next) && tiles.Climb(top) != 0 {
			going.stairs = append(going.stairs, cell)
		}
	})

	return going
}

// leadingInto lists the stairs on the floors above and below that lead
// into the cells a resize from bounds to next gives up.
func (s *State) leadingInto(docs frame.Docs, bounds, next image.Rectangle) []string {
	var leading []string
	at := indexOf(s.floors, s.floor)
	for _, beside := range []struct {
		index, climb int
		way          string
	}{{at - 1, 1, "up"}, {at + 1, -1, "down"}} {
		if at < 0 || beside.index < 0 || beside.index >= len(s.floors) {
			continue
		}
		name := s.names.Name(s.floors[beside.index], "another floor")
		docs.View(s.floors[beside.index], func(document client.Document) {
			other, ok := document.Model.(*tilefloor.Document)
			if !ok {
				return
			}
			eachCell(other.Floor.Bounds(), func(cell image.Point) {
				top, err := other.Floor.Top(cell.X, cell.Y)
				if err == nil && cell.In(bounds) && !cell.In(next) && tiles.Climb(top) == beside.climb {
					leading = append(leading, fmt.Sprintf("the stairs on %s at %d,%d, which lead %s into them",
						name, cell.X, cell.Y, beside.way))
				}
			})
		})
	}

	return leading
}

// resizeAsk is a resize asked for while the floor was read, finished once it
// is not: what it gives up on the floor, and its anchors.
type resizeAsk struct {
	resize pendingResize
	from   image.Rectangle
	going  shrinking
	lost   []floor.Anchor
}

// said is what the resize question says of what goes, besides anchors.
func (g shrinking) said() []string {
	var lines []string
	if len(g.npcs) > 0 {
		lines = append(lines, fmt.Sprintf("This takes %d %s off the map: %s.",
			len(g.npcs), plural(len(g.npcs), "NPC", "NPCs"), strings.Join(g.npcs, ", ")))
	}
	if len(g.sounds) > 0 {
		lines = append(lines, fmt.Sprintf("This removes the ambient %s: %s.",
			plural(len(g.sounds), "sound", "sounds"), strings.Join(g.sounds, ", ")))
	}
	if len(g.stairs) > 0 {
		cells := make([]string, 0, len(g.stairs))
		for _, cell := range g.stairs {
			cells = append(cells, strconv.Itoa(cell.X)+","+strconv.Itoa(cell.Y))
		}
		lines = append(lines, "This removes the stairs at "+strings.Join(cells, "; ")+".")
	}
	for _, leading := range g.leading {
		lines = append(lines, "It leaves "+leading+", leading nowhere.")
	}

	return lines
}

// eachCell calls visit for every cell of bounds.
func eachCell(bounds image.Rectangle, visit func(image.Point)) {
	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for col := bounds.Min.X; col < bounds.Max.X; col++ {
			visit(image.Pt(col, row))
		}
	}
}
