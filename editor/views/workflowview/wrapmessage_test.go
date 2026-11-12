package workflowview_test

import (
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/ui"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestASayWrapsItsLineOnTheNode grows a Say a line for each further line
// its message takes, at most four, and leaves the node under it where it
// was put (increment 50).
func TestASayWrapsItsLineOnTheNode(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSay)
	f.add(flow.KindWait)
	waitAt := f.at("node:" + flow.KindWait + ":1")
	f.click("node:" + flow.KindSay + ":1")
	f.typeInto("param-message", "Fresh bread, still warm from the oven this morning!")
	if !f.has("say-lines:say:1=3") {
		t.Fatal("the line wraps on the node")
	}
	if f.at("node:"+flow.KindWait+":1") != waitAt {
		t.Fatal("the node under it stays where it was put")
	}
	f.typeInto("param-message", strings.Repeat("Fresh bread, still warm! ", 6))
	if !f.has("say-lines:say:1=4") {
		t.Fatalf("it shows four lines at most: %q", f.nth(flow.KindSay, 1).Params["message"])
	}
}

// TestASaysFieldHasRoom gives a Say's field three rows before anything is
// typed, and lets it grow past a Log's six (increment 50).
func TestASaysFieldHasRoom(t *testing.T) {
	t.Parallel()
	log := withNode(t, flow.KindLog)
	say := withNode(t, flow.KindSay)
	logGap := log.at("ui:insert-value").Y - log.at("ui:param-message").Y
	sayGap := say.at("ui:insert-value").Y - say.at("ui:param-message").Y
	if sayGap-logGap < ui.LineHeight/ui.Half {
		t.Fatalf("a Say's field starts taller: %d against %d", sayGap, logGap)
	}
	say.typeOnly("param-message", strings.Repeat("Fresh bread, still warm! ", 40))
	log.typeOnly("param-message", strings.Repeat("Fresh bread, still warm! ", 40))
	if say.at("ui:insert-value").Y-log.at("ui:insert-value").Y < 3*ui.LineHeight {
		t.Fatal("and grows past six rows")
	}
}
