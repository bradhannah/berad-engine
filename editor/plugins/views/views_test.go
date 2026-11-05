package views_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugins/views"
)

func TestSettingViewsUndoesAndRefusesDuplicates(t *testing.T) {
	t.Parallel()
	document := &views.Document{Views: nil}
	harbour := views.View{Name: "Harbour, this week", Search: "", Ticks: map[string][]string{"zone": {"Z1"}}}
	args, err := json.Marshal(views.SetArgs{Views: []views.View{harbour}})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := (views.Set{}).Apply(document, args)
	if err != nil || len(document.Views) != 1 {
		t.Fatalf("one view saved: %v %+v", err, document.Views)
	}
	if _, err := (views.Set{}).Apply(document, applied.Inverse); err != nil || len(document.Views) != 0 {
		t.Fatalf("undone, none: %v %+v", err, document.Views)
	}
	twice, err := json.Marshal(views.SetArgs{Views: []views.View{harbour, {Name: "harbour, THIS week", Search: "", Ticks: nil}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (views.Set{}).Apply(document, twice); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("two views with one name: %v", err)
	}
	data, err := (&views.Document{Views: []views.View{harbour}}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := views.Decode(data)
	if err != nil || !read.(*views.Document).Views[0].Same(harbour) {
		t.Fatalf("read back: %v %s", err, data)
	}
}

func TestSameIgnoresOrderAndEmptyFacets(t *testing.T) {
	t.Parallel()
	first := views.View{Name: "A", Search: "bell", Ticks: map[string][]string{"kind": {"npc", "workflow"}, "tags": nil}}
	second := views.View{Name: "B", Search: " bell ", Ticks: map[string][]string{"kind": {"workflow", "npc"}}}
	if !first.Same(second) {
		t.Fatal("the same ticks in another order narrow the same way")
	}
	if first.Same(views.View{Name: "C", Search: "bell", Ticks: map[string][]string{"kind": {"npc"}}}) {
		t.Fatal("fewer ticks narrow differently")
	}
}
