package show_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/show"
)

// TestMarkupRuns reads bold, italic and colour, nested, into runs; [[ is a
// [; and a colour by name or by #rrggbb.
func TestMarkupRuns(t *testing.T) {
	t.Parallel()
	runs, problems := show.Parse("Welcome to [b]Marta's [i]Fish[/i][/b]! Today: [color=gold]mackerel[/color], " +
		"[[sale] [colour=#AA3300]now[/colour].")
	want := []show.Run{
		{Text: "Welcome to ", Bold: false, Italic: false, Colour: ""},
		{Text: "Marta's ", Bold: true, Italic: false, Colour: ""},
		{Text: "Fish", Bold: true, Italic: true, Colour: ""},
		{Text: "! Today: ", Bold: false, Italic: false, Colour: ""},
		{Text: "mackerel", Bold: false, Italic: false, Colour: "gold"},
		{Text: ", [sale] ", Bold: false, Italic: false, Colour: ""},
		{Text: "now", Bold: false, Italic: false, Colour: "#aa3300"},
		{Text: ".", Bold: false, Italic: false, Colour: ""},
	}
	if len(problems) != 0 || !slices.Equal(runs, want) {
		t.Fatalf("runs %+v, problems %v", runs, problems)
	}
	if plain := show.Plain("[b]Fresh[/b] off the [i]boat[/i]."); plain != "Fresh off the boat." {
		t.Fatalf("plain: %q", plain)
	}
}

// TestMarkupProblems shows a tag left open, one closing nothing, one not
// known and a colour not known as typed, each with a problem.
func TestMarkupProblems(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]string{
		"[b]Mackerel today":        "[b] is never closed",
		"Mackerel[/i] today":       "[/i] closes nothing",
		"A [wave]big[/wave] fish":  "[wave] is not a tag",
		"[color=pink]Pink[/color]": "pink is not a colour",
	} {
		problems := show.Problems(text)
		if len(problems) == 0 || !strings.Contains(problems[0], want) {
			t.Errorf("%q: %v", text, problems)
		}
	}
	if plain := show.Plain("[b]Mackerel today"); plain != "[b]Mackerel today" {
		t.Fatalf("a tag left open shows as typed: %q", plain)
	}
}

// TestEscapedTextReadsAsWritten keeps a value's text as written, however
// it looks like markup, and times reading by the text shown.
func TestEscapedTextReadsAsWritten(t *testing.T) {
	t.Parallel()
	if plain := show.Plain("Hello, " + show.Escape("[i]Bob") + "!"); plain != "Hello, [i]Bob!" {
		t.Fatalf("a value's text is never markup: %q", plain)
	}
	long := strings.Repeat("a", 24)
	if show.ReadingTicks("[b]"+long+"[/b]") != show.ReadingTicks(long) {
		t.Fatal("reading time counts only the text shown")
	}
}
