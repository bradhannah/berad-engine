package mapview_test

import (
	"image"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/engine/ids"
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

// TestFamilyOfManyWalls changes the family of more walls than one paint
// command may hold: they go in several commands, none over the limit, that
// share a gesture, so every wall changes and one undo takes them all back.
func TestFamilyOfManyWalls(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	const side = 70
	walled, err := tilefloor.NewDocument(side, side, "grass")
	if err != nil {
		t.Fatal(err)
	}
	for col := range side {
		for row := range side {
			if _, err := walled.Floor.SetTop(col, row, "wall.a"); err != nil {
				t.Fatal(err)
			}
		}
	}
	f.docs.Put(groundID, "Ground", walled)
	f.keys("S")
	f.click("cell:2,3")
	if got := len(f.state.Selected()); got != side*side {
		t.Fatalf("selected %d walls, want every one of %d", got, side*side)
	}
	f.keys("4")
	commands := f.requests.Of(tilefloor.PaintEdit)
	changed := 0
	for _, command := range commands {
		var args tilefloor.PaintArgs
		if err := command.Decode(&args); err != nil {
			t.Fatal(err)
		}
		if len(args.Cells) == 0 || len(args.Cells) > tilefloor.MaxCells {
			t.Fatalf("a command of %d cells, of at most %d", len(args.Cells), tilefloor.MaxCells)
		}
		if command.Command.Gesture == "" || command.Command.Gesture != commands[0].Command.Gesture {
			t.Fatalf("the commands share a gesture, to undo as one step: %q", command.Command.Gesture)
		}
		changed += len(args.Cells)
	}
	if changed != side*side {
		t.Fatalf("changed %d walls of %d", changed, side*side)
	}
	for _, command := range commands {
		command.Succeed(ids.ULID{})
	}
	if !strings.Contains(f.status(), "Changed 4900 walls") {
		t.Fatalf("the status counts every wall once all are changed: %q", f.status())
	}
}
