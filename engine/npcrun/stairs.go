package npcrun

import (
	"image"
	"slices"

	pathfind "github.com/bradhannah/berad-sdk/modules/berad-path"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

// Climb is a flight of stairs on a Go to's way: the floor it leaves, the
// cell of the stairs, and the floor it leads to.
type Climb struct {
	From ids.ULID
	At   image.Point
	To   ids.ULID
}

// Way returns the stairs a Go to still has to take, in order, none when
// its place is on the floor the NPC is on.
func (n *NPC) Way() []Climb { return slices.Clone(n.work.climbs) }

// stairsOn are a floor's stairs that lead somewhere: each cell and the
// index of the floor it leads to, where the same cell can be stood on.
type stairsOn struct {
	cell  image.Point
	floor int
}

// stairsOf lists the stairs of each floor of a zone, by its index.
func stairsOf(zone play.Zone, floors []ids.ULID) [][]stairsOn {
	all := make([][]stairsOn, len(floors))
	for index, floorID := range floors {
		zone.Floor(floorID, func(floor *tilefloor.Floor) {
			bounds := floor.Bounds()
			for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
				for col := bounds.Min.X; col < bounds.Max.X; col++ {
					top, _ := floor.Top(col, row) //nolint:errcheck // Inside the floor.
					target := index + tiles.Climb(top)
					cell := image.Pt(col, row)
					if target != index && target >= 0 && target < len(floors) && play.CanStand(zone, floors[target], cell) {
						all[index] = append(all[index], stairsOn{cell: cell, floor: target})
					}
				}
			}
		})
	}

	return all
}

// hop is a place a route across floors can stand: a floor, by index, and a
// cell; the cost of getting there; and the way there, the cells walked and
// the stairs taken.
type hop struct {
	floor  int
	cell   image.Point
	cost   int
	cells  []image.Point
	climbs []Climb
}

// findWay finds a cheapest route from a cell of one floor of a zone to a
// cell of another, or the same, walking each floor by the steps the player
// can take and changing floor where stairs lead, as the player does. It
// never steps onto stairs it does not mean to take, as stepping onto them
// takes the stairs.
func findWay(zone play.Zone, floors []ids.ULID, from hop, goal hop) (hop, bool) {
	stairs := stairsOf(zone, floors)
	best := map[[3]int]int{}
	waiting := []hop{from}
	var found hop
	reached := false
	for len(waiting) > 0 {
		slices.SortStableFunc(waiting, func(a, b hop) int { return a.cost - b.cost })
		here := waiting[0]
		waiting = waiting[1:]
		if reached && here.cost >= found.cost {
			break
		}
		key := [3]int{here.floor, here.cell.X, here.cell.Y}
		if cost, seen := best[key]; seen && cost <= here.cost {
			continue
		}
		best[key] = here.cost
		if here.floor == goal.floor {
			if route, cost, ok := leg(zone, floors[here.floor], here.cell, goal.cell, stairs[here.floor]); ok &&
				(!reached || here.cost+cost < found.cost) {
				found, reached = extend(here, goal.floor, goal.cell, route, cost, nil), true
			}
		}
		waiting = append(waiting, climbsFrom(zone, floors, here, stairs)...)
	}

	return found, reached
}

// climbsFrom are the hops from a place to the far end of each flight of
// stairs on its floor that it can walk to.
func climbsFrom(zone play.Zone, floors []ids.ULID, here hop, stairs [][]stairsOn) []hop {
	var next []hop
	for _, flight := range stairs[here.floor] {
		if flight.cell == here.cell {
			continue
		}
		route, cost, ok := leg(zone, floors[here.floor], here.cell, flight.cell, stairs[here.floor])
		if !ok {
			continue
		}
		taken := &Climb{From: floors[here.floor], At: flight.cell, To: floors[flight.floor]}
		next = append(next, extend(here, flight.floor, flight.cell, route, cost, taken))
	}

	return next
}

// extend is a hop on from another by a route on its floor, ending on a
// floor and cell, with the stairs it takes at its end, if any.
func extend(at hop, floor int, cell image.Point, route []image.Point, cost int, taken *Climb) hop {
	made := hop{
		floor: floor, cell: cell, cost: at.cost + cost, cells: append(slices.Clone(at.cells), route...),
		climbs: slices.Clone(at.climbs),
	}
	if taken != nil {
		made.climbs = append(made.climbs, *taken)
	}

	return made
}

// leg finds a cheapest route on one floor that steps onto no stairs but its
// goal, and returns the cells after the start and its cost.
func leg(zone play.Zone, floorID ids.ULID, from, goal image.Point, stairs []stairsOn) ([]image.Point, int, bool) {
	if from == goal {
		return nil, 0, true
	}
	var route []image.Point
	cost, found := 0, false
	zone.Floor(floorID, func(floor *tilefloor.Floor) {
		result, err := pathfind.Find(pathfind.Query{
			Start: from, Goal: goal, Bounds: floor.Bounds(), Moves: pathfind.Eight, MinStep: straightCost,
			MaxVisited: maxVisited,
			Cost: func(at, to image.Point) (int, bool) {
				delta := to.Sub(at)
				if !play.CanStep(floor, at, delta) || (to != goal && onStairs(stairs, to)) {
					return 0, false
				}
				if delta.X != 0 && delta.Y != 0 {
					return diagonalCost, true
				}

				return straightCost, true
			},
		})
		if err == nil && result.Outcome == pathfind.Found {
			route, cost, found = result.Route[1:], result.Cost, true
		}
	})

	return route, cost, found
}

// onStairs reports whether a cell holds stairs that lead somewhere.
func onStairs(stairs []stairsOn, cell image.Point) bool {
	return slices.ContainsFunc(stairs, func(flight stairsOn) bool { return flight.cell == cell })
}

// climbAt is where stairs on a cell of the NPC's floor lead, by the index
// of the floor in the zone, or -1 for none, the zone lacking that floor or
// the same cell there not standable.
func climbAt(zone play.Zone, floors []ids.ULID, floorID ids.ULID, cell image.Point) int {
	index := slices.Index(floors, floorID)
	target := -1
	zone.Floor(floorID, func(floor *tilefloor.Floor) {
		if !floor.In(cell.X, cell.Y) {
			return
		}
		top, _ := floor.Top(cell.X, cell.Y) //nolint:errcheck // Inside the floor.
		if climb := tiles.Climb(top); climb != 0 && index >= 0 {
			target = index + climb
		}
	})
	if target < 0 || target >= len(floors) || !play.CanStand(zone, floors[target], cell) {
		return -1
	}

	return target
}
