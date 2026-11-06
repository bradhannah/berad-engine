package browser_test

import "testing"

// TestPreviewShowsOneLarge previews a zone large, with a tab for its
// floor, and the Right arrow walks the strip to the floor.
func TestPreviewShowsOneLarge(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("place:Zones", false)
	f.click("ui:view-preview", false)
	if !f.has("pane=preview") || !f.has("previewing=Town") {
		t.Fatal("Preview shows the first zone large")
	}
	f.click("ui:floor-tab-0", false)
	f.keys("Right")
	if !f.has("previewing=Ground") {
		t.Fatal("Right walks the strip to the zone's floor")
	}
	f.keys("Left")
	if !f.has("previewing=Town") {
		t.Fatal("Left walks back")
	}
}
