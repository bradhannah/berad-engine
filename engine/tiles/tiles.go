// Package tiles defines what a tile is and the built-in stand-ins: typed
// placeholder tiles that let an author lay out a map before its art exists.
//
// Stand-ins are a fixed set for now. Editable definitions arrive with the
// definition editor; until then a floor stores stand-in IDs.
package tiles

import "slices"

// Layer is where a tile sits on a cell. Every cell has a ground tile and at
// most one tile on top of it.
type Layer int

// Layers, bottom to top.
const (
	LayerGround Layer = iota + 1
	LayerTop
)

// Category is a tile's role, which a stand-in makes recognisable on the map.
type Category string

// Categories of stand-in.
const (
	CategoryGround       Category = "ground"
	CategoryWall         Category = "wall"
	CategoryDoor         Category = "door"
	CategoryPickupSmall  Category = "pickup-small"
	CategoryPickupMedium Category = "pickup-medium"
)

// Definition describes one kind of tile. For walls, each definition is a wall
// family: art for the family supplies one variant per Mask value.
type Definition struct {
	// ID is permanent and is what floors store.
	ID string
	// Name is the label shown to authors.
	Name     string
	Layer    Layer
	Category Category
	// Color is the stand-in colour, 0xRRGGBB.
	Color       uint32
	Passable    bool
	BlocksSight bool
	Gettable    bool
}

// IsWall reports whether the tile is a wall of some family.
func (d Definition) IsWall() bool { return d.Category == CategoryWall }

// Joins reports whether walls connect to this tile when their variants are
// chosen. Doors sit in wall runs, so walls join them too.
func (d Definition) Joins() bool {
	return d.Category == CategoryWall || d.Category == CategoryDoor
}

// UnassignedWall is the wall family a wall has until the author chooses one.
const UnassignedWall = "wall"

// Stand-in colours.
const (
	colorGrass    = 0x8fc06a
	colorFloor    = 0xe8dcc4
	colorRoad     = 0xa8aba4
	colorWater    = 0x5c8cc8
	colorWall     = 0x4a4f58
	colorWallA    = 0x9c4a3c
	colorWallB    = 0x6f7f96
	colorWallC    = 0x8a6a45
	colorDoor     = 0xc58f3d
	colorPickupS  = 0xd9a400
	colorPickupM  = 0x3f8f8a
	colorUnknown  = 0xc9cbc5
	colorNoGround = 0xffffff
)

// StandIns returns the built-in stand-in definitions in the order the editor
// offers them. An empty ID on the ground layer means unassigned ground.
func StandIns() []Definition {
	return []Definition{
		ground("grass", "Grass", colorGrass, true),
		ground("floor", "Floor", colorFloor, true),
		ground("road", "Road", colorRoad, true),
		ground("water", "Water", colorWater, false),
		wall(UnassignedWall, "Wall (no family)", colorWall),
		wall("wall.a", "Wall family A", colorWallA),
		wall("wall.b", "Wall family B", colorWallB),
		wall("wall.c", "Wall family C", colorWallC),
		{
			ID: "door", Name: "Door", Layer: LayerTop, Category: CategoryDoor, Color: colorDoor,
			Passable: true, BlocksSight: true, Gettable: false,
		},
		pickup("pickup.small", "Small pickup", CategoryPickupSmall, colorPickupS),
		pickup("pickup.medium", "Medium pickup", CategoryPickupMedium, colorPickupM),
	}
}

// Lookup returns a stand-in by ID.
func Lookup(id string) (Definition, bool) {
	all := StandIns()
	index := slices.IndexFunc(all, func(definition Definition) bool { return definition.ID == id })
	if index < 0 {
		var none Definition

		return none, false
	}

	return all[index], true
}

// ColorOf returns the stand-in colour for an ID: white for unassigned ground,
// a neutral grey for an ID no definition has.
func ColorOf(id string) uint32 {
	if id == "" {
		return colorNoGround
	}
	if definition, ok := Lookup(id); ok {
		return definition.Color
	}

	return colorUnknown
}

// WallFamilies returns the wall definitions.
func WallFamilies() []Definition {
	var families []Definition
	for _, definition := range StandIns() {
		if definition.IsWall() {
			families = append(families, definition)
		}
	}

	return families
}

// IsWall reports whether an ID names a wall family.
func IsWall(id string) bool {
	definition, ok := Lookup(id)

	return ok && definition.IsWall()
}

// Joins reports whether walls connect to the tile an ID names.
func Joins(id string) bool {
	definition, ok := Lookup(id)

	return ok && definition.Joins()
}

func ground(id, name string, color uint32, passable bool) Definition {
	return Definition{
		ID: id, Name: name, Layer: LayerGround, Category: CategoryGround, Color: color,
		Passable: passable, BlocksSight: false, Gettable: false,
	}
}

func wall(id, name string, color uint32) Definition {
	return Definition{
		ID: id, Name: name, Layer: LayerTop, Category: CategoryWall, Color: color,
		Passable: false, BlocksSight: true, Gettable: false,
	}
}

func pickup(id, name string, category Category, color uint32) Definition {
	return Definition{
		ID: id, Name: name, Layer: LayerTop, Category: category, Color: color,
		Passable: true, BlocksSight: false, Gettable: true,
	}
}

// Mask records which of a tile's four neighbours it joins. Art for a wall
// family supplies one variant per value, so corners, ends, junctions and
// straight runs are chosen automatically.
type Mask uint8

// Neighbour bits.
const (
	North Mask = 1 << iota
	East
	South
	West
)

// MaskOf builds a mask from the four neighbours.
func MaskOf(north, east, south, west bool) Mask {
	var mask Mask
	for bit, joined := range map[Mask]bool{North: north, East: east, South: south, West: west} {
		if joined {
			mask |= bit
		}
	}

	return mask
}

// Has reports whether the mask joins a neighbour.
func (m Mask) Has(direction Mask) bool { return m&direction != 0 }
