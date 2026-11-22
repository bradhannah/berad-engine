package playview

import (
	"fmt"
	"image"

	"github.com/bradhannah/berad-engine/editor/views/docsource"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// travel takes a player who is leaving by a portal where it leads
// (increment 64), as a game's own window does: to a cell of the zone being
// played, or into another zone. The tab's documents arrive while it works,
// so it asks for what the portal's zone needs as soon as the player steps
// on, and the player waits on the portal, the screen staying black for a
// portal that fades, until all of it has arrived. A portal that leads
// nowhere, or to a cell nothing can stand on, leaves the player where they
// are, and the status bar says why.
func (s *State) travel(ctx *frame.Context) {
	portal, leaving := s.game.Leaving()
	if !leaving {
		return
	}
	asking := docsource.New(ctx.Docs).Asking(func(documentID ids.ULID) { s.subscribe(documentID, ctx) })
	start, state, why := gameload.Destination(asking, portal.To)
	switch {
	case state == gameload.Waiting:
		return
	case state == gameload.Missing:
		s.stay(ctx, portal, why)

		return
	case start.Zone == s.game.Zone():
		if !s.game.Dark() {
			return
		}
		if err := s.game.Place(start.Floor, image.Pt(start.Col, start.Row)); err != nil {
			s.stay(ctx, portal, err.Error())
		}

		return
	}
	loaded, ready := s.prepare(ctx, &start)
	if !ready || !s.game.Dark() {
		return
	}
	if loaded.Project.Zone == nil {
		s.stay(ctx, portal, gameload.NoZone)

		return
	}
	if err := s.enter(ctx, loaded); err != nil {
		s.stay(ctx, portal, err.Error())

		return
	}
	ctx.Frame.Status(fmt.Sprintf("The portal led to %s. The zone left has stopped; the values and the clock go on.",
		zoneName(s.zoneNamed)))
}

// stay leaves the player on a portal that cannot take them, and says why.
func (s *State) stay(ctx *frame.Context, portal floor.Portal, why string) {
	s.game.Stay()
	ctx.Frame.Refuse(fmt.Sprintf("The portal at %d,%d cannot take the player: %s.", portal.Col, portal.Row, why))
}
