package art

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/content"
)

// A template is a drawing aid kept beside a sprite and never part of it
// (increment 16): rules that mark the pixels near a tile's edges discouraged
// or not allowed, and guide lines to draw to. The Pixel editor's tools never
// paint a pixel the sprite's template does not allow. Games never read
// templates, so they belong to the editor alone.

// TemplateFormatName names the template format; TemplateVersion is the
// version this package writes.
const (
	TemplateFormatName = "template"
	TemplateVersion    = 1
)

// Template limits. A rule's width and a guide's distance are in pixels.
const (
	MaxRules           = 16
	MaxGuides          = 16
	MaxRuleWidth       = art.MaxTileSize / 2
	MaxGuideNameLength = 40
)

// ErrTemplate reports a template that is not valid: an unknown edge, mark,
// line or starting point, a width or distance out of range, a guide's name
// that is empty, untrimmed or too long, or too many rules or guides.
var ErrTemplate = errors.New("invalid template")

// Mark is what a template says of one pixel. A rule marks pixels discouraged
// or not allowed; a pixel no rule marks is open.
type Mark string

// Marks, from the least strict.
const (
	MarkOpen        Mark = ""
	MarkDiscouraged Mark = "discouraged"
	MarkNotAllowed  Mark = "not-allowed"
)

// strictness orders marks, so the strictest of overlapping rules wins.
func (m Mark) strictness() int {
	switch m {
	case MarkDiscouraged:
		return 1
	case MarkNotAllowed:
		return 2 //nolint:mnd // The strictest mark.
	case MarkOpen:
	}

	return 0
}

// Edge names what a rule measures from: every side of the tile, its left
// and right sides, its top or its bottom.
type Edge string

// Edges.
const (
	EdgeBorder Edge = "border"
	EdgeSides  Edge = "sides"
	EdgeTop    Edge = "top"
	EdgeBottom Edge = "bottom"
)

// Edges lists the edges in the order the Template editor offers them.
func Edges() []Edge { return []Edge{EdgeBorder, EdgeSides, EdgeTop, EdgeBottom} }

// Rule marks the pixels within Width of its edge: a border rule of width 1
// marks the outermost ring of pixels.
type Rule struct {
	Edge  Edge `json:"edge"`
	Width int  `json:"width"`
	Mark  Mark `json:"mark"`
}

// covers reports whether the rule marks a pixel of a tile of a size.
func (r Rule) covers(col, row, width, height int) bool {
	left, right, top, bottom := col, width-1-col, row, height-1-row
	switch r.Edge {
	case EdgeBorder:
		return min(left, right, top, bottom) < r.Width
	case EdgeSides:
		return min(left, right) < r.Width
	case EdgeTop:
		return top < r.Width
	case EdgeBottom:
		return bottom < r.Width
	}

	return false
}

// Line says which way a guide runs: across the tile, or down it.
type Line string

// Lines.
const (
	LineAcross Line = "across"
	LineDown   Line = "down"
)

// From says where a guide is measured from: the top or the left, the bottom
// or the right, or the middle.
type From string

// Starting points.
const (
	FromStart  From = "start"
	FromEnd    From = "end"
	FromMiddle From = "middle"
)

// Guide is a named line to draw to, across or down the tile, at a distance
// in pixels from where it is measured; from the middle, a negative distance
// is above or to the left. A guide lies between pixels: across at 2 from the
// top is the line under the second row.
type Guide struct {
	Name string `json:"name"`
	Line Line   `json:"line"`
	From From   `json:"from"`
	At   int    `json:"at"`
}

// Position returns where the guide lies on a tile of a size, as a number of
// pixels from the top or the left, kept within the tile.
func (g Guide) Position(width, height int) int {
	length := height
	if g.Line == LineDown {
		length = width
	}
	position := g.At
	switch g.From {
	case FromEnd:
		position = length - g.At
	case FromMiddle:
		position = length/2 + g.At //nolint:mnd // The middle of the tile.
	case FromStart:
	}

	return max(0, min(length, position))
}

// Template is a template's rules and guides.
type Template struct {
	Rules  []Rule  `json:"rules"`
	Guides []Guide `json:"guides"`
}

// Mark returns what the template says of a pixel of a tile of a size: the
// strictest mark of the rules that cover it.
func (t Template) Mark(col, row, width, height int) Mark {
	mark := MarkOpen
	for _, rule := range t.Rules {
		if rule.covers(col, row, width, height) && rule.Mark.strictness() > mark.strictness() {
			mark = rule.Mark
		}
	}

	return mark
}

