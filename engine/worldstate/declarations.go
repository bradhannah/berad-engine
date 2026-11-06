package worldstate

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Holds says what a list of declarations is for, which decides what it may
// declare.
type Holds int

const (
	// HoldsWorldState is the world-state document's list: the world's values
	// and groups, and NPC and zone templates with the values and groups in
	// them.
	HoldsWorldState Holds = iota
	// HoldsNPC is one NPC's own values and groups.
	HoldsNPC
	// HoldsZone is one zone's own values and groups.
	HoldsZone
	// HoldsLocals is a workflow's local values, which have no groups.
	HoldsLocals
)

// allows reports why a list cannot hold a declaration, or nil. Where in the
// list it sits is checked with the rest of the list.
func (h Holds) allows(declared Declaration) error {
	var allowed bool
	switch h {
	case HoldsWorldState:
		allowed = declared.Scope == ScopeWorld || declared.Scope.Owned()
	case HoldsNPC:
		allowed = declared.Scope == ScopeNPC && declared.Type != TypeTemplate
	case HoldsZone:
		allowed = declared.Scope == ScopeZone && declared.Type != TypeTemplate
	case HoldsLocals:
		allowed = declared.Scope == ScopeLocal && declared.IsValue()
	}
	if !allowed {
		return fmt.Errorf("%w: a %s %s cannot be declared here", ErrValue, declared.Scope.Label(), declared.Type.Label())
	}

	return nil
}

// Declarations is a list of declared values, groups and templates, kept in
// ID order, which is the order they were made in. The world-state document
// holds one, so does each NPC and zone for its own values, and each
// workflow for its local values.
type Declarations struct {
	holds  Holds
	values []Declaration
}

// NewDeclarations returns an empty list for what it holds.
func NewDeclarations(holds Holds) *Declarations {
	return &Declarations{holds: holds, values: nil}
}

// Holds returns what the list is for.
func (l *Declarations) Holds() Holds { return l.holds }

// Values returns copies of the declarations, in ID order.
func (l *Declarations) Values() []Declaration {
	copies := make([]Declaration, 0, len(l.values))
	for _, declared := range l.values {
		copies = append(copies, declared.Clone())
	}

	return copies
}

// Len counts the declarations.
func (l *Declarations) Len() int { return len(l.values) }

// Value returns a declaration by ID.
func (l *Declarations) Value(id ids.ULID) (Declaration, bool) {
	index := l.index(id)
	if index < 0 {
		return New(ids.ULID{}, "", "", ""), false
	}

	return l.values[index].Clone(), true
}

// Insert adds a declaration.
func (l *Declarations) Insert(declared Declaration) error {
	return l.InsertAll([]Declaration{declared})
}

// InsertAll adds declarations, all or none, such as a group with what it
// held, put back by undo.
func (l *Declarations) InsertAll(declared []Declaration) error {
	working := slices.Clone(l.values)
	for _, each := range declared {
		if slices.ContainsFunc(working, func(held Declaration) bool { return held.ID == each.ID }) {
			return fmt.Errorf("%w: %s is already declared", ErrValue, each.ID)
		}
		working = append(working, each.Clone())
	}
	if err := l.check(working); err != nil {
		return err
	}
	l.values = working
	l.sort()

	return nil
}

// Remove takes out a declaration holding nothing and returns it.
func (l *Declarations) Remove(id ids.ULID) (Declaration, error) {
	removed, err := l.RemoveAll([]ids.ULID{id})
	if err != nil {
		return New(ids.ULID{}, "", "", ""), err
	}

	return removed[0], nil
}

// RemoveAll takes declarations out, all or none, and returns them in the
// order asked. What is left may not be in a group or template that went.
func (l *Declarations) RemoveAll(removing []ids.ULID) ([]Declaration, error) {
	removed := make([]Declaration, 0, len(removing))
	working := slices.Clone(l.values)
	for _, id := range removing {
		index := slices.IndexFunc(working, func(held Declaration) bool { return held.ID == id })
		if index < 0 {
			return nil, fmt.Errorf("%w: no value %s", ErrValue, id)
		}
		removed = append(removed, working[index].Clone())
		working = slices.Delete(working, index, index+1)
	}
	if err := l.check(working); err != nil {
		return nil, fmt.Errorf("%w: remove what it holds with it", err)
	}
	l.values = working

	return removed, nil
}

