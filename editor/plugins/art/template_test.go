package art_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// pickup keeps the outermost ring of pixels clear and discourages the next.
func pickup() artplugin.Template {
	return artplugin.Template{
		Rules: []artplugin.Rule{
			{Edge: artplugin.EdgeBorder, Width: 1, Mark: artplugin.MarkNotAllowed},
			{Edge: artplugin.EdgeBorder, Width: 2, Mark: artplugin.MarkDiscouraged},
		},
		Guides: []artplugin.Guide{
			{Name: "centre", Line: artplugin.LineDown, From: artplugin.FromMiddle, At: 0},
			{Name: "rests on", Line: artplugin.LineAcross, From: artplugin.FromEnd, At: 2},
		},
	}
}

// character keeps three pixels clear at each side, discourages the fourth
// and the top two rows.
func character() artplugin.Template {
	return artplugin.Template{
		Rules: []artplugin.Rule{
			{Edge: artplugin.EdgeSides, Width: 3, Mark: artplugin.MarkNotAllowed},
			{Edge: artplugin.EdgeSides, Width: 4, Mark: artplugin.MarkDiscouraged},
			{Edge: artplugin.EdgeTop, Width: 2, Mark: artplugin.MarkDiscouraged},
		},
		Guides: nil,
	}
}

func TestTemplateMarksTheStrictestRule(t *testing.T) {
	t.Parallel()
	const size = 8
	marks := pickup().Marks(size, size)
	count := func(mark artplugin.Mark) int {
		return len(slices.DeleteFunc(slices.Clone(marks), func(other artplugin.Mark) bool { return other != mark }))
	}
	if count(artplugin.MarkNotAllowed) != 28 || count(artplugin.MarkDiscouraged) != 20 || count(artplugin.MarkOpen) != 16 {
		t.Fatalf("an 8 × 8 pickup marks %d, %d and %d", count(artplugin.MarkNotAllowed),
			count(artplugin.MarkDiscouraged), count(artplugin.MarkOpen))
	}
	for _, at := range []struct {
		col, row int
		want     artplugin.Mark
	}{
		{0, 0, artplugin.MarkNotAllowed}, {7, 3, artplugin.MarkNotAllowed}, {1, 1, artplugin.MarkDiscouraged},
		{6, 4, artplugin.MarkDiscouraged}, {2, 2, artplugin.MarkOpen}, {5, 5, artplugin.MarkOpen},
	} {
		if got := pickup().Mark(at.col, at.row, size, size); got != at.want {
			t.Fatalf("pixel %d, %d is %q, not %q", at.col, at.row, got, at.want)
		}
	}
	// Rules measure from the edges, so the same template fits a taller
	// sprite: a character's sides are kept clear the whole way down.
	for row := range 48 {
		if character().Mark(2, row, 24, 48) != artplugin.MarkNotAllowed ||
			character().Mark(21, row, 24, 48) != artplugin.MarkNotAllowed {
			t.Fatalf("row %d of a character's sides is open", row)
		}
	}
	if character().Mark(3, 30, 24, 48) != artplugin.MarkDiscouraged ||
		character().Mark(12, 1, 24, 48) != artplugin.MarkDiscouraged ||
		character().Mark(12, 2, 24, 48) != artplugin.MarkOpen {
		t.Fatal("a character discourages its fourth column and its top two rows")
	}
}

func TestGuidesLieBetweenPixels(t *testing.T) {
	t.Parallel()
	for _, at := range []struct {
		guide artplugin.Guide
		want  int
	}{
		{artplugin.Guide{Name: "a", Line: artplugin.LineAcross, From: artplugin.FromStart, At: 10}, 10},
		{artplugin.Guide{Name: "b", Line: artplugin.LineAcross, From: artplugin.FromEnd, At: 2}, 46},
		{artplugin.Guide{Name: "c", Line: artplugin.LineDown, From: artplugin.FromMiddle, At: 0}, 12},
		{artplugin.Guide{Name: "d", Line: artplugin.LineDown, From: artplugin.FromMiddle, At: -3}, 9},
		{artplugin.Guide{Name: "e", Line: artplugin.LineAcross, From: artplugin.FromEnd, At: 60}, 0},
		{artplugin.Guide{Name: "f", Line: artplugin.LineDown, From: artplugin.FromStart, At: 30}, 24},
	} {
		if got := at.guide.Position(24, 48); got != at.want {
			t.Fatalf("guide %s lies at %d, not %d", at.guide.Name, got, at.want)
		}
	}
}

