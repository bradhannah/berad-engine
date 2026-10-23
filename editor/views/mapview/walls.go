package mapview

import (
	"cmp"
	"image"
	"slices"

	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

// Cell is a floor position.
type Cell = image.Point

func isWallAt(floorData *floor.Floor, cell Cell) bool {
	top, err := floorData.Top(cell.X, cell.Y)

	return err == nil && tiles.IsWall(top)
}

func neighbours4(cell Cell) [4]Cell {
	return [4]Cell{
		image.Pt(cell.X, cell.Y-1), image.Pt(cell.X+1, cell.Y), image.Pt(cell.X, cell.Y+1), image.Pt(cell.X-1, cell.Y),
	}
}

// WallRun returns every wall cell connected to start through side-by-side
// walls of any family: one structural run.
func WallRun(floorData *floor.Floor, start Cell) []Cell {
	if !isWallAt(floorData, start) {
		return nil
	}
	seen := map[Cell]bool{start: true}
	queue := []Cell{start}
	for len(queue) > 0 {
		cell := queue[0]
		queue = queue[1:]
		for _, next := range neighbours4(cell) {
			if !seen[next] && isWallAt(floorData, next) {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}

	return sortedCells(seen)
}

// RoomWalls returns the walls around the closed room containing start: the
// space reachable from start without crossing a wall or a door. It returns
// false when that space reaches the floor's edge, so it is not closed.
func RoomWalls(floorData *floor.Floor, start Cell) ([]Cell, bool) {
	if !floorData.In(start.X, start.Y) || isWallAt(floorData, start) {
		return nil, false
	}
	inside := map[Cell]bool{start: true}
	queue := []Cell{start}
	for len(queue) > 0 {
		cell := queue[0]
		queue = queue[1:]
		for _, next := range neighbours4(cell) {
			if !floorData.In(next.X, next.Y) {
				return nil, false
			}
			top, _ := floorData.Top(next.X, next.Y) //nolint:errcheck // In bounds, checked above.
			if inside[next] || tiles.Joins(top) {
				continue
			}
			inside[next] = true
			queue = append(queue, next)
		}
	}
	walls := map[Cell]bool{}
	for cell := range inside {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				next := image.Pt(cell.X+dx, cell.Y+dy)
				if isWallAt(floorData, next) {
					walls[next] = true
				}
			}
		}
	}

	return sortedCells(walls), true
}

// WallsIn returns the wall cells inside a rectangle of cells, inclusive.
func WallsIn(floorData *floor.Floor, from, to Cell) []Cell {
	walls := map[Cell]bool{}
	for row := min(from.Y, to.Y); row <= max(from.Y, to.Y); row++ {
		for col := min(from.X, to.X); col <= max(from.X, to.X); col++ {
			if isWallAt(floorData, image.Pt(col, row)) {
				walls[image.Pt(col, row)] = true
			}
		}
	}

	return sortedCells(walls)
}

// LineCells returns the cells on a straight line between two cells, so a fast
// drag paints every cell it passes over.
func LineCells(start, end Cell) []Cell {
	cells := []Cell{}
	spanX, spanY := abs(end.X-start.X), -abs(end.Y-start.Y)
	stepX, stepY := sign(end.X-start.X), sign(end.Y-start.Y)
	err := spanX + spanY
	cell := start
	for {
		cells = append(cells, cell)
		if cell == end {
			return cells
		}
		doubled := err + err
		if doubled >= spanY {
			err += spanY
			cell.X += stepX
		}
		if doubled <= spanX {
			err += spanX
			cell.Y += stepY
		}
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}

func sortedCells(set map[Cell]bool) []Cell {
	cells := make([]Cell, 0, len(set))
	for cell := range set {
		cells = append(cells, cell)
	}
	slices.SortFunc(cells, func(a, b Cell) int { return cmp.Or(cmp.Compare(a.Y, b.Y), cmp.Compare(a.X, b.X)) })

	return cells
}
