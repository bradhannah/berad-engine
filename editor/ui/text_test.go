package ui_test

import (
	"strings"
	"testing"
)

// TestFitText keeps text that fits, and cuts text that does not, ending it
// with an ellipsis inside the width.
func TestFitText(t *testing.T) {
	t.Parallel()
	loaded := theme(t)
	short := "Ready"
	if got := loaded.FitText(short, 200); got != short {
		t.Fatalf("text that fits is kept: %q", got)
	}
	long := "Jumped to 09:00. NPCs start their workflows again."
	got := loaded.FitText(long, 120)
	if !strings.HasSuffix(got, "…") || loaded.TextWidth(got) > 120 || !strings.HasPrefix(long, strings.TrimSuffix(got, "…")) {
		t.Fatalf("cut to %q (%d px)", got, loaded.TextWidth(got))
	}
	if got := loaded.FitText(long, 2); got != "" {
		t.Fatalf("nothing fits: %q", got)
	}
}
