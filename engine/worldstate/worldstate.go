// Package worldstate is a project's declared values: what workflows can set
// and check. Every value is declared before it is used, with a name, a type,
// a scope and a default, so every reader knows its type and a typo cannot
// make a second value.
//
// A value's scope says whose it is. A world value is one for the whole game,
// a zone value one in each zone, and an NPC value one on each NPC, kept with
// the NPC wherever it is placed. A local value belongs to one workflow and is
// declared on the workflow itself, not here. A declaration covers its whole
// scope: declaring mood for NPCs gives every NPC its own mood.
//
// References keep each value's ID, so renaming a value changes no workflow.
package worldstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// FormatName and FormatVersion identify the world-state document inside
// saved envelopes.
const (
	FormatName    = "world-state"
	FormatVersion = 1
)

// Name is the world-state document's name in every project.
const Name = "World state"

// worldStateID is the world-state document's fixed ID. Its time part is
// zero, so no generated ID ever equals it.
const worldStateID = "00000000000000000000000021"

// ID returns the ID of a project's world-state document.
func ID() ids.ULID { return ids.MustParse(worldStateID) }

// Limits on declarations.
const (
	// MaxValues bounds the values one document or workflow declares.
	MaxValues = 1000
	// MaxName bounds a value's name and a choice's name, in characters.
	MaxName = 40
	// MaxChoices bounds the choices of a list of choices.
	MaxChoices = 24
	// MaxDescription bounds a value's description, in characters.
	MaxDescription = 2000
	// MaxNumber bounds a number, either side of zero.
	MaxNumber = 1_000_000_000
)

// Scope says whose a value is.
type Scope string

// Scopes, as paths spell them.
const (
	ScopeLocal Scope = "local"
	ScopeNPC   Scope = "npc"
	ScopeZone  Scope = "zone"
	ScopeWorld Scope = "world"
)

// Scopes returns the scopes the world-state document declares values in, in
// the order windows list them.
func Scopes() []Scope { return []Scope{ScopeWorld, ScopeZone, ScopeNPC} }

// Label names a scope for people.
func (s Scope) Label() string {
	switch s {
	case ScopeLocal:
		return "Local"
	case ScopeNPC:
		return "NPC"
	case ScopeZone:
		return "Zone"
	case ScopeWorld:
		return "World"
	}

	return string(s)
}

// Type is the kind of value a declaration holds.
type Type string

// Types. An NPC value names an NPC, or none, and is for local values only:
// keeping an NPC in a saved value waits until saves are designed.
const (
	TypeNumber Type = "number"
	TypeYesNo  Type = "yes-no"
	TypeChoice Type = "choice"
	TypeNPC    Type = "npc"
)

// Types returns the types a scope's values can have, in the order windows
// offer them.
func Types(scope Scope) []Type {
	if scope == ScopeLocal {
		return []Type{TypeNumber, TypeYesNo, TypeChoice, TypeNPC}
	}

	return []Type{TypeNumber, TypeYesNo, TypeChoice}
}

// Label names a type for people.
func (t Type) Label() string {
	switch t {
	case TypeNumber:
		return "number"
	case TypeYesNo:
		return "yes or no"
	case TypeChoice:
		return "choice"
	case TypeNPC:
		return "NPC"
	}

	return string(t)
}

// Errors.
var (
	// ErrValue reports an invalid declaration or change to one.
	ErrValue = errors.New("invalid value")
	// ErrData reports saved world state that cannot be read.
	ErrData = errors.New("invalid world-state data")
)

// namePattern is what a name may be: lower-case letters, digits and
// underscores, starting with a letter.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// CheckName reports a name that is not fit for a path, such as george's.
func CheckName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > MaxName || !namePattern.MatchString(name) {
		return fmt.Errorf("%w: %q: a name is up to %d lower-case letters, digits and underscores, starting with a letter",
			ErrValue, name, MaxName)
	}

	return nil
}