// Replace changes a declaration, keeping its ID, and returns it as it was.
func (l *Declarations) Replace(declared Declaration) (Declaration, error) {
	index := l.index(declared.ID)
	if index < 0 {
		return New(ids.ULID{}, "", "", ""), fmt.Errorf("%w: no value %s", ErrValue, declared.ID)
	}
	working := slices.Clone(l.values)
	previous := working[index]
	working[index] = declared.Clone()
	if err := l.check(working); err != nil {
		return previous, err
	}
	l.values = working

	return previous.Clone(), nil
}

// Clone returns an independent copy.
func (l *Declarations) Clone() *Declarations {
	return &Declarations{holds: l.holds, values: l.Values()}
}

// Equal reports whether two lists declare the same.
func (l *Declarations) Equal(other *Declarations) bool {
	return l.holds == other.holds && slices.EqualFunc(l.values, other.values, Declaration.Equal)
}

// MarshalJSON writes the declarations as a list.
func (l *Declarations) MarshalJSON() ([]byte, error) {
	values := l.values
	if values == nil {
		values = []Declaration{}
	}
	data, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode values: %w", err)
	}

	return data, nil
}

// Read reads a saved list of declarations.
func (l *Declarations) Read(data json.RawMessage) error {
	var values []Declaration
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("%w: %w", ErrData, err)
	}
	if err := l.check(values); err != nil {
		return fmt.Errorf("%w: %w", ErrData, err)
	}
	l.values = values
	l.sort()

	return nil
}

// Children returns what a group or template holds, or with a zero parent
// what is at the top of a scope, in name order.
func (l *Declarations) Children(scope Scope, parent ids.ULID) []Declaration {
	var children []Declaration
	for _, declared := range l.values {
		if declared.Parent == parent && (!parent.IsZero() || declared.Scope == scope) {
			children = append(children, declared.Clone())
		}
	}
	slices.SortFunc(children, func(a, b Declaration) int { return cmp.Compare(a.Name, b.Name) })

	return children
}

// Within returns everything a group or template holds, at any depth.
func (l *Declarations) Within(id ids.ULID) []ids.ULID {
	var within []ids.ULID
	for _, declared := range l.values {
		if declared.ID != id && l.isWithin(declared.ID, id) {
			within = append(within, declared.ID)
		}
	}

	return within
}

func (l *Declarations) isWithin(inner, container ids.ULID) bool {
	for range MaxDepth + 1 {
		declared, ok := l.Value(inner)
		if !ok || declared.Parent.IsZero() {
			return false
		}
		if declared.Parent == container {
			return true
		}
		inner = declared.Parent
	}

	return false
}

// Find returns what a group or template holds by name, or with a zero
// parent what is at the top of a scope.
func (l *Declarations) Find(scope Scope, parent ids.ULID, name string) (Declaration, bool) {
	for _, declared := range l.Children(scope, parent) {
		if declared.Name == name {
			return declared, true
		}
	}

	return New(ids.ULID{}, "", "", ""), false
}

// Names returns a declaration's name and its groups', from the top: a
// template's value starts with the template's name, as in
// guards.how_many_arrested.
func (l *Declarations) Names(named ids.ULID) []string {
	var names []string
	for range MaxDepth + 1 {
		declared, ok := l.Value(named)
		if !ok {
			break
		}
		names = append(names, declared.Name)
		if declared.Parent.IsZero() {
			break
		}
		named = declared.Parent
	}
	slices.Reverse(names)

	return names
}

// Path is how a workflow names a declaration for its own NPC, zone or
// world, such as world.quests.dragon_slain or npc.guards.how_many_arrested.
func (l *Declarations) Path(named ids.ULID) string {
	declared, ok := l.Value(named)
	if !ok {
		return ""
	}

	return string(declared.Scope) + "." + strings.Join(l.Names(named), ".")
}

// Template returns the template a declaration is in, if any.
func (l *Declarations) Template(inner ids.ULID) (ids.ULID, bool) {
	for range MaxDepth + 1 {
		declared, ok := l.Value(inner)
		if !ok {
			return ids.ULID{}, false
		}
		if declared.Type == TypeTemplate {
			return declared.ID, true
		}
		inner = declared.Parent
	}

	return ids.ULID{}, false
}

