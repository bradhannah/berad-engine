package mapview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/views/mapview"
)

func TestWallAlgorithms(t *testing.T) {
	t.Parallel()
	fl := groundFloor(t).Floor
	if run := mapview.WallRun(fl, image.Pt(2, 2)); len(run) != 17 {
		t.Fatalf("run of %d walls", len(run))
	}
	if run := mapview.WallRun(fl, image.Pt(0, 0)); run != nil {
		t.Fatal("grass is not a wall run")
	}
	walls, closed := mapview.RoomWalls(fl, image.Pt(3, 3))
	if !closed || len(walls) != 17 {
		t.Fatalf("room walls %d %v", len(walls), closed)
	}
	if _, closed := mapview.RoomWalls(fl, image.Pt(9, 9)); closed {
		t.Fatal("the outside reaches the floor's edge")
	}
	if got := mapview.WallsIn(fl, image.Pt(5, 6), image.Pt(0, 0)); len(got) != 10 {
		t.Fatalf("walls in a rectangle drawn backwards: %d", len(got))
	}
	line := mapview.LineCells(image.Pt(0, 0), image.Pt(4, 2))
	if len(line) != 5 || line[0] != image.Pt(0, 0) || line[4] != image.Pt(4, 2) {
		t.Fatalf("line %v", line)
	}
	for index := 1; index < len(line); index++ {
		step := line[index].Sub(line[index-1])
		if step.X < -1 || step.X > 1 || step.Y < -1 || step.Y > 1 {
			t.Fatalf("line skips a cell: %v", line)
		}
	}
}
