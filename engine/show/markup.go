package show

import (
	"regexp"
	"slices"
	"strings"
)

// Markup in a line's or a box's text: [b]bold[/b], [i]italic[/i] and
// [color=gold]gold[/color], which nest, and [[ for a [ itself. A colour is
// one of Colours, which a game draws from its own palette, or #rrggbb.
// Nothing in markup changes the flow: there are no pauses. A tag left
// open, one closing nothing, or one not known is shown as typed, and
// Parse says what is wrong with it.
const (
	tagOpen    = "["
	tagClose   = "]"
	tagBold    = "b"
	tagItalic  = "i"
	tagColour  = "color"
	tagColourB = "colour"
)

// Colours returns the colours markup can name, in the order an editor
// offers them.
func Colours() []string {
	return []string{"red", "orange", "gold", "green", "blue", "purple", "grey", "white"}
}

// hexColour is a colour written as #rrggbb.
var hexColour = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Run is a stretch of text drawn one way: bold or not, italic or not, and
// in a colour, a name or #rrggbb, or "" for the text's own.
type Run struct {
	Text   string
	Bold   bool
	Italic bool
	Colour string
}

// token is one piece of a text: its own words, or a tag, opening or
// closing one of bold, italic or colour.
type token struct {
	text    string
	tag     string
	closing bool
	colour  string
	matched bool
}

// Parse reads a text's markup into runs, and says what is wrong with any
// tag, each problem once, in the order met. A tag that is wrong is shown as
// typed.
func Parse(text string) ([]Run, []string) {
	tokens, problems := tokenize(text)
	problems = append(problems, match(tokens)...)
	var runs []Run
	bold, italic := 0, 0
	var colours []string
	add := func(words string) {
		if words == "" {
			return
		}
		run := Run{Text: words, Bold: bold > 0, Italic: italic > 0, Colour: ""}
		if len(colours) > 0 {
			run.Colour = colours[len(colours)-1]
		}
		if last := len(runs) - 1; last >= 0 && sameLook(runs[last], run) {
			runs[last].Text += words

			return
		}
		runs = append(runs, run)
	}
	for _, each := range tokens {
		switch {
		case each.tag == "" || !each.matched:
			add(each.text)
		case each.tag == tagBold:
			bold += step(each.closing)
		case each.tag == tagItalic:
			italic += step(each.closing)
		case each.closing:
			colours = colours[:len(colours)-1]
		default:
			colours = append(colours, each.colour)
		}
	}

	return runs, dedupe(problems)
}

// Plain returns a text as it reads without its markup: what the Log shows
// and reading time counts.
func Plain(text string) string {
	runs, _ := Parse(text)
	var plain strings.Builder
	for _, run := range runs {
		plain.WriteString(run.Text)
	}

	return plain.String()
}

// Escape returns a text that reads exactly as written, its every [ doubled,
// for a value's text in a message, which the game sets and is never
// markup.
func Escape(text string) string { return strings.ReplaceAll(text, tagOpen, tagOpen+tagOpen) }

// Problems says what is wrong with a text's markup, or nothing.
func Problems(text string) []string {
	_, problems := Parse(text)

	return problems
}

func step(closing bool) int {
	if closing {
		return -1
	}

	return 1
}

func sameLook(a, b Run) bool { return a.Bold == b.Bold && a.Italic == b.Italic && a.Colour == b.Colour }

// tokenize splits a text into its words and its tags; [[ is a [, and a tag
// not known is words, with a problem.
func tokenize(text string) ([]token, []string) {
	var tokens []token
	var problems []string
	var words strings.Builder
	flush := func() {
		if words.Len() > 0 {
			tokens = append(tokens, token{text: words.String(), tag: "", closing: false, colour: "", matched: false})
			words.Reset()
		}
	}
	for rest := text; rest != ""; {
		if escaped, found := strings.CutPrefix(rest, tagOpen+tagOpen); found {
			words.WriteString(tagOpen)
			rest = escaped

			continue
		}
		inside, after, isTag := cutTag(rest)
		if !isTag {
			words.WriteString(rest[:1])
			rest = rest[1:]

			continue
		}
		known, problem := readTag(inside)
		if problem != "" {
			problems = append(problems, problem)
			words.WriteString(tagOpen + inside + tagClose)
			rest = after

			continue
		}
		flush()
		known.text = tagOpen + inside + tagClose
		tokens = append(tokens, known)
		rest = after
	}
	flush()

	return tokens, problems
}

// cutTag reads a tag at the start of a text: what is inside its brackets,
// and the text after it.
func cutTag(text string) (string, string, bool) {
	if !strings.HasPrefix(text, tagOpen) {
		return "", text, false
	}
	inside, after, closed := strings.Cut(text[len(tagOpen):], tagClose)
	if !closed || inside == "" || strings.Contains(inside, tagOpen) || strings.Contains(inside, "\n") {
		return "", text, false
	}

	return inside, after, true
}

// readTag reads what is inside a tag's brackets: bold, italic or a colour,
// opening or closing, or what is wrong with it.
func readTag(inside string) (token, string) {
	made := token{text: "", tag: "", closing: false, colour: "", matched: false}
	name, closing := strings.CutPrefix(inside, "/")
	made.closing = closing
	switch {
	case name == tagBold || name == tagItalic:
		made.tag = name
	case closing && (name == tagColour || name == tagColourB):
		made.tag = tagColour
	case !closing && (strings.HasPrefix(name, tagColour+"=") || strings.HasPrefix(name, tagColourB+"=")):
		_, colour, _ := strings.Cut(name, "=")
		if !slices.Contains(Colours(), colour) && !hexColour.MatchString(colour) {
			return made, tagOpen + inside + tagClose + ": " + colour + " is not a colour; use one of " +
				strings.Join(Colours(), ", ") + ", or #rrggbb"
		}
		made.tag, made.colour = tagColour, strings.ToLower(colour)
	default:
		return made, tagOpen + inside + tagClose + " is not a tag; markup has [b], [i] and [color=…]"
	}

	return made, ""
}

// match pairs each closing tag with the latest open one of its kind; one
// left open, or one closing nothing, stays as typed, with a problem.
func match(tokens []token) []string {
	var problems []string
	open := map[string][]int{}
	for index := range tokens {
		each := &tokens[index]
		if each.tag == "" {
			continue
		}
		if !each.closing {
			open[each.tag] = append(open[each.tag], index)

			continue
		}
		waiting := open[each.tag]
		if len(waiting) == 0 {
			problems = append(problems, each.text+" closes nothing")

			continue
		}
		opener := waiting[len(waiting)-1]
		open[each.tag] = waiting[:len(waiting)-1]
		each.matched, tokens[opener].matched = true, true
	}
	for index := range tokens {
		if tokens[index].tag != "" && !tokens[index].closing && !tokens[index].matched {
			problems = append(problems, tokens[index].text+" is never closed")
		}
	}

	return problems
}

func dedupe(problems []string) []string {
	var kept []string
	for _, problem := range problems {
		if !slices.Contains(kept, problem) {
			kept = append(kept, problem)
		}
	}

	return kept
}