// Choice is one value of a list of choices. Its ID stays the same when it is
// renamed, so what workflows set and check keeps working.
type Choice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Declaration declares one value.
type Declaration struct {
	ID    ids.ULID `json:"id"`
	Name  string   `json:"name"`
	Scope Scope    `json:"scope"`
	Type  Type     `json:"type"`
	// Choices are a list of choices' values, in order.
	Choices []Choice `json:"choices,omitempty"`
	// Default is the value it holds before anything sets it, as Encode
	// writes it.
	Default     json.RawMessage `json:"default"`
	Description string          `json:"description,omitempty"`
}

// Clone returns an independent copy.
func (d Declaration) Clone() Declaration {
	d.Choices = slices.Clone(d.Choices)
	d.Default = slices.Clone(d.Default)

	return d
}

// Equal reports whether two declarations are the same.
func (d Declaration) Equal(other Declaration) bool {
	return d.ID == other.ID && d.Name == other.Name && d.Scope == other.Scope && d.Type == other.Type &&
		slices.Equal(d.Choices, other.Choices) && bytes.Equal(compact(d.Default), compact(other.Default)) &&
		d.Description == other.Description
}

// Path is how windows show the value, such as npc.mood.
func (d Declaration) Path() string { return string(d.Scope) + "." + d.Name }

// Choice returns one of a list of choices by ID.
func (d Declaration) Choice(id string) (Choice, bool) {
	index := slices.IndexFunc(d.Choices, func(choice Choice) bool { return choice.ID == id })
	if index < 0 {
		return Choice{ID: "", Name: ""}, false
	}

	return d.Choices[index], true
}

// NewChoiceID returns a choice ID the declaration does not use yet.
func (d Declaration) NewChoiceID() string {
	for number := 1; ; number++ {
		id := "c" + strconv.Itoa(number)
		if _, used := d.Choice(id); !used {
			return id
		}
	}
}

// Zero returns the value a declaration starts at: its default, or the type's
// own zero when the default cannot be read.
func (d Declaration) Zero() Value {
	value, err := d.Parse(d.Default)
	if err != nil {
		return d.Empty()
	}

	return value
}

// Empty returns the type's own zero: nothing, no, the first choice or none.
func (d Declaration) Empty() Value {
	empty := Value{Type: d.Type, Number: 0, Yes: false, Choice: "", NPC: ids.ULID{}}
	if d.Type == TypeChoice && len(d.Choices) > 0 {
		empty.Choice = d.Choices[0].ID
	}

	return empty
}

// New returns a declaration of a type in a scope with the type's own default.
func New(id ids.ULID, name string, scope Scope, kind Type) Declaration {
	declared := Declaration{
		ID: id, Name: name, Scope: scope, Type: kind, Choices: nil, Default: nil, Description: "",
	}
	if kind == TypeChoice {
		declared.Choices = []Choice{{ID: "c1", Name: "first"}, {ID: "c2", Name: "second"}}
	}
	declared.Default = declared.Empty().Encode()

	return declared
}

// Check reports a declaration that is not valid in one of the scopes given.
func (d Declaration) Check(scopes ...Scope) error {
	if d.ID.IsZero() {
		return fmt.Errorf("%w: a value needs an ID", ErrValue)
	}
	if err := CheckName(d.Name); err != nil {
		return err
	}
	if !slices.Contains(scopes, d.Scope) {
		return fmt.Errorf("%w: %s cannot be declared here", ErrValue, d.Scope.Label())
	}
	if !slices.Contains(Types(d.Scope), d.Type) {
		return fmt.Errorf("%w: a %s value cannot be a %s", ErrValue, d.Scope.Label(), d.Type.Label())
	}
	if err := d.checkChoices(); err != nil {
		return err
	}
	if _, err := d.Parse(d.Default); err != nil {
		return fmt.Errorf("%w: default: %w", ErrValue, err)
	}
	if utf8.RuneCountInString(d.Description) > MaxDescription {
		return fmt.Errorf("%w: a description is at most %d characters", ErrValue, MaxDescription)
	}

	return nil
}

