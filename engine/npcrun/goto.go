package npcrun

import (
	"fmt"
	"image"
	"math"

	pathfind "github.com/bradhannah/berad-sdk/modules/berad-path"

	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Route search costs: a straight step and a diagonal one, roughly in the
// ratio of their lengths, and how many cells a search may visit.
const (
	straightCost = 10
	diagonalCost = 14
	maxVisited   = 40000
)

// actGoTo walks a Go to's route to its anchor, one cell at a time, finding
// the route when the node starts and again if something has come into the
// way. It carries on, with a status, when there is no destination, it is on
// another floor, or there is no way there.
func (n *NPC) actGoTo(world World, node flow.Node) (string, bool) {
	if !n.work.started {
		if status := n.plan(world.Zone, node); status != "" {
			n.work.skipped = status

			return flow.OutputNext, true
		}
	}
	if len(n.work.route) == 0 {
		return flow.OutputNext, true
	}
	next := n.work.route[0]
	if n.step(world.Zone, next.Sub(n.Cell)) {
		n.work.route = n.work.route[1:]

		return "", false
	}
	if status := n.plan(world.Zone, node); status != "" {
		n.work.skipped = status

		return flow.OutputNext, true
	}

	return "", false
}

// plan sets up a Go to: finds its anchor on the NPC's floor and a route
// there. It returns why it cannot, or "".
func (n *NPC) plan(zone play.Zone, node flow.Node) string {
	destination, err := node.Anchor("destination")
	if err != nil || destination.IsZero() {
		return StatusNoAnchor
	}
	if destination.Floor != n.Floor {
		return StatusOtherFloor
	}
	status := StatusNoAnchor
	zone.Floor(n.Floor, func(floor *tilefloor.Floor) {
		anchor, _, found := floor.Anchor(destination.Anchor)
		if !found {
			return
		}
		goal := image.Pt(anchor.Col, anchor.Row)
		route, ok := findRoute(floor, n.Cell, goal)
		if !ok {
			status = StatusNoRoute

			return
		}
		n.work = routeWork(route, anchor.Name, goal)
		status = ""
	})

	return status
}

// findRoute finds a cheapest route on a floor, by the steps the player can
// take, and returns the cells after the start.
func findRoute(floor *tilefloor.Floor, from, goal image.Point) ([]image.Point, bool) {
	if from == goal {
		return nil, true
	}
	result, err := pathfind.Find(pathfind.Query{
		Start: from, Goal: goal, Bounds: floor.Bounds(), Moves: pathfind.Eight, MinStep: straightCost,
		MaxVisited: maxVisited,
		Cost: func(at, to image.Point) (int, bool) {
			delta := to.Sub(at)
			if !play.CanStep(floor, at, delta) {
				return 0, false
			}
			if delta.X != 0 && delta.Y != 0 {
				return diagonalCost, true
			}

			return straightCost, true
		},
	})
	if err != nil || result.Outcome != pathfind.Found {
		return nil, false
	}

	return result.Route[1:], true
}

// Activity says in a few words what the NPC is doing on its node, as the
// Watch window shows it.
func (n *NPC) Activity(workflow *flow.Workflow) string {
	if workflow == nil {
		return StatusNoBehaviour
	}
	node, ok := workflow.Node(n.Node)
	if !ok {
		return "Starting"
	}
	kind, _ := flow.KindOf(node.Kind)
	switch {
	case node.Kind == flow.KindWait && n.work.waiting:
		clockName := "game time"
		if n.work.real {
			clockName = "real time"
		}

		return fmt.Sprintf("Wait %s of %s (%s)", seconds(max(n.work.waitLeft, 0)), seconds(n.work.waitTotal), clockName)
	case node.Kind == flow.KindMove && n.work.started:
		tile := min(n.work.movesTotal-n.work.movesLeft, n.work.movesTotal)

		return fmt.Sprintf("Move %s, %d of %d %s", n.work.way, tile, n.work.movesTotal, plural(n.work.movesTotal))
	case node.Kind == flow.KindGoTo && n.work.started:
		left := len(n.work.route)

		return fmt.Sprintf("Walking to %s, %d %s to go", n.work.goal, left, plural(left))
	default:
		return kind.Label
	}
}

// seconds writes a number of seconds as "1.2 s", "12 min 5 s" or "1 h 30 min".
func seconds(value float64) string {
	const minute, hour, tenths = 60, 3600, 10
	value = math.Round(value*tenths) / tenths
	switch {
	case value < minute && value != math.Trunc(value):
		return fmt.Sprintf("%.1f s", value)
	case value < minute:
		return fmt.Sprintf("%d s", int(value))
	case value < hour:
		whole := int(value)
		if whole%minute == 0 {
			return fmt.Sprintf("%d min", whole/minute)
		}

		return fmt.Sprintf("%d min %d s", whole/minute, whole%minute)
	default:
		whole := int(value) / minute
		if whole%minute == 0 {
			return fmt.Sprintf("%d h", whole/minute)
		}

		return fmt.Sprintf("%d h %d min", whole/minute, whole%minute)
	}
}

func plural(count int) string {
	if count == 1 {
		return "tile"
	}

	return "tiles"
}
