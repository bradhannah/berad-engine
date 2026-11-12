package workflowview_test

import (
	"fmt"
	"image"
	"strings"
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/ui"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// typeOnly types into a field without saving.
func (f *fixture) typeOnly(field, text string) {
	f.t.Helper()
	f.click("ui:" + field)
	f.keys("Cmd+A")
	input := ui.NewInput(f.at("ui:" + field))
	input.Text = text
	f.frame(input)
}

// TestOneSaveForSeveralFields types an option's name and share, then saves
// both with one click: one command, so one undo step. A draft survives the
// form being rebuilt when an option is added.
func TestOneSaveForSeveralFields(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.add("" + flow.KindRandom)
	before := len(f.requests.Of(workflowplugin.ReplaceEdit))
	f.typeOnly("options-1-label", "left")
	f.typeOnly("options-1-share", "3")
	if f.state.Unsaved(f.docs) != 2 {
		t.Fatalf("unsaved %d", f.state.Unsaved(f.docs))
	}
	f.click("ui:save")
	if replaces := len(f.requests.Of(workflowplugin.ReplaceEdit)); replaces != before+1 {
		t.Fatalf("one Save sends one command, not %d", replaces-before)
	}
	options, _ := f.nth(flow.KindRandom, 1).Shares("options")
	if options[0].Label != "left" || options[0].Share != 3 {
		t.Fatalf("options %+v", options)
	}
	f.typeOnly("options-2-label", "right")
	f.click("ui:add-option")
	if f.state.Unsaved(f.docs) != 1 {
		t.Fatal("adding an option keeps the typed name")
	}
	f.state.Save(f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	options, _ = f.nth(flow.KindRandom, 1).Shares("options")
	if len(options) != 3 || options[1].Label != "right" {
		t.Fatalf("options after adding one %+v", options)
	}
}

// TestABadFieldSavesNothing saves a good share beside a share that is not a
// number: nothing is sent, and the status says why.
func TestABadFieldSavesNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.add("" + flow.KindRandom)
	before := len(f.requests.Of(workflowplugin.ReplaceEdit))
	f.typeOnly("options-1-share", "2")
	f.typeOnly("options-2-share", "lots")
	if f.state.Save(f.ctx) || len(f.requests.Of(workflowplugin.ReplaceEdit)) != before || f.status() == "" {
		t.Fatalf("a bad field stops the save: %q", f.status())
	}
}

// TestSelectingAnotherNodeAsks types into a node, then clicks another:
// Cancel stays, and Don't save moves on without saving.
func TestSelectingAnotherNodeAsks(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.add("" + flow.KindWait)
	before := len(f.requests.Of(workflowplugin.ReplaceEdit))
	f.typeOnly("param-duration", "9")
	f.click("node:start:1")
	f.click("ui:cancel")
	if !f.has("selected:"+flow.KindWait) || f.state.Unsaved(f.docs) != 1 {
		t.Fatal("Cancel keeps the Wait selected with its change")
	}
	f.click("node:start:1")
	f.click("ui:discard-changes")
	if !f.has("selected:"+flow.KindStart) || len(f.requests.Of(workflowplugin.ReplaceEdit)) != before {
		t.Fatal("Don't save selects the start and saves nothing")
	}
}

// TestInsertValueKeepsTheCaret puts a constant where the caret was in a
// line typed and saved, though clicking Insert value… took the focus; the
// field keeps it, so typing goes on after the constant.
func TestInsertValueKeepsTheCaret(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSay)
	f.withConstants()
	f.typeInto("param-message", "Hello there")
	f.click("ui:param-message")
	f.keys("Home")
	f.click("ui:insert-value")
	f.typeText("greeting")
	f.click("valuerow:const.npc.shop.greeting")
	if !f.has("line={const.npc.shop.greeting}Hello there") {
		t.Fatal("the constant goes where the caret was, not at the end")
	}
	f.typeText(" ")
	if !f.has("line={const.npc.shop.greeting} Hello there") || !f.has("unsaved=1") {
		t.Fatal("typing goes on after it, all waiting for Save")
	}
}

