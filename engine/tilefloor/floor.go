// Package tilefloor is a tile floor: a rectangular grid with a ground tile and
// at most one tile on top in each cell, plus the anchors that mark named points
// on it.
//
// A floor is one level of a zone. Richer world structure (areas, placements)
// arrives as games need it.
package tilefloor

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

// FormatName identifies tile floors inside saved envelopes.
const FormatName = "tilefloor"

// FormatVersion is the tile floor format version this package writes.
// Version 2 split cells into ground and top layers and added anchors.
const FormatVersion = 2

// MaxSide bounds a floor's width and height.
const MaxSide = 1024

// MaxNameLength bounds an anchor name, in bytes.
const MaxNameLength = 200

// ErrSize reports invalid floor dimensions.
var ErrSize = errors.New("invalid floor size")

// ErrBounds reports coordinates outside the floor.
var ErrBounds = errors.New("position outside floor")

// ErrData reports saved floor data that does not match its dimensions.
var ErrData = errors.New("invalid floor data")

// ErrAnchor reports an invalid anchor change.
var ErrAnchor = errors.New("invalid anchor")

// ErrNoAnchor reports an anchor ID the floor does not hold.
var ErrNoAnchor = errors.New("no such anchor")

// Anchor is a single named point on a floor that other content refers to.
type Anchor struct {
	ID   ids.ULID `json:"id"`
	Name string   `json:"name"`
	Col  int      `json:"col"`
	Row  int      `json:"row"`
}

// AnchorRef refers to an anchor: the floor that holds it and its ID.
type AnchorRef struct {
	Floor  ids.ULID `json:"floor"`
	Anchor ids.ULID `json:"anchor"`
}

// IsZero reports whether the reference points at nothing.
func (r AnchorRef) IsZero() bool { return r.Floor.IsZero() && r.Anchor.IsZero() }

// Floor is a grid of tile IDs in row-major order, one ground layer and one top
// layer. The empty ID is no tile.
type Floor struct {
	width   int
	height  int
	ground  []string
	top     []string
	anchors []Anchor
}

type wireFloor struct {
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	Ground  []string `json:"ground"`
	Top     []string `json:"top"`
	Anchors []Anchor `json:"anchors"`
}

// New returns a floor of the given size with every cell's ground set to one
// tile and nothing on top.
func New(width, height int, ground string) (*Floor, error) {
	if width < 1 || height < 1 || width > MaxSide || height > MaxSide {
		return nil, fmt.Errorf("%w: %dx%d", ErrSize, width, height)
	}
	cells := make([]string, width*height)
	for index := range cells {
		cells[index] = ground
	}

	return &Floor{width: width, height: height, ground: cells, top: make([]string, width*height), anchors: []Anchor{}}, nil
}

// Width returns the number of columns.
func (f *Floor) Width() int { return f.width }

// Height returns the number of rows.
func (f *Floor) Height() int { return f.height }

// In reports whether a position is on the floor.
func (f *Floor) In(col, row int) bool {
	return col >= 0 && row >= 0 && col < f.width && row < f.height
}

// Ground returns the ground tile at a position.
func (f *Floor) Ground(col, row int) (string, error) {
	return f.at(f.ground, col, row)
}

// Top returns the tile on top at a position; empty means nothing.
func (f *Floor) Top(col, row int) (string, error) {
	return f.at(f.top, col, row)
}

// SetGround changes the ground tile at a position and returns the previous one.
func (f *Floor) SetGround(col, row int, tile string) (string, error) {
	return f.set(f.ground, col, row, tile)
}

// SetTop changes the tile on top at a position and returns the previous one.
func (f *Floor) SetTop(col, row int, tile string) (string, error) {
	return f.set(f.top, col, row, tile)
}

// Joins returns which neighbours of a cell hold a tile walls join to. Art for
// a wall family uses it to choose the cell's variant.
func (f *Floor) Joins(col, row int) tiles.Mask {
	joined := func(c, r int) bool {
		if !f.In(c, r) {
			return false
		}

		return tiles.Joins(f.top[r*f.width+c])
	}

	return tiles.MaskOf(joined(col, row-1), joined(col+1, row), joined(col, row+1), joined(col-1, row))
}

// Anchors returns the floor's anchors in their stored order.
func (f *Floor) Anchors() []Anchor {
	return slices.Clone(f.anchors)
}

// Anchor returns an anchor by ID and its position in the stored order.
func (f *Floor) Anchor(id ids.ULID) (Anchor, int, bool) {
	index := slices.IndexFunc(f.anchors, func(anchor Anchor) bool { return anchor.ID == id })
	if index < 0 {
		return noAnchor(), -1, false
	}

	return f.anchors[index], index, true
}

// AnchorAt returns the anchor at a position, if any.
func (f *Floor) AnchorAt(col, row int) (Anchor, bool) {
	for _, anchor := range f.anchors {
		if anchor.Col == col && anchor.Row == row {
			return anchor, true
		}
	}

	return noAnchor(), false
}

func noAnchor() Anchor {
	var none Anchor

	return none
}

// InsertAnchor adds an anchor at a position in the stored order; an index past
// the end appends. IDs and names must be unique on the floor and the anchor
// must lie on it.
func (f *Floor) InsertAnchor(index int, anchor Anchor) error {
	if anchor.ID.IsZero() {
		return fmt.Errorf("%w: missing ID", ErrAnchor)
	}
	if _, _, exists := f.Anchor(anchor.ID); exists {
		return fmt.Errorf("%w: ID %s already used", ErrAnchor, anchor.ID)
	}
	if err := f.checkAnchor(anchor, ids.ULID{}); err != nil {
		return err
	}
	index = min(max(index, 0), len(f.anchors))
	f.anchors = slices.Insert(f.anchors, index, anchor)

	return nil
}

