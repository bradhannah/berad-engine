package game

import (
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
)

// FadeTicks is how long a portal that fades takes to fade the screen to
// black, and as long again to fade it in where the portal leads: a third of
// a second each way.
const FadeTicks = 20

// passage is the player on their way through a portal (increment 64).
//
// A step that lands on a portal starts the player leaving. The game cannot
// take them where the portal leads by itself: that may be another zone,
// whose documents whoever runs the game must gather first. So the game
// holds the player on the portal's cell, lets the step finish so they are
// seen to stand on it, then fades the screen when the portal fades, and
// waits to be told: Enter for another zone, Place for a cell of this one,
// or Stay to call it off. The world carries on meanwhile.
type passage struct {
	// leaving is the portal the player is leaving by, nil when they are not.
	leaving *tilefloor.Portal
	// landed says the step onto the portal has finished: the player stands
	// on its cell. Nothing changes on the screen before it has.
	landed bool
	// out counts the ticks since the player landed.
	out int
	// in counts down the ticks left of fading in after a fade.
	in int
}

// Leaving returns the portal the player has stepped onto and is waiting to
// be taken through, if any.
func (g *Game) Leaving() (tilefloor.Portal, bool) {
	if g.passage.leaving == nil {
		var none tilefloor.Portal

		return none, false
	}

	return *g.passage.leaving, true
}

// Dark reports whether the player can be taken where their portal leads
// now: once they stand on its cell for a portal that cuts, and once the
// screen has then faded to black for one that fades.
func (g *Game) Dark() bool {
	leaving := g.passage.leaving

	return leaving != nil && g.passage.landed &&
		(leaving.Change != tilefloor.ChangeFade || g.passage.out >= FadeTicks)
}

// Darkness is how far the screen is faded to black, from 0, not at all, to
// 1, black: rising once the player stands on a portal that fades, and
// falling once they arrive.
func (g *Game) Darkness() float64 {
	switch leaving := g.passage.leaving; {
	case leaving != nil && leaving.Change == tilefloor.ChangeFade:
		return float64(min(g.passage.out, FadeTicks)) / FadeTicks
	case leaving != nil:
		return 0
	}

	return float64(g.passage.in) / FadeTicks
}

// Stay calls off a leaving: the player stays on the portal's cell and can
// walk again, as when a portal leads nowhere, or when a Map's preview, which
// stays in its zone, meets one. A screen that had begun to fade fades back
// in. The portal is not set off again until the player steps onto it anew.
func (g *Game) Stay() {
	if g.passage.leaving == nil {
		return
	}
	g.passage.in = 0
	if g.passage.leaving.Change == tilefloor.ChangeFade {
		g.passage.in = min(g.passage.out, FadeTicks)
	}
	g.passage.leaving, g.passage.landed, g.passage.out = nil, false, 0
}

// arrived ends a leaving once the player has been put where the portal
// leads: a screen that faded to black fades in.
func (g *Game) arrived() {
	if g.passage.leaving == nil {
		return
	}
	g.passage.in = 0
	if g.passage.leaving.Change == tilefloor.ChangeFade {
		g.passage.in = FadeTicks
	}
	g.passage.leaving, g.passage.landed, g.passage.out = nil, false, 0
}

// waiting reports whether the keys wait: while the player leaves by a
// portal, and while the screen fades in where it led.
func (g *Game) waiting() bool { return g.passage.leaving != nil || g.passage.in > 0 }

// pass counts a tick of the player's passage, and starts one when the tick's
// step took them onto a portal. The step is left to finish before anything
// is counted, so the player is seen to walk onto the portal and stand there
// before the screen changes. Only a step sets a portal off: stairs that take
// the player to another floor win over a portal on their cell, and arriving
// on a portal's cell, by a portal or by being moved, does nothing, so the
// way back can stand where the player arrives.
func (g *Game) pass(before play.Player, outcome play.Outcome) {
	if g.passage.leaving != nil {
		switch {
		case g.passage.landed:
			g.passage.out++
		case !g.run.Player().Moving():
			g.passage.landed = true
		}

		return
	}
	if g.passage.in > 0 {
		g.passage.in--
	}
	after := g.run.Player()
	if outcome != play.Walking || after.Floor != before.Floor || after.Cell == before.Cell {
		return
	}
	g.levels.Floor(after.Floor, func(floor *tilefloor.Floor) {
		if portal, found := floor.PortalAt(after.Cell.X, after.Cell.Y); found {
			g.passage.leaving, g.passage.landed, g.passage.out = &portal, false, 0
		}
	})
}