// TestThePreviewReadsTheField reads the line typed and not saved yet, and
// says what keeps one naming no value from being read.
func TestThePreviewReadsTheField(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSay)
	f.withConstants()
	f.typeOnly("param-message", "Back in {const.world.restock_minutes} minutes")
	if !f.has("reads=Back in 30 minutes") || !f.has("unsaved=1") {
		t.Fatal("the line as it would read follows what is typed")
	}
	f.typeOnly("param-message", "Back in {const.world.restock}")
	if !f.has("reads-problem=No value is called const.world.restock. Put one in with Insert value…") {
		t.Fatal("a path naming no value says so where the line would read")
	}
}

// TestAConstantChosenAsksBeforeLeaving chooses a constant for a Wait, then
// clicks the start: the window asks, as for typing, and Don't save leaves
// the Wait as it was.
func TestAConstantChosenAsksBeforeLeaving(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindWait)
	f.withConstants()
	f.click("ui:constant-duration")
	f.typeText("restock")
	f.click("valuerow:const.world.restock_minutes")
	f.click("node:start:1")
	f.click("ui:discard-changes")
	wait, err := f.nth(flow.KindWait, 1).Duration("duration")
	if !f.has("selected:"+flow.KindStart) || err != nil || !wait.Constant.IsZero() {
		t.Fatalf("Don't save moves on and leaves the Wait typed: %+v %v", wait, err)
	}
}

// TestAConstantChosenSurvivesARebuild chooses a constant for a random
// choice's first share, then adds an option, which rebuilds the form: the
// constant is still chosen, and Save keeps it with the new option.
func TestAConstantChosenSurvivesARebuild(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindRandom)
	f.withConstants()
	f.click("ui:constant-options-1")
	f.typeText("restock")
	f.click("valuerow:const.world.restock_minutes")
	f.click("ui:add-option")
	if !f.has("constant:options-1=const.world.restock_minutes") || !f.has("unsaved=1") {
		t.Fatal("the constant chosen outlasts the form being rebuilt")
	}
	f.click("ui:save")
	options, err := f.nth(flow.KindRandom, 1).Shares("options")
	if err != nil || len(options) != 3 || options[0].Constant.Value != restockID {
		t.Fatalf("Save keeps it beside the new option: %+v %v", options, err)
	}
}

// TestALongLineGrowsItsField wraps a Log's long line in its field, which
// grows a row at a time, pushing Insert value… down, up to six rows. A
// Say's field starts with more room (increment 50).
func TestALongLineGrowsItsField(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindLog)
	short := f.at("ui:insert-value").Y
	f.typeOnly("param-message", strings.Repeat("Fresh bread, still warm! ", 4))
	grown := f.at("ui:insert-value").Y
	if grown < short+ui.LineHeight/ui.Half {
		t.Fatalf("a line that wraps grows its field: Insert value… from %d to %d", short, grown)
	}
	f.typeOnly("param-message", strings.Repeat("Fresh bread, still warm! ", 40))
	tallest := f.at("ui:insert-value").Y
	if tallest <= grown || tallest > short+6*ui.LineHeight {
		t.Fatalf("it grows to six rows at most: Insert value… from %d to %d", short, tallest)
	}
}

// TestWriteOpensTheLineInTheSheet writes a Say's line in the writing
// sheet, from what its field holds; Done puts what was written back in the
// field, waiting for Save as typing does.
func TestWriteOpensTheLineInTheSheet(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSay)
	f.withConstants()
	f.typeInto("param-message", "Hello! {const.npc.shop.greeting}")
	f.click("ui:write-line")
	if !f.has("sheet") || !f.has("sheet-text=Hello! {const.npc.shop.greeting}") {
		t.Fatal("Write… opens the line in the sheet")
	}
	f.typeText(" Come again.")
	f.click("ui:sheet-done")
	if f.has("sheet") || !f.has("line=Hello! {const.npc.shop.greeting} Come again.") || !f.has("unsaved=1") {
		t.Fatal("Done puts the line back in its field, waiting for Save")
	}
	f.click("ui:save")
	if !f.has("message=Hello! {const.npc.shop.greeting} Come again.") {
		t.Fatal("Save keeps it")
	}
}