func (d Declaration) checkChoices() error {
	if d.Type != TypeChoice {
		if len(d.Choices) > 0 {
			return fmt.Errorf("%w: only a list of choices has choices", ErrValue)
		}

		return nil
	}
	if len(d.Choices) == 0 || len(d.Choices) > MaxChoices {
		return fmt.Errorf("%w: a list has 1 to %d choices", ErrValue, MaxChoices)
	}
	seenIDs, seenNames := map[string]bool{}, map[string]bool{}
	for _, choice := range d.Choices {
		if choice.ID == "" || seenIDs[choice.ID] {
			return fmt.Errorf("%w: each choice needs its own ID", ErrValue)
		}
		if choice.Name == "" || utf8.RuneCountInString(choice.Name) > MaxName || seenNames[choice.Name] {
			return fmt.Errorf("%w: each choice needs its own name of up to %d characters", ErrValue, MaxName)
		}
		seenIDs[choice.ID], seenNames[choice.Name] = true, true
	}

	return nil
}

// Value is one value as it is now.
type Value struct {
	Type   Type
	Number int
	Yes    bool
	// Choice is a choice's ID.
	Choice string
	// NPC is an NPC's ID; zero is none.
	NPC ids.ULID
}

// Equal reports whether two values are the same.
func (v Value) Equal(other Value) bool { return v == other }

// Encode writes a value as saved: a number, true or false, a choice's ID, or
// an NPC's ID with "" for none.
func (v Value) Encode() json.RawMessage {
	var encoded []byte
	switch v.Type {
	case TypeNumber:
		encoded = []byte(strconv.Itoa(v.Number))
	case TypeYesNo:
		encoded = []byte(strconv.FormatBool(v.Yes))
	case TypeChoice:
		encoded = []byte(strconv.Quote(v.Choice))
	case TypeNPC:
		npc := ""
		if !v.NPC.IsZero() {
			npc = v.NPC.String()
		}
		encoded = []byte(strconv.Quote(npc))
	}

	return encoded
}

// Parse reads a saved value of the declaration's type.
func (d Declaration) Parse(raw json.RawMessage) (Value, error) {
	value := Value{Type: d.Type, Number: 0, Yes: false, Choice: "", NPC: ids.ULID{}}
	var err error
	switch d.Type {
	case TypeNumber:
		err = json.Unmarshal(raw, &value.Number)
		if err == nil && (value.Number > MaxNumber || value.Number < -MaxNumber) {
			err = fmt.Errorf("%w: a number is from %d to %d", ErrValue, -MaxNumber, MaxNumber)
		}
	case TypeYesNo:
		err = json.Unmarshal(raw, &value.Yes)
	case TypeChoice:
		err = json.Unmarshal(raw, &value.Choice)
		if _, found := d.Choice(value.Choice); err == nil && !found {
			err = fmt.Errorf("%w: %q is not one of its choices", ErrValue, value.Choice)
		}
	case TypeNPC:
		var npc string
		if err = json.Unmarshal(raw, &npc); err == nil && npc != "" {
			value.NPC, err = ids.Parse(npc)
		}
	default:
		err = fmt.Errorf("%w: unknown type %q", ErrValue, d.Type)
	}
	if err != nil {
		return value, fmt.Errorf("%w: %s: %w", ErrValue, d.Path(), err)
	}

	return value, nil
}

// Label shows a value for people: 12, yes, a choice's name, or an NPC's ID.
func (d Declaration) Label(value Value) string {
	switch d.Type {
	case TypeNumber:
		return strconv.Itoa(value.Number)
	case TypeYesNo:
		if value.Yes {
			return "yes"
		}

		return "no"
	case TypeChoice:
		if choice, found := d.Choice(value.Choice); found {
			return choice.Name
		}

		return "?"
	case TypeNPC:
		if value.NPC.IsZero() {
			return "none"
		}

		return value.NPC.String()
	}

	return "?"
}

// compact returns JSON without spaces, so equal values compare equal.
func compact(raw json.RawMessage) []byte {
	var buffer bytes.Buffer
	if json.Compact(&buffer, raw) != nil {
		return raw
	}

	return buffer.Bytes()
}

// Declarations is a list of declared values kept in ID order, which is the
// order they were made in. The world-state document holds one, and so does
// each workflow for its local values.
type Declarations struct {
	scopes []Scope
	values []Declaration
}

// NewDeclarations returns an empty list for values of the scopes given.
func NewDeclarations(scopes ...Scope) *Declarations {
	return &Declarations{scopes: slices.Clone(scopes), values: nil}
}

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
		return Declaration{
			ID: ids.ULID{}, Name: "", Scope: "", Type: "", Choices: nil, Default: nil, Description: "",
		}, false
	}

	return l.values[index].Clone(), true
}

