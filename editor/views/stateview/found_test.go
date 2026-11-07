package stateview_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/stateview"
)

// openAt opens the window at an item, as a search's Open does.
func (f *fixture) openAt(item string) {
	f.t.Helper()
	f.state.Changed(client.Update{Attention: true, Item: item}, f.ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestFindConstantsByWords finds the guards' greeting by its path, its
// default and Guard 1's own text, and the letter by a word on its second
// line.
func TestFindConstantsByWords(t *testing.T) {
	t.Parallel()
	f := constantsFixture(t)
	for words, want := range map[string]string{
		"greeting": stateview.ConstantItem + greetingID.String(),
		"who goes": stateview.ConstantItem + greetingID.String(),
		"brave":    stateview.ConstantItem + greetingID.String() + ":" + guardID.String(),
		"roof":     stateview.ConstantItem + letterID.String(),
	} {
		found := stateview.FindConstants(f.ctx.Docs, words)
		if len(found) != 1 || found[0].Item != want {
			t.Errorf("%q finds %s: %+v", words, want, found)
		}
	}
	if found := stateview.FindConstants(f.ctx.Docs, "brave"); len(found) == 1 && found[0].Holder == "" {
		t.Fatal("a holder's own text says whose it is")
	}
}

// TestOpenedAtAConstant opens the window at the letter, then at Guard 1's
// own greeting: each is selected.
func TestOpenedAtAConstant(t *testing.T) {
	t.Parallel()
	f := constantsFixture(t)
	f.openAt(stateview.ConstantItem + letterID.String())
	if !f.has("selected=world.letter") {
		t.Fatal("the letter is selected")
	}
	f.openAt(stateview.ConstantItem + greetingID.String() + ":" + guardID.String())
	if !f.has("selected=npc.guard1.guards.greeting") {
		t.Fatal("Guard 1's own greeting is selected")
	}
}