// TestASayBreaksItsLine breaks a Say's line with Shift+Enter, and Save
// keeps the break; Enter in a Log's message still saves it as one line.
func TestASayBreaksItsLine(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSay)
	f.typeOnly("param-message", "Good morning.")
	f.keys("Shift+Enter")
	f.typeText("Come in.")
	if !f.has("line=Good morning.\nCome in.") || !f.has("unsaved=1") {
		t.Fatal("Shift+Enter starts a new line in a Say's line, waiting for Save")
	}
	f.keys("Enter")
	if !f.has("message=Good morning.\nCome in.") {
		t.Fatal("Enter saves the line, its break kept")
	}
	log := withNode(t, flow.KindLog)
	log.typeOnly("param-message", "Opened the shop.")
	log.keys("Shift+Enter")
	if !log.has("message=Opened the shop.") {
		t.Fatal("a Log's message stays one line: Shift+Enter saves it")
	}
}

// TestTheInspectorScrolls fills a Say's inspector past its notes with a long
// line in a short window: the wheel scrolls it, a script finds the colours
// below by scrolling to them, and another node starts at the top.
func TestTheInspectorScrolls(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSay)
	f.ctx.Size = image.Pt(1200, 600)
	f.typeOnly("param-message", strings.Repeat("Fresh bread, still warm! ", 40))
	if !f.has("inspector-scrolls") {
		t.Fatal("a long line in a short window fills the inspector past its notes, so it scrolls")
	}
	top := f.at("ui:insert-value")
	wheel := ui.NewInput(top)
	wheel.WheelY = -1
	f.frame(wheel)
	if scrolled := f.at("ui:insert-value"); scrolled.Y != top.Y-ui.RowHeight {
		t.Fatalf("a notch of the wheel scrolls a row: Insert value… from %d to %d", top.Y, scrolled.Y)
	}
	f.click("ui:swatch:node:green")
	if !f.has("tint:" + flow.KindSay + ":1=green") {
		t.Fatal("a script finds the colours under the notes by scrolling to them")
	}
	f.click("ui:save")
	f.click("node:start:1")
	if f.has("inspector-scrolls") {
		t.Fatal("the start's inspector starts at the top, and has no need to scroll")
	}
}

// TestATimeMovesLaterInOneSave moves a time switch's first time later, from
// 09:00–17:00 to 17:00–20:00: its new start is its old end. Both ends are
// typed and saved together, so the time is read as a whole, not refused for
// the moment its start meets the end it is leaving. A start typed to meet
// its end, alone, is still refused.
func TestATimeMovesLaterInOneSave(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("node:start:1")
	f.add("" + flow.KindTimeSwitch)
	before, _ := f.nth(flow.KindTimeSwitch, 1).TimeRanges("ranges")
	end := fmt.Sprintf("%02d:%02d", before[0].To/60, before[0].To%60)
	f.typeOnly("ranges-1-from", end)
	f.typeOnly("ranges-1-to", "23:30")
	f.click("ui:save")
	spans, _ := f.nth(flow.KindTimeSwitch, 1).TimeRanges("ranges")
	if spans[0].From != before[0].To || spans[0].To != 23*60+30 {
		message, _ := f.ctx.Frame.Message()
		t.Fatalf("the time moved later in one save: %+v, status %q", spans[0], message)
	}
	f.typeOnly("ranges-1-from", "23:30")
	f.click("ui:save")
	spans, _ = f.nth(flow.KindTimeSwitch, 1).TimeRanges("ranges")
	if message, _ := f.ctx.Frame.Message(); spans[0].From != before[0].To || !strings.Contains(message, "end after it starts") {
		t.Fatalf("a start typed to meet its end is refused: %+v, status %q", spans[0], message)
	}
}
