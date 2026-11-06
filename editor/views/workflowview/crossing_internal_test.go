package workflowview

import (
	"image"
	"slices"
	"testing"
)

func TestCrossingFindsWhereARouteRunsInside(t *testing.T) {
	t.Parallel()
	rect := image.Rect(100, 50, 200, 100)
	route := []image.Point{{0, 70}, {300, 70}, {300, 200}}
	want := [][2]image.Point{{{100, 70}, {200, 70}}}
	if got := crossing(route, rect); !slices.Equal(got, want) {
		t.Fatalf("across the rectangle: %v, want %v", got, want)
	}
	down := []image.Point{{150, 0}, {150, 300}}
	if got := crossing(down, rect); !slices.Equal(got, [][2]image.Point{{{150, 50}, {150, 100}}}) {
		t.Fatalf("down through the rectangle: %v", got)
	}
	if got := crossing([]image.Point{{0, 20}, {300, 20}, {300, 40}}, rect); len(got) != 0 {
		t.Fatalf("a route above the rectangle crosses nothing: %v", got)
	}
}
