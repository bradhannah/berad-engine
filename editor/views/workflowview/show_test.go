package workflowview_test

import (
	"testing"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// TestSayAndShake gives a Say a line with a value, reads it as it would for
// Ada, shows its seconds only once it shows for seconds, and names a Shake
// screen's strength and seconds on its node.
func TestSayAndShake(t *testing.T) {
	t.Parallel()
	f := withNode(t, flow.KindSay)
	f.typeInto("param-message", "Hello from {npc.name}")
	if !f.has("message=Hello from {npc.name}") || !f.has("reads=Hello from Ada Clerk") {
		t.Fatal("a Say's line holds a value and reads as it would for Ada")
	}
	if !f.has("title:" + flow.KindSay + ":1=Say · Hello from {npc.name}") {
		t.Fatal("the node shows its line")
	}
	if _, found := f.state.Locate("ui:param-seconds", f.docs); found {
		t.Fatal("no seconds while it shows for as long as it takes to read")
	}
	f.click("ui:" + flow.ParamShows + "-" + flow.ShowsSeconds)
	f.typeInto("param-seconds", "6")
	if seconds, err := f.nth(flow.KindSay, 1).Seconds(); err != nil || seconds != 6 {
		t.Fatalf("it shows for 6 seconds: %d %v", seconds, err)
	}
	f.add(flow.KindShake)
	if !f.has("title:" + flow.KindShake + ":1=Shake screen · moderate · 2 s") {
		t.Fatal("a Shake screen names its strength and seconds")
	}
}