// RemoveAnchor deletes an anchor and returns it with its former position.
func (f *Floor) RemoveAnchor(id ids.ULID) (Anchor, int, error) {
	anchor, index, ok := f.Anchor(id)
	if !ok {
		return noAnchor(), -1, fmt.Errorf("%w: %s", ErrNoAnchor, id)
	}
	f.anchors = slices.Delete(f.anchors, index, index+1)

	return anchor, index, nil
}

// UpdateAnchor changes an anchor's name and position, returning its previous
// state.
func (f *Floor) UpdateAnchor(changed Anchor) (Anchor, error) {
	previous, index, ok := f.Anchor(changed.ID)
	if !ok {
		return noAnchor(), fmt.Errorf("%w: %s", ErrNoAnchor, changed.ID)
	}
	if err := f.checkAnchor(changed, changed.ID); err != nil {
		return noAnchor(), err
	}
	f.anchors[index] = changed

	return previous, nil
}

// checkAnchor validates an anchor's name and position, ignoring the anchor
// with the given ID when checking that names are unique.
func (f *Floor) checkAnchor(anchor Anchor, self ids.ULID) error {
	name := strings.TrimSpace(anchor.Name)
	if name == "" || name != anchor.Name || len(name) > MaxNameLength {
		return fmt.Errorf("%w: name %q", ErrAnchor, anchor.Name)
	}
	if !f.In(anchor.Col, anchor.Row) {
		return fmt.Errorf("%w: (%d, %d) is outside the floor", ErrAnchor, anchor.Col, anchor.Row)
	}
	for _, other := range f.anchors {
		if other.ID != self && strings.EqualFold(other.Name, name) {
			return fmt.Errorf("%w: another anchor is already called %q", ErrAnchor, name)
		}
	}

	return nil
}

// Clone returns an independent copy.
func (f *Floor) Clone() *Floor {
	return &Floor{
		width: f.width, height: f.height,
		ground: slices.Clone(f.ground), top: slices.Clone(f.top), anchors: slices.Clone(f.anchors),
	}
}

// Equal reports whether two floors hold the same size, tiles and anchors.
func (f *Floor) Equal(other *Floor) bool {
	return f.width == other.width && f.height == other.height &&
		slices.Equal(f.ground, other.ground) && slices.Equal(f.top, other.top) && slices.Equal(f.anchors, other.anchors)
}

// Encode returns the floor's saved data.
func (f *Floor) Encode() (json.RawMessage, error) {
	data, err := json.Marshal(wireFloor{
		Width: f.width, Height: f.height, Ground: f.ground, Top: f.top, Anchors: f.anchors,
	})
	if err != nil {
		return nil, fmt.Errorf("encode floor: %w", err)
	}

	return data, nil
}

// Decode reads saved floor data in the current format.
func Decode(data json.RawMessage) (*Floor, error) {
	var wire wireFloor
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	floor, err := New(wire.Width, wire.Height, "")
	if err != nil {
		return nil, err
	}
	cells := wire.Width * wire.Height
	if len(wire.Ground) != cells || len(wire.Top) != cells {
		return nil, fmt.Errorf("%w: %d ground and %d top cells for %dx%d",
			ErrData, len(wire.Ground), len(wire.Top), wire.Width, wire.Height)
	}
	copy(floor.ground, wire.Ground)
	copy(floor.top, wire.Top)
	for _, anchor := range wire.Anchors {
		if err := floor.InsertAnchor(len(floor.anchors), anchor); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrData, err)
		}
	}

	return floor, nil
}

// Format returns the tile floor format registration.
func Format() content.Format {
	return content.Format{
		Name:       FormatName,
		Current:    FormatVersion,
		Migrations: map[int]content.Migration{1: upgradeFrom1},
	}
}

// wireFloor1 is version 1: one tile per cell and no anchors.
type wireFloor1 struct {
	Width  int      `json:"width"`
	Height int      `json:"height"`
	Tiles  []string `json:"tiles"`
}

// upgradeFrom1 moves each cell's tile to the ground layer. Version 1 walls
// become walls with no family on top of plain floor.
func upgradeFrom1(data json.RawMessage) (json.RawMessage, error) {
	var old wireFloor1
	if err := json.Unmarshal(data, &old); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	upgraded := wireFloor{
		Width: old.Width, Height: old.Height,
		Ground: make([]string, len(old.Tiles)), Top: make([]string, len(old.Tiles)), Anchors: []Anchor{},
	}
	for index, tile := range old.Tiles {
		upgraded.Ground[index] = tile
		if tile == tiles.UnassignedWall {
			upgraded.Ground[index] = "floor"
			upgraded.Top[index] = tiles.UnassignedWall
		}
	}
	encoded, err := json.Marshal(upgraded)
	if err != nil {
		return nil, fmt.Errorf("upgrade floor: %w", err)
	}

	return encoded, nil
}

func (f *Floor) at(layer []string, col, row int) (string, error) {
	index, err := f.index(col, row)
	if err != nil {
		return "", err
	}

	return layer[index], nil
}

func (f *Floor) set(layer []string, col, row int, tile string) (string, error) {
	index, err := f.index(col, row)
	if err != nil {
		return "", err
	}
	previous := layer[index]
	layer[index] = tile

	return previous, nil
}

func (f *Floor) index(col, row int) (int, error) {
	if !f.In(col, row) {
		return 0, fmt.Errorf("%w: (%d, %d) on %dx%d", ErrBounds, col, row, f.width, f.height)
	}

	return row*f.width + col, nil
}