// Insert adds a declaration.
func (l *Declarations) Insert(declared Declaration) error {
	if l.index(declared.ID) >= 0 {
		return fmt.Errorf("%w: %s is already declared", ErrValue, declared.ID)
	}
	if len(l.values) >= MaxValues {
		return fmt.Errorf("%w: at most %d values", ErrValue, MaxValues)
	}
	working := append(slices.Clone(l.values), declared.Clone())
	if err := l.check(working); err != nil {
		return err
	}
	l.values = working
	l.sort()

	return nil
}

// Remove takes a declaration out and returns it.
func (l *Declarations) Remove(id ids.ULID) (Declaration, error) {
	index := l.index(id)
	if index < 0 {
		return Declaration{
			ID: ids.ULID{}, Name: "", Scope: "", Type: "", Choices: nil, Default: nil, Description: "",
		}, fmt.Errorf("%w: no value %s", ErrValue, id)
	}
	removed := l.values[index]
	l.values = slices.Delete(slices.Clone(l.values), index, index+1)

	return removed, nil
}

// Replace changes a declaration, keeping its ID, and returns it as it was.
func (l *Declarations) Replace(declared Declaration) (Declaration, error) {
	index := l.index(declared.ID)
	if index < 0 {
		return Declaration{
			ID: ids.ULID{}, Name: "", Scope: "", Type: "", Choices: nil, Default: nil, Description: "",
		}, fmt.Errorf("%w: no value %s", ErrValue, declared.ID)
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
	return &Declarations{scopes: slices.Clone(l.scopes), values: l.Values()}
}

// Equal reports whether two lists declare the same values.
func (l *Declarations) Equal(other *Declarations) bool {
	return slices.EqualFunc(l.values, other.values, Declaration.Equal)
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

// Read reads a saved list of declarations for the scopes the list was made
// for.
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

// check enforces what every list keeps: valid declarations with unique IDs,
// and names unique within each scope.
func (l *Declarations) check(values []Declaration) error {
	if len(values) > MaxValues {
		return fmt.Errorf("%w: at most %d values", ErrValue, MaxValues)
	}
	seen, paths := map[ids.ULID]bool{}, map[string]bool{}
	for _, declared := range values {
		if err := declared.Check(l.scopes...); err != nil {
			return err
		}
		if seen[declared.ID] {
			return fmt.Errorf("%w: %s is declared twice", ErrValue, declared.ID)
		}
		if paths[declared.Path()] {
			return fmt.Errorf("%w: %s is already declared", ErrValue, declared.Path())
		}
		seen[declared.ID], paths[declared.Path()] = true, true
	}

	return nil
}

func (l *Declarations) sort() {
	slices.SortFunc(l.values, func(a, b Declaration) int { return a.ID.Compare(b.ID) })
}

func (l *Declarations) index(id ids.ULID) int {
	return slices.IndexFunc(l.values, func(declared Declaration) bool { return declared.ID == id })
}

// Document is a project's world state: the world, zone and NPC values it
// declares.
type Document struct {
	Values *Declarations
}

// NewDocument returns a world state that declares nothing yet.
func NewDocument() *Document {
	return &Document{Values: NewDeclarations(Scopes()...)}
}

type wireDocument struct {
	Values json.RawMessage `json:"values"`
}

// Encode returns the saved data.
func (d *Document) Encode() (json.RawMessage, error) {
	values, err := d.Values.MarshalJSON()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(wireDocument{Values: values})
	if err != nil {
		return nil, fmt.Errorf("encode world state: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
func (d *Document) Clone() *Document { return &Document{Values: d.Values.Clone()} }

// Decode reads saved world state.
func Decode(data json.RawMessage) (*Document, error) {
	var wire wireDocument
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	decoded := NewDocument()
	if len(wire.Values) > 0 {
		if err := decoded.Values.Read(wire.Values); err != nil {
			return nil, err
		}
	}

	return decoded, nil
}

// Format returns the world-state format registration.
func Format() content.Format {
	return content.Format{Name: FormatName, Current: FormatVersion, Migrations: nil}
}