// Marks returns the mark of every pixel of a tile of a size, row by row.
func (t Template) Marks(width, height int) []Mark {
	marks := make([]Mark, 0, width*height)
	for row := range height {
		for col := range width {
			marks = append(marks, t.Mark(col, row, width, height))
		}
	}

	return marks
}

// Validate reports a template that is not valid.
func (t Template) Validate() error {
	if len(t.Rules) > MaxRules || len(t.Guides) > MaxGuides {
		return fmt.Errorf("%w: %d rules and %d guides", ErrTemplate, len(t.Rules), len(t.Guides))
	}
	for index, rule := range t.Rules {
		if !slices.Contains(Edges(), rule.Edge) || rule.Width < 1 || rule.Width > MaxRuleWidth ||
			(rule.Mark != MarkDiscouraged && rule.Mark != MarkNotAllowed) {
			return fmt.Errorf("%w: rule %d", ErrTemplate, index+1)
		}
	}
	for index, guide := range t.Guides {
		if err := guide.validate(); err != nil {
			return fmt.Errorf("guide %d: %w", index+1, err)
		}
	}

	return nil
}

func (g Guide) validate() error {
	switch {
	case g.Name == "" || strings.TrimSpace(g.Name) != g.Name || utf8.RuneCountInString(g.Name) > MaxGuideNameLength:
		return fmt.Errorf("%w: name %q", ErrTemplate, g.Name)
	case g.Line != LineAcross && g.Line != LineDown:
		return fmt.Errorf("%w: line %q", ErrTemplate, g.Line)
	case g.From != FromStart && g.From != FromEnd && g.From != FromMiddle:
		return fmt.Errorf("%w: from %q", ErrTemplate, g.From)
	case g.At < -art.MaxTileSize || g.At > art.MaxTileSize:
		return fmt.Errorf("%w: at %d", ErrTemplate, g.At)
	}

	return nil
}

// Clone returns an independent copy.
func (t Template) Clone() Template {
	return Template{Rules: slices.Clone(t.Rules), Guides: slices.Clone(t.Guides)}
}

// Equal reports whether two templates have the same rules and guides.
func (t Template) Equal(other Template) bool {
	return slices.Equal(t.Rules, other.Rules) && slices.Equal(t.Guides, other.Guides)
}

// Encode returns the template's saved data.
func (t Template) Encode() (json.RawMessage, error) {
	saved := t.Clone()
	if saved.Rules == nil {
		saved.Rules = []Rule{}
	}
	if saved.Guides == nil {
		saved.Guides = []Guide{}
	}
	data, err := json.Marshal(saved)
	if err != nil {
		return nil, fmt.Errorf("encode template: %w", err)
	}

	return data, nil
}

// DecodeTemplateData reads saved template data in the current format.
func DecodeTemplateData(data json.RawMessage) (Template, error) {
	var template Template
	if err := json.Unmarshal(data, &template); err != nil {
		return template, fmt.Errorf("%w: %w", ErrTemplate, err)
	}
	if err := template.Validate(); err != nil {
		return template, err
	}

	return template, nil
}

// TemplateFormat returns the template format registration.
func TemplateFormat() content.Format {
	return content.Format{Name: TemplateFormatName, Current: TemplateVersion, Migrations: nil}
}

// TemplateDocument is a template held by the editor.
type TemplateDocument struct {
	Template Template
}

// Format names the document format.
func (d *TemplateDocument) Format() string { return TemplateFormatName }

// Encode returns the saved data.
func (d *TemplateDocument) Encode() (json.RawMessage, error) {
	data, err := d.Template.Encode()
	if err != nil {
		return nil, fmt.Errorf("template document: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
//
//nolint:ireturn // Documents are polymorphic; callers hold them as model.Document.
func (d *TemplateDocument) Clone() model.Document {
	return &TemplateDocument{Template: d.Template.Clone()}
}

// DecodeTemplate reads a saved template.
//
//nolint:ireturn // Decoders return the polymorphic model.Document.
func DecodeTemplate(data json.RawMessage) (model.Document, error) {
	template, err := DecodeTemplateData(data)
	if err != nil {
		return nil, fmt.Errorf("template document: %w", err)
	}

	return &TemplateDocument{Template: template}, nil
}
