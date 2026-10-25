package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
)

// Number returns a whole-number parameter, such as a Move's tiles.
func (n Node) Number(name string) (int, error) {
	var number int

	return number, n.decode(name, &number)
}

// Text returns a text parameter.
func (n Node) Text(name string) (string, error) {
	var text string

	return text, n.decode(name, &text)
}

// Cue returns a cue parameter's cue ID; "" means none is chosen.
func (n Node) Cue(name string) (string, error) {
	var cue string

	return cue, n.decode(name, &cue)
}

// Anchor returns an anchor parameter; a zero reference means none is chosen.
func (n Node) Anchor(name string) (tilefloor.AnchorRef, error) {
	var ref tilefloor.AnchorRef

	return ref, n.decode(name, &ref)
}

// Choice returns a choice parameter's value.
func (n Node) Choice(name string) (string, error) {
	var chosen string

	return chosen, n.decode(name, &chosen)
}

// Duration returns a duration parameter.
func (n Node) Duration(name string) (Duration, error) {
	var wait Duration

	return wait, n.decode(name, &wait)
}

// Shares returns a random choice's options.
func (n Node) Shares(name string) ([]Share, error) {
	var options []Share

	return options, n.decode(name, &options)
}

// TimeRange returns a time range parameter.
func (n Node) TimeRange(name string) (TimeRange, error) {
	var span TimeRange

	return span, n.decode(name, &span)
}

// TimeRanges returns a list of time ranges.
func (n Node) TimeRanges(name string) ([]TimeRange, error) {
	var spans []TimeRange

	return spans, n.decode(name, &spans)
}

// WithParam returns a copy of the node with one parameter set.
func (n Node) WithParam(name string, value any) (Node, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return Node{}, fmt.Errorf("%w: %s: %w", ErrNode, name, err)
	}
	changed := n.Clone()
	if changed.Params == nil {
		changed.Params = map[string]json.RawMessage{}
	}
	changed.Params[name] = data

	return changed, nil
}

// WithLink returns a copy of the node with an output connected to another
// node, or cleared when to is zero.
func (n Node) WithLink(output string, to ids.ULID) Node {
	return withLink(n, output, to)
}

func (n Node) decode(name string, target any) error {
	value, ok := n.Params[name]
	if !ok {
		return fmt.Errorf("%w: %s has no %s", ErrNode, n.Kind, name)
	}
	if err := json.Unmarshal(value, target); err != nil {
		return fmt.Errorf("%w: %s %s: %w", ErrNode, n.Kind, name, err)
	}

	return nil
}