func TestTemplateFormat(t *testing.T) {
	t.Parallel()
	data, err := pickup().Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := artplugin.DecodeTemplateData(data)
	if err != nil || !decoded.Equal(pickup()) {
		t.Fatalf("round trip: %+v %v", decoded, err)
	}
	empty, err := artplugin.Template{Rules: nil, Guides: nil}.Encode()
	if err != nil || string(empty) != `{"rules":[],"guides":[]}` {
		t.Fatalf("an empty template is saved as %s: %v", empty, err)
	}
	for _, bad := range []string{
		`{"rules":[{"edge":"left","width":1,"mark":"not-allowed"}],"guides":[]}`,
		`{"rules":[{"edge":"top","width":0,"mark":"not-allowed"}],"guides":[]}`,
		`{"rules":[{"edge":"top","width":65,"mark":"discouraged"}],"guides":[]}`,
		`{"rules":[{"edge":"top","width":1,"mark":""}],"guides":[]}`,
		`{"rules":[],"guides":[{"name":"","line":"across","from":"start","at":1}]}`,
		`{"rules":[],"guides":[{"name":" feet","line":"across","from":"start","at":1}]}`,
		`{"rules":[],"guides":[{"name":"feet","line":"diagonal","from":"start","at":1}]}`,
		`{"rules":[],"guides":[{"name":"feet","line":"across","from":"top","at":1}]}`,
		`{"rules":[],"guides":[{"name":"feet","line":"across","from":"start","at":200}]}`,
		`[`,
	} {
		if _, err := artplugin.DecodeTemplateData([]byte(bad)); !errors.Is(err, artplugin.ErrTemplate) {
			t.Fatalf("decoded %s: %v", bad, err)
		}
	}
	long := artplugin.Template{Rules: nil, Guides: []artplugin.Guide{
		{Name: strings.Repeat("x", artplugin.MaxGuideNameLength+1), Line: artplugin.LineDown, From: artplugin.FromStart, At: 1},
	}}
	if err := long.Validate(); !errors.Is(err, artplugin.ErrTemplate) {
		t.Fatalf("a long guide name: %v", err)
	}
}

func TestTemplateEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	verify(t, &artplugin.TemplateDocument{Template: artplugin.Template{Rules: nil, Guides: nil}}, []check{
		{artplugin.SetTemplate{}, encode(t, artplugin.TemplateArgs{Template: pickup()})},
		{artplugin.SetTemplate{}, encode(t, artplugin.TemplateArgs{Template: character()})},
	})
	verify(t, &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: id(1), Template: ids.ULID{}}}, []check{
		{artplugin.UseTemplate{}, encode(t, artplugin.UseTemplateArgs{Template: id(2)})},
		{artplugin.UseTemplate{}, encode(t, artplugin.UseTemplateArgs{Template: ids.ULID{}})},
	})
	document := &artplugin.TemplateDocument{Template: pickup()}
	bad := artplugin.Template{Rules: []artplugin.Rule{{Edge: "left", Width: 1, Mark: artplugin.MarkNotAllowed}}, Guides: nil}
	if _, err := (artplugin.SetTemplate{}).Apply(document, encode(t, artplugin.TemplateArgs{Template: bad})); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("a bad template was set: %v", err)
	}
	if !document.Template.Equal(pickup()) {
		t.Fatal("a refused edit changed the template")
	}
}

func TestSpriteRefersToItsTemplate(t *testing.T) {
	t.Parallel()
	plain := &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: id(1), Template: ids.ULID{}}}
	if len(plain.References()) != 0 {
		t.Fatal("a sprite drawn to no template refers to one")
	}
	drawn := &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: id(1), Template: id(2)}}
	if references := drawn.References(); len(references) != 1 || references[0].Document != id(2) {
		t.Fatalf("a sprite refers to %+v", references)
	}
}
