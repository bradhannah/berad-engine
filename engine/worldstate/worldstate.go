// Package worldstate is a project's declared values: what workflows can set
// and check. Every value is declared before it is used, with a name, a type
// and a default, so every reader knows its type and a typo cannot make a
// second value.
//
// A value's scope says whose it is. A world value is one for the whole game,
// declared in the world-state document. An NPC's values and a zone's are its
// own: declared in its own document, or given to it by a template, a named
// set of values declared in the world-state document and applied to any
// number of NPCs or zones, each of which then has its own copy of each value.
// Every NPC also has a name. A local value belongs to one workflow and is
// declared on the workflow itself.
//
// Values are organised in groups, as in world.quests.dragon_slain, which are
// declared like values and hold them. Local values have no groups.
//
// References keep each value's ID, so renaming or regrouping a value changes
// no workflow.
package worldstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// FormatName and FormatVersion identify the world-state document inside
// saved envelopes. Version 2 added groups, templates and the integer, bool
// and text types; version 1 is upgraded to it.
const (
	FormatName    = "world-state"
	FormatVersion = 3
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
	// MaxValues bounds the values and groups one document or workflow
	// declares.
	MaxValues = 1000
	// MaxName bounds a value's name and a choice's name, in characters.
	MaxName = 40
	// MaxDepth bounds how many groups deep a value can be, a template
	// counting as one.
	MaxDepth = 6
	// MaxChoices bounds the choices of a list of choices.
	MaxChoices = 24
	// MaxDescription bounds a description, in characters.
	MaxDescription = 2000
	// MaxInteger bounds an integer, either side of zero: in practice no
	// limit.
	MaxInteger = 1_000_000_000_000_000
	// MaxText bounds a text value, in characters.
	MaxText = 200
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

// Scopes returns the scopes the World state window shows, in its order.
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

// Owned reports whether each NPC or zone has its own values in the scope.
func (s Scope) Owned() bool { return s == ScopeNPC || s == ScopeZone }

// Type is the kind of value a declaration holds, or a group or template
// holding other declarations.
type Type string

// Types. An NPC value names an NPC, or none, and a place value a spot on a
// map, or none; both are for local values only: keeping an NPC in a saved
// value waits until saves are designed, and places elsewhere until they
// are needed. A group
// holds values and other groups; a template, an NPC or zone scope's own kind
// of group, is applied to NPCs or zones.
const (
	TypeInteger  Type = "integer"
	TypeBool     Type = "bool"
	TypeChoice   Type = "choice"
	TypeText     Type = "text"
	TypeNPC      Type = "npc"
	TypePlace    Type = "place"
	TypeGroup    Type = "group"
	TypeTemplate Type = "template"
	// TypeZone, TypeWorkflow and TypeSprite refer to a zone, a workflow or
	// a sprite, by ID; for now only an NPC's built-in values have them.
	TypeZone     Type = "zone"
	TypeWorkflow Type = "workflow"
	TypeSprite   Type = "sprite"
)

// IsRef reports whether a value of the type refers to a zone, a workflow
// or a sprite, kept in Value.Ref.
func (t Type) IsRef() bool { return t == TypeZone || t == TypeWorkflow || t == TypeSprite }

// Types returns the value types a scope's values can have, in the order
// windows offer them.
func Types(scope Scope) []Type {
	if scope == ScopeLocal {
		return []Type{TypeInteger, TypeBool, TypeChoice, TypeText, TypeNPC, TypePlace}
	}

	return []Type{TypeInteger, TypeBool, TypeChoice, TypeText}
}

// IsValue reports whether a declaration of the type holds a value, rather
// than other declarations.
func (t Type) IsValue() bool { return t != TypeGroup && t != TypeTemplate }

// Role is what a workflow's local value is to whatever calls the workflow:
// a local of its own, an input the caller fills or an output handed back.
type Role string

// Roles. Only a local value has one other than RoleLocal.
const (
	RoleLocal  Role = ""
	RoleInput  Role = "input"
	RoleOutput Role = "output"
)

// Roles returns the roles a local value can have, in the order windows
// offer them.
func Roles() []Role { return []Role{RoleLocal, RoleInput, RoleOutput} }

// Label names a role for people.
func (r Role) Label() string {
	switch r {
	case RoleInput:
		return "Input"
	case RoleOutput:
		return "Output"
	case RoleLocal:
	}

	return "Local"
}

// Place is a spot on a map: an anchor of a floor. Zero is none.
type Place struct {
	Floor  ids.ULID `json:"floor"`
	Anchor ids.ULID `json:"anchor"`
}

// IsZero reports a place that is none.
func (p Place) IsZero() bool { return p.Floor.IsZero() && p.Anchor.IsZero() }

// Label names a type for people.
func (t Type) Label() string {
	if t == TypeNPC {
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

// SplitName splits a dotted name, such as quests.dragon_slain, into the
// names of its groups and its own name, checking each.
func SplitName(dotted string) ([]string, error) {
	names := strings.Split(dotted, ".")
	if len(names) > MaxDepth+1 {
		return nil, fmt.Errorf("%w: %q: at most %d groups deep", ErrValue, dotted, MaxDepth)
	}
	for _, name := range names {
		if err := CheckName(name); err != nil {
			return nil, err
		}
	}

	return names, nil
}

// SuggestName suggests a name from a display name, such as an NPC's or a
// zone's path name: lower case, with each run of other characters an
// underscore, so Old Tom becomes old_tom. A name with no letters gives the
// fallback, and one starting with a digit has it first.
func SuggestName(display, fallback string) string {
	var built strings.Builder
	gap := false
	for _, char := range strings.ToLower(display) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			if gap && built.Len() > 0 {
				built.WriteByte('_')
			}
			built.WriteRune(char)
			gap = false

			continue
		}
		gap = true
	}
	name := built.String()
	if name == "" {
		return fallback
	}
	if name[0] >= '0' && name[0] <= '9' {
		name = fallback + "_" + name
	}
	if len(name) > MaxName {
		name = strings.TrimRight(name[:MaxName], "_")
	}

	return name
}

// UniqueName returns a name not in taken: the suggestion, or it with a
// number added.
func UniqueName(suggestion string, taken map[string]bool) string {
	if !taken[suggestion] {
		return suggestion
	}
	for number := 2; ; number++ {
		suffix := "_" + strconv.Itoa(number)
		base := suggestion
		if len(base)+len(suffix) > MaxName {
			base = base[:MaxName-len(suffix)]
		}
		if candidate := base + suffix; !taken[candidate] {
			return candidate
		}
	}
}

// Choice is one value of a list of choices. Its ID stays the same when it is
// renamed, so what workflows set and check keeps working.
type Choice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Declaration declares one value, group or template.
type Declaration struct {
	ID    ids.ULID `json:"id"`
	Name  string   `json:"name"`
	Scope Scope    `json:"scope"`
	Type  Type     `json:"type"`
	// Parent is the group or template it is in; zero at the top.
	Parent ids.ULID `json:"parent,omitzero"`
	// Choices are a list of choices' values, in order.
	Choices []Choice `json:"choices,omitempty"`
	// Min and Max are an integer's limits, if it has them.
	Min *int `json:"min,omitempty"`
	Max *int `json:"max,omitempty"`
	// Default is the value it holds before anything sets it, as Encode
	// writes it; a group or template has none.
	Default     json.RawMessage `json:"default,omitempty"`
	Description string          `json:"description,omitempty"`
	// Role is a local value's role to a caller of its workflow.
	Role Role `json:"role,omitempty"`
}

// Clone returns an independent copy.
func (d Declaration) Clone() Declaration {
	d.Choices = slices.Clone(d.Choices)
	d.Default = slices.Clone(d.Default)
	d.Min, d.Max = cloneLimit(d.Min), cloneLimit(d.Max)

	return d
}

func cloneLimit(limit *int) *int {
	if limit == nil {
		return nil
	}
	copied := *limit

	return &copied
}

// Limit returns an integer limit to keep in a declaration.
func Limit(value int) *int { return &value }

// Equal reports whether two declarations are the same.
func (d Declaration) Equal(other Declaration) bool {
	return d.ID == other.ID && d.Name == other.Name && d.Scope == other.Scope && d.Type == other.Type &&
		d.Parent == other.Parent && slices.Equal(d.Choices, other.Choices) &&
		limitsEqual(d.Min, other.Min) && limitsEqual(d.Max, other.Max) &&
		bytes.Equal(compact(d.Default), compact(other.Default)) && d.Description == other.Description &&
		d.Role == other.Role
}

func limitsEqual(a, b *int) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

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

// Lowest returns the least an integer can be: its minimum, or the engine's.
func (d Declaration) Lowest() int {
	if d.Min != nil {
		return *d.Min
	}

	return -MaxInteger
}

// Highest returns the greatest an integer can be: its maximum, or the
// engine's.
func (d Declaration) Highest() int {
	if d.Max != nil {
		return *d.Max
	}

	return MaxInteger
}

// Hold returns an integer held within the declaration's limits.
func (d Declaration) Hold(number int) int { return min(max(number, d.Lowest()), d.Highest()) }

// Zero returns the value a declaration starts at: its default, or the type's
// own zero when the default cannot be read.
func (d Declaration) Zero() Value {
	value, err := d.Parse(d.Default)
	if err != nil || !d.Fits(value) {
		return d.Empty()
	}

	return value
}

// Empty returns the type's own zero: nought held within its limits, false,
// the first choice, no text or no NPC.
func (d Declaration) Empty() Value {
	empty := NoValue()
	empty.Type = d.Type
	switch d.Type {
	case TypeInteger:
		empty.Integer = d.Hold(0)
	case TypeChoice:
		if len(d.Choices) > 0 {
			empty.Choice = d.Choices[0].ID
		}
	case TypeBool, TypeText, TypeNPC, TypePlace, TypeGroup, TypeTemplate, TypeZone, TypeWorkflow, TypeSprite:
	}

	return empty
}

// New returns a declaration of a type in a scope, at the top, with the
// type's own default; a list starts with two choices.
func New(id ids.ULID, name string, scope Scope, kind Type) Declaration {
	declared := Declaration{
		ID: id, Name: name, Scope: scope, Type: kind, Parent: ids.ULID{}, Choices: nil, Min: nil, Max: nil,
		Default: nil, Description: "", Role: RoleLocal,
	}
	if kind == TypeChoice {
		declared.Choices = []Choice{{ID: "c1", Name: "first"}, {ID: "c2", Name: "second"}}
	}
	if kind.IsValue() {
		declared.Default = declared.Empty().Encode()
	}

	return declared
}

// Check reports a declaration that is not valid on its own; where it sits is
// checked by the list it is in.
func (d Declaration) Check() error {
	if d.ID.IsZero() {
		return fmt.Errorf("%w: a value needs an ID", ErrValue)
	}
	if err := CheckName(d.Name); err != nil {
		return err
	}
	if utf8.RuneCountInString(d.Description) > MaxDescription {
		return fmt.Errorf("%w: a description is at most %d characters", ErrValue, MaxDescription)
	}
	if d.Role != RoleLocal && (d.Scope != ScopeLocal || !slices.Contains(Roles(), d.Role)) {
		return fmt.Errorf("%w: only a local value is an input or an output", ErrValue)
	}
	if !d.Type.IsValue() {
		return d.checkContainer()
	}
	if !slices.Contains(Types(d.Scope), d.Type) {
		return fmt.Errorf("%w: a %s value cannot be a %s", ErrValue, d.Scope.Label(), d.Type.Label())
	}
	if err := d.checkChoices(); err != nil {
		return err
	}
	if err := d.checkLimits(); err != nil {
		return err
	}
	value, err := d.Parse(d.Default)
	if err != nil {
		return fmt.Errorf("%w: default: %w", ErrValue, err)
	}
	if !d.Fits(value) {
		return fmt.Errorf("%w: its default is from %d to %d", ErrValue, d.Lowest(), d.Highest())
	}

	return nil
}

// checkContainer checks a group or template: it holds nothing of its own,
// and a template belongs to the NPC or zone scope.
func (d Declaration) checkContainer() error {
	if len(d.Choices) > 0 || d.Min != nil || d.Max != nil || len(d.Default) > 0 {
		return fmt.Errorf("%w: a %s holds values, not a value of its own", ErrValue, d.Type.Label())
	}
	switch {
	case d.Type == TypeTemplate && !d.Scope.Owned():
		return fmt.Errorf("%w: templates are for NPCs and zones", ErrValue)
	case d.Type == TypeGroup && d.Scope == ScopeLocal:
		return fmt.Errorf("%w: local values have no groups", ErrValue)
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

func (d Declaration) checkLimits() error {
	if d.Type != TypeInteger {
		if d.Min != nil || d.Max != nil {
			return fmt.Errorf("%w: only an integer has limits", ErrValue)
		}

		return nil
	}
	for _, limit := range []*int{d.Min, d.Max} {
		if limit != nil && (*limit > MaxInteger || *limit < -MaxInteger) {
			return fmt.Errorf("%w: a limit is from %d to %d", ErrValue, -MaxInteger, MaxInteger)
		}
	}
	if d.Lowest() > d.Highest() {
		return fmt.Errorf("%w: the minimum is more than the maximum", ErrValue)
	}

	return nil
}

// Fits reports whether a value is one of a declaration's: its type, for an
// integer within its limits, and for a list one of its choices.
func (d Declaration) Fits(value Value) bool {
	if value.Type != d.Type {
		return false
	}
	switch d.Type {
	case TypeInteger:
		return value.Integer >= d.Lowest() && value.Integer <= d.Highest()
	case TypeChoice:
		_, found := d.Choice(value.Choice)

		return found
	case TypeBool, TypeText, TypeNPC, TypePlace, TypeGroup, TypeTemplate, TypeZone, TypeWorkflow, TypeSprite:
	}

	return true
}

// AtMost reports whether an integer is at its maximum; one with no maximum
// never is.
func (d Declaration) AtMost(value Value) bool {
	return d.Type == TypeInteger && d.Max != nil && value.Integer >= *d.Max
}

// AtLeast reports whether an integer is at its minimum; one with no minimum
// never is.
func (d Declaration) AtLeast(value Value) bool {
	return d.Type == TypeInteger && d.Min != nil && value.Integer <= *d.Min
}

// Value is one value as it is now.
type Value struct {
	Type    Type
	Integer int
	Bool    bool
	// Choice is a choice's ID.
	Choice string
	Text   string
	// NPC is an NPC's ID; zero is none.
	NPC ids.ULID
	// Place is a spot on a map; zero is none.
	Place Place
	// Ref is the zone, workflow or sprite a value of those types refers
	// to; zero is none.
	Ref ids.ULID
}

// NoValue returns a value of no type.
func NoValue() Value {
	return Value{Type: "", Integer: 0, Bool: false, Choice: "", Text: "", NPC: ids.ULID{},
		Place: Place{Floor: ids.ULID{}, Anchor: ids.ULID{}}, Ref: ids.ULID{},
	}
}

// Equal reports whether two values are the same.
func (v Value) Equal(other Value) bool { return v == other }

// Encode writes a value as saved: an integer, true or false, a choice's ID,
// text, an NPC's ID with "" for none, or a place's floor and anchor.
func (v Value) Encode() json.RawMessage {
	var encoded []byte
	switch v.Type {
	case TypeInteger:
		encoded = []byte(strconv.Itoa(v.Integer))
	case TypeBool:
		encoded = []byte(strconv.FormatBool(v.Bool))
	case TypeChoice:
		encoded = quoted(v.Choice)
	case TypeText:
		encoded = quoted(v.Text)
	case TypeNPC:
		encoded = quotedID(v.NPC)
	case TypeZone, TypeWorkflow, TypeSprite:
		encoded = quotedID(v.Ref)
	case TypePlace:
		encoded = encodePlace(v.Place)
	case TypeGroup, TypeTemplate:
	}

	return encoded
}

// quotedID writes an ID in quotes, or "" for none.
func quotedID(id ids.ULID) []byte {
	if id.IsZero() {
		return quoted("")
	}

	return quoted(id.String())
}

// encodePlace writes a place as its floor and anchor.
func encodePlace(place Place) []byte {
	data, err := json.Marshal(place)
	if err != nil {
		return []byte("null")
	}

	return data
}

func quoted(text string) []byte {
	data, err := json.Marshal(text)
	if err != nil {
		return []byte(`""`)
	}

	return data
}

// Parse reads a saved value of the declaration's type. An integer is read
// within the engine's bounds; whether it is within the declaration's own
// limits is for Fits.
func (d Declaration) Parse(raw json.RawMessage) (Value, error) {
	value := NoValue()
	value.Type = d.Type
	if err := d.parseInto(raw, &value); err != nil {
		return value, fmt.Errorf("%w: %s: %w", ErrValue, d.Name, err)
	}

	return value, nil
}

// parseInto reads a saved value into the field its type keeps it in.
//
//nolint:wrapcheck // Parse wraps what this returns.
func (d Declaration) parseInto(raw json.RawMessage, value *Value) error {
	switch d.Type {
	case TypeInteger:
		return parseInteger(raw, &value.Integer)
	case TypeBool:
		return json.Unmarshal(raw, &value.Bool)
	case TypeChoice:
		if err := json.Unmarshal(raw, &value.Choice); err != nil {
			return err
		}
		if _, found := d.Choice(value.Choice); !found {
			return fmt.Errorf("%w: %q is not one of its choices", ErrValue, value.Choice)
		}
	case TypeText:
		return parseText(raw, &value.Text)
	case TypeNPC:
		return parseNPC(raw, &value.NPC)
	case TypeZone, TypeWorkflow, TypeSprite:
		return parseNPC(raw, &value.Ref)
	case TypePlace:
		return json.Unmarshal(raw, &value.Place)
	case TypeGroup, TypeTemplate:
		return fmt.Errorf("%w: a %s has no value", ErrValue, d.Type.Label())
	default:
		return fmt.Errorf("%w: unknown type %q", ErrValue, d.Type)
	}

	return nil
}

//nolint:wrapcheck // Parse wraps what this returns.
func parseInteger(raw json.RawMessage, integer *int) error {
	if err := json.Unmarshal(raw, integer); err != nil {
		return err
	}
	if *integer > MaxInteger || *integer < -MaxInteger {
		return fmt.Errorf("%w: an integer is from %d to %d", ErrValue, -MaxInteger, MaxInteger)
	}

	return nil
}

//nolint:wrapcheck // Parse wraps what this returns.
func parseText(raw json.RawMessage, text *string) error {
	if err := json.Unmarshal(raw, text); err != nil {
		return err
	}
	if utf8.RuneCountInString(*text) > MaxText {
		return fmt.Errorf("%w: text is at most %d characters", ErrValue, MaxText)
	}

	return nil
}

// parseNPC reads an NPC's ID, or another document's, or "" for none.
//
//nolint:wrapcheck // Parse wraps what this returns.
func parseNPC(raw json.RawMessage, npc *ids.ULID) error {
	var saved string
	if err := json.Unmarshal(raw, &saved); err != nil || saved == "" {
		return err
	}
	parsed, err := ids.Parse(saved)
	*npc = parsed

	return err
}

// labelNone is how a value that refers to nothing reads.
const labelNone = "none"

// Label shows a value for people: 12, true, a choice's name, text in
// quotes, or an NPC's, a zone's, a workflow's or a sprite's ID; Text shows
// it with names.
func (d Declaration) Label(value Value) string {
	switch d.Type {
	case TypeInteger:
		return strconv.Itoa(value.Integer)
	case TypeBool:
		return strconv.FormatBool(value.Bool)
	case TypeChoice:
		if choice, found := d.Choice(value.Choice); found {
			return choice.Name
		}
	case TypeText:
		return string(quoted(value.Text))
	case TypeNPC:
		if value.NPC.IsZero() {
			return labelNone
		}

		return value.NPC.String()
	case TypePlace:
		if value.Place.IsZero() {
			return "no place yet"
		}

		return "a place"
	case TypeZone, TypeWorkflow, TypeSprite:
		if value.Ref.IsZero() {
			return labelNone
		}

		return value.Ref.String()
	case TypeGroup, TypeTemplate:
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

// nameID is the ID of every NPC's name value. Its time part is zero, so no
// generated ID ever equals it.
const nameID = "0000000000000000000000NAME"

// NameID returns the ID of every NPC's name value.
func NameID() ids.ULID { return ids.MustParse(nameID) }

// NameDeclaration returns an NPC's name value: text that starts as its
// display name, which workflows may set, such as to a name the player
// learns.
func NameDeclaration(display string) Declaration {
	declared := New(NameID(), "name", ScopeNPC, TypeText)
	if utf8.RuneCountInString(display) > MaxText {
		display = string([]rune(display)[:MaxText])
	}
	declared.Default = quoted(display)
	declared.Description = "The NPC's name. It starts as its name in the project browser."

	return declared
}

// Document is a project's world state: the world's values and their groups,
// the NPC and zone templates, and the signals workflows send each other.
type Document struct {
	Values  *Declarations
	Signals []Signal
}

// NewDocument returns a world state that declares nothing yet.
func NewDocument() *Document {
	return &Document{Values: NewDeclarations(HoldsWorldState), Signals: nil}
}

type wireDocument struct {
	Values  json.RawMessage `json:"values"`
	Signals []Signal        `json:"signals,omitempty"`
}

// Encode returns the saved data.
func (d *Document) Encode() (json.RawMessage, error) {
	values, err := d.Values.MarshalJSON()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(wireDocument{Values: values, Signals: d.Signals})
	if err != nil {
		return nil, fmt.Errorf("encode world state: %w", err)
	}

	return data, nil
}

// Clone returns an independent copy.
func (d *Document) Clone() *Document {
	return &Document{Values: d.Values.Clone(), Signals: slices.Clone(d.Signals)}
}

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
	if err := CheckSignals(wire.Signals); err != nil {
		return nil, err
	}
	decoded.Signals = wire.Signals

	return decoded, nil
}

// Format returns the world-state format registration.
func Format() content.Format {
	return content.Format{Name: FormatName, Current: FormatVersion, Migrations: map[int]content.Migration{
		1: upgradeFrom1, 2: upgradeFrom2,
	}}
}