// sibling identifies a name among what one group holds, or what is at the
// top of one scope.
type sibling struct {
	scope  Scope
	parent ids.ULID
	name   string
}

// check enforces what every list keeps: valid declarations of what it
// holds with unique IDs; groups and templates as parents in the same scope,
// templates only at the top and no deeper than MaxDepth; an NPC or zone
// value in the world state only inside a template; and names unique among
// what one group holds.
func (l *Declarations) check(values []Declaration) error {
	if len(values) > MaxValues {
		return fmt.Errorf("%w: at most %d values", ErrValue, MaxValues)
	}
	byID := make(map[ids.ULID]Declaration, len(values))
	for _, declared := range values {
		if err := declared.Check(); err != nil {
			return err
		}
		if err := l.holds.allows(declared); err != nil {
			return err
		}
		if _, seen := byID[declared.ID]; seen {
			return fmt.Errorf("%w: %s is declared twice", ErrValue, declared.ID)
		}
		byID[declared.ID] = declared
	}
	names := map[sibling]bool{}
	for _, declared := range values {
		if err := l.checkPlace(declared, byID); err != nil {
			return err
		}
		key := sibling{scope: declared.Scope, parent: declared.Parent, name: declared.Name}
		if names[key] {
			return fmt.Errorf("%w: %s is already declared there", ErrValue, declared.Name)
		}
		names[key] = true
	}

	return nil
}

// checkPlace checks where a declaration sits: under groups or a template in
// its own scope, no deeper than MaxDepth.
func (l *Declarations) checkPlace(declared Declaration, byID map[ids.ULID]Declaration) error {
	if declared.Type == TypeTemplate && !declared.Parent.IsZero() {
		return fmt.Errorf("%w: a template is not in a group", ErrValue)
	}
	inTemplate := declared.Type == TypeTemplate
	child := declared
	for depth := 0; !child.Parent.IsZero(); depth++ {
		parent, found := byID[child.Parent]
		switch {
		case depth >= MaxDepth:
			return fmt.Errorf("%w: %s is more than %d groups deep", ErrValue, declared.Name, MaxDepth)
		case !found || parent.Type.IsValue():
			return fmt.Errorf("%w: %s is not in a group", ErrValue, declared.Name)
		case parent.Scope != declared.Scope:
			return fmt.Errorf("%w: %s is in another scope's group", ErrValue, declared.Name)
		}
		inTemplate = inTemplate || parent.Type == TypeTemplate
		child = parent
	}
	if l.holds == HoldsWorldState && declared.Scope.Owned() && !inTemplate {
		return fmt.Errorf("%w: an %s value here belongs in a template", ErrValue, declared.Scope.Label())
	}

	return nil
}

func (l *Declarations) sort() {
	slices.SortFunc(l.values, func(a, b Declaration) int { return a.ID.Compare(b.ID) })
}

func (l *Declarations) index(id ids.ULID) int {
	return slices.IndexFunc(l.values, func(declared Declaration) bool { return declared.ID == id })
}

// Owned is what one NPC or zone keeps of its values, in its own document:
// the values, constants and groups declared for it alone, the templates
// applied to it, and its own value of any of their constants, as saved. A
// value is kept while its template is off, and counts again when it goes
// back on; one for a constant no longer declared is ignored, so Undo brings
// both back.
type Owned struct {
	Values    *Declarations
	Templates []ids.ULID
	Constants map[ids.ULID]json.RawMessage
}

// NewOwned returns nothing owned yet, for an NPC or a zone.
func NewOwned(scope Scope) Owned {
	holds := HoldsNPC
	if scope == ScopeZone {
		holds = HoldsZone
	}

	return Owned{Values: NewDeclarations(holds), Templates: nil, Constants: nil}
}

// Clone returns an independent copy.
func (o Owned) Clone() Owned {
	copied := Owned{Values: nil, Templates: slices.Clone(o.Templates), Constants: nil}
	for id, raw := range o.Constants {
		copied.SetConstant(id, raw)
	}
	if o.Values != nil {
		copied.Values = o.Values.Clone()
	}

	return copied
}

// Len counts what is declared, applied and set.
func (o Owned) Len() int {
	count := len(o.Templates) + len(o.Constants)
	if o.Values != nil {
		count += o.Values.Len()
	}

	return count
}

