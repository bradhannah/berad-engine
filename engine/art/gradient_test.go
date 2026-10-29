package art_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/engine/art"
)

func TestGradient(t *testing.T) {
	t.Parallel()
	left := art.Shade{Color: 0x2E2C33, Opacity: 100}
	right := art.Shade{Color: 0xE1E3E6, Opacity: 40}
	shades, err := art.Gradient(left, right, 5, true, false)
	if err != nil || len(shades) != 5 {
		t.Fatalf("gradient: %v %v", shades, err)
	}
	got := ""
	for _, shade := range shades {
		got += shade.Color.String() + " "
		if shade.Opacity != 100 || !shade.ID.IsZero() || shade.Label != "" {
			t.Fatalf("a colour-only gradient keeps the left opacity and has no ID or label: %+v", shade)
		}
	}
	if got != "#2E2C33 #56555B #828187 #B0B1B5 #E1E3E6 " {
		t.Fatalf("steps in OKLab, ends included: %s", got)
	}
	faded, err := art.Gradient(left, right, 4, false, true)
	if err != nil {
		t.Fatal(err)
	}
	for at, want := range []int{100, 80, 60, 40} {
		if faded[at].Opacity != want || faded[at].Color != left.Color {
			t.Fatalf("an opacity-only gradient, step %d: %+v", at+1, faded[at])
		}
	}
	for _, steps := range []int{1, art.MaxShades + 1} {
		if _, err := art.Gradient(left, right, steps, true, true); err == nil {
			t.Fatalf("a gradient of %d steps", steps)
		}
	}
	if _, err := art.Gradient(left, art.Shade{Color: 0, Opacity: 0}, 3, true, true); err == nil {
		t.Fatal("a gradient to 0% opacity")
	}
}
