package tileart_test

import (
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/views/tileart"
)

// TestGlassIsPremultipliedForDrawing checks the tile picture's colours are
// scaled by their alpha, as Ebitengine takes them, so a 40% blue over white
// shows a pale blue, not a brighter colour.
func TestGlassIsPremultipliedForDrawing(t *testing.T) {
	t.Parallel()
	got := tileart.Premultiply([]byte{0x30, 0x60, 0xc0, 102, 255, 128, 0, 255, 9, 9, 9, 0})
	want := []byte{19, 38, 76, 102, 255, 128, 0, 255, 0, 0, 0, 0}
	if !slices.Equal(got, want) {
		t.Fatalf("premultiplied %v, want %v", got, want)
	}
}