// Equal reports whether two owners keep the same; nothing declared equals
// an empty list.
func (o Owned) Equal(other Owned) bool {
	if o.Values == nil || other.Values == nil {
		return o.Len() == 0 && other.Len() == 0
	}

	return o.Values.Equal(other.Values) && slices.Equal(o.Templates, other.Templates) &&
		maps.EqualFunc(o.Constants, other.Constants, func(a, b json.RawMessage) bool {
			return bytes.Equal(compact(a), compact(b))
		})
}

// SetConstant keeps the owner's own value of a constant, as saved.
func (o *Owned) SetConstant(constant ids.ULID, raw json.RawMessage) {
	if o.Constants == nil {
		o.Constants = map[ids.ULID]json.RawMessage{}
	}
	o.Constants[constant] = slices.Clone(raw)
}

// ClearConstant forgets the owner's own value of a constant, so it has the
// default again, and returns what it was.
func (o *Owned) ClearConstant(constant ids.ULID) (json.RawMessage, bool) {
	raw, had := o.Constants[constant]
	delete(o.Constants, constant)
	if len(o.Constants) == 0 {
		o.Constants = nil
	}

	return raw, had
}

// ConstantValue returns the owner's own value of a constant, when it has
// one that fits.
func (o Owned) ConstantValue(declared Declaration) (Value, bool) {
	raw, had := o.Constants[declared.ID]
	if !had {
		return NoValue(), false
	}
	value, err := declared.Parse(raw)
	if err != nil || !declared.Fits(value) {
		return NoValue(), false
	}

	return value, true
}

// Apply applies a template, at the end of the list.
func (o *Owned) Apply(template ids.ULID) error {
	_, err := o.ApplyAt(template, len(o.Templates))

	return err
}

// ApplyAt applies a template at a place in the list, past the end to append,
// and returns where it went.
func (o *Owned) ApplyAt(template ids.ULID, index int) (int, error) {
	if template.IsZero() || slices.Contains(o.Templates, template) {
		return -1, fmt.Errorf("%w: template %s is missing or already applied", ErrValue, template)
	}
	if len(o.Templates) >= MaxValues {
		return -1, fmt.Errorf("%w: at most %d templates", ErrValue, MaxValues)
	}
	index = min(max(index, 0), len(o.Templates))
	o.Templates = slices.Insert(slices.Clone(o.Templates), index, template)

	return index, nil
}

// Unapply takes a template off and returns where it was.
func (o *Owned) Unapply(template ids.ULID) (int, error) {
	index := slices.Index(o.Templates, template)
	if index < 0 {
		return -1, fmt.Errorf("%w: template %s is not applied", ErrValue, template)
	}
	o.Templates = slices.Delete(slices.Clone(o.Templates), index, index+1)

	return index, nil
}

// WireOwned is how an NPC or zone document saves what it owns, both left
// out while empty.
type WireOwned struct {
	Values    json.RawMessage              `json:"values,omitempty"`
	Templates []ids.ULID                   `json:"templates,omitempty"`
	Constants map[ids.ULID]json.RawMessage `json:"constants,omitempty"`
}

// Wire returns what an NPC or zone document saves of what it owns.
func (o Owned) Wire() (WireOwned, error) {
	wire := WireOwned{Values: nil, Templates: o.Templates, Constants: o.Constants}
	if o.Values != nil && o.Values.Len() > 0 {
		values, err := o.Values.MarshalJSON()
		if err != nil {
			return wire, err
		}
		wire.Values = values
	}

	return wire, nil
}

// ReadOwned reads what an NPC or zone owns, as saved.
func ReadOwned(scope Scope, wire WireOwned) (Owned, error) {
	owned := NewOwned(scope)
	if len(wire.Values) > 0 {
		if err := owned.Values.Read(wire.Values); err != nil {
			return owned, err
		}
	}
	for _, template := range wire.Templates {
		if err := owned.Apply(template); err != nil {
			return owned, fmt.Errorf("%w: %w", ErrData, err)
		}
	}
	if len(wire.Constants) > MaxValues {
		return owned, fmt.Errorf("%w: at most %d constants of its own", ErrData, MaxValues)
	}
	for constant, raw := range wire.Constants {
		if constant.IsZero() || !json.Valid(raw) {
			return owned, fmt.Errorf("%w: a constant of its own needs an ID and a value", ErrData)
		}
		owned.SetConstant(constant, raw)
	}

	return owned, nil
}
