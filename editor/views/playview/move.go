package playview

import (
	"fmt"
	"image"
	"strings"

	"github.com/bradhannah/berad-engine/editor/views/docsource"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/game"
)

// Move the player here (increment 63): a Map's cell menu moves the player
// of the game running here to a cell, into another zone if need be. The game
// goes on with its values and its clock; a zone left stops, and the zone
// entered starts its NPCs at their placements.

// noGameToMove is what the tab says when asked to move a player with no
// game running: it does not launch one, which nobody asked for.
const noGameToMove = "No game is running to move the player in. Press Cmd+R to launch."

// askMove takes a request to move the player. With no game running there
// is no player to move: the tab says so and launches nothing.
func (s *State) askMove(ctx *frame.Context, start game.Start) {
	if s.game == nil {
		s.launching, s.moving = false, nil
		if s.problem == "" {
			s.problem = noGameToMove
		}
		ctx.Frame.Refuse(noGameToMove)

		return
	}
	s.moving = &start
}

// move carries out the move that waits: at once within the zone being
// played, and into another zone once what that zone needs has loaded, the
// game going on where it is until then.
func (s *State) move(ctx *frame.Context) {
	if s.game == nil {
		s.moving = nil

		return
	}
	target := *s.moving
	if target.Zone == s.game.Zone() {
		s.moving = nil
		if err := s.game.Place(target.Floor, image.Pt(target.Col, target.Row)); err != nil {
			ctx.Frame.Refuse(sentence(err.Error()))

			return
		}
		ctx.Frame.Status(fmt.Sprintf("Moved the player to cell %d, %d. The game goes on.", target.Col, target.Row))

		return
	}
	loaded, ready := s.prepare(ctx, &target)
	if !ready {
		return
	}
	s.moving = nil
	if loaded.Project.Zone == nil {
		ctx.Frame.Refuse("The player cannot be moved there: that zone is not in the project.")

		return
	}
	if err := s.game.Enter(loaded.Project); err != nil {
		ctx.Frame.Refuse(sentence(err.Error()))

		return
	}
	docs := docsource.New(ctx.Docs)
	s.floors = loaded.Project.Zone.Floors()
	s.screen.Start(loaded, docs)
	if len(loaded.Cues) > 0 {
		ctx.Sounds.Prepare(loaded.Cues)
	}
	ctx.Frame.Status(fmt.Sprintf(
		"Moved the player to cell %d, %d of %s. The zone left has stopped; the values and the clock go on.",
		target.Col, target.Row, zoneName(docs.Name(target.Zone))))
}

// zoneName is a zone's name, or a word for one with none.
func zoneName(name string) string {
	if name == "" {
		return "that zone"
	}

	return name
}

// sentence starts a message with a capital and ends it with a full stop.
func sentence(message string) string {
	if message == "" {
		return message
	}

	return strings.ToUpper(message[:1]) + message[1:] + "."
}
