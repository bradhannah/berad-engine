package window

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// TestTheTextScaleIsChosenAndKept asks for a larger scale in one pane: every
// pane's Text button names it, the status bar says so, and it is kept for
// this size of screen (increment 56).
func TestTheTextScaleIsChosenAndKept(t *testing.T) {
	t.Parallel()
	first, _ := panelPane("map")
	second, _ := panelPane("workflow")
	shown := &shell{
		tabs: []*tab{{panes: []*pane{first}}, {panes: []*pane{second}}}, active: 1, folded: map[string]bool{},
		leftFolded: map[string]bool{}, widths: map[string]kindWidths{}, screen: "1512x982", known: map[string]bool{},
		textScale: 1,
	}
	second.host.frame.StepTextScale(1)
	second.host.frame.StepTextScale(1)
	shown.followTextScale()
	if shown.textScale != 1.25 || first.host.frame.TextScale() != 1.25 || second.host.frame.TextScale() != 1.25 {
		t.Fatalf("scale %v, panes %v %v", shown.textScale, first.host.frame.TextScale(), second.host.frame.TextScale())
	}
	if said, _ := second.host.frame.Message(); !strings.HasPrefix(said, "Text and controls at 125%") {
		t.Fatalf("the status bar says %q", said)
	}
	if kept := shown.kept.Screens["1512x982"].TextScale; kept != 1.25 {
		t.Fatalf("kept %v", kept)
	}
	second.host.frame.AskTextScale(1)
	shown.followTextScale()
	if shown.textScale != 1 || shown.minWidth != 0 {
		t.Fatalf("Cmd+Shift+0 gives %v", shown.textScale)
	}
}

// TestTheTextScaleIsPutBack puts back the scale kept for this size of
// screen, and not another's.
func TestTheTextScaleIsPutBack(t *testing.T) {
	t.Parallel()
	first, _ := panelPane("map")
	shown := &shell{
		tabs: []*tab{{panes: []*pane{first}}}, folded: map[string]bool{}, leftFolded: map[string]bool{},
		widths: map[string]kindWidths{}, screen: "1512x982", known: map[string]bool{}, textScale: 1,
	}
	kept, err := json.Marshal(protocol.Layout{Screens: map[string]protocol.Arrangement{
		"1512x982":  {Kinds: map[string]protocol.KindLayout{}, TextScale: 1.5},
		"2560x1440": {Kinds: map[string]protocol.KindLayout{}, TextScale: 0.75},
	}})
	if err != nil {
		t.Fatal(err)
	}
	shown.keepArrangement(kept)
	shown.followTextScale()
	if shown.textScale != 1.5 || first.host.frame.TextScale() != 1.5 {
		t.Fatalf("scale %v, pane %v", shown.textScale, first.host.frame.TextScale())
	}
}
