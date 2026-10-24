# Vocabulary

One term per concept. Use these terms in code, documents and the editor's
interface, and do not introduce synonyms.

## Spatial terms, from the top down

| Term | Meaning |
| --- | --- |
| **Network** | The travel layer between zones: nodes on a map, where the space between them is not simulated. |
| **Zone** | The live unit. Floors and areas within a zone are simulated together, and the entities in it are alive. Leaving a zone is a departure. A whole city, including its interiors, can be one zone. |
| **Floor** | A zone's vertical level (its "z"). |
| **Area** | An authored, named region, defined only when something needs to refer to it ("wander in area X"). Areas can be indoors or outdoors, may overlap, and can sit inside other areas. A kind (for example building or room) may add detail. |
| **Anchor** | A single named point that other content refers to, such as a destination or a place to stand. |
| **Tile** | One grid cell. |
| **Zone origin** | Cell 0,0, the fixed point every floor of a zone is positioned from. Columns count to the right and rows downwards; cells left of or above it have negative numbers. |
| **Floor bounds** | The rectangle of cells a floor covers, around the zone origin. Resizing a floor changes its bounds. |
| **Placement** | An object or NPC put on a map. One definition can have many placements. |

Retired synonyms: "meta map" and "major map area" (use zone), "site" and "space"
(use area), "instance" (use placement), "vertical slice" (use floor).

## Boundaries

- **Zones** are the hard boundary: entities outside the current zone are not
  simulated. When an entity leaves the simulation, what happens to the objects it
  carries is decided by those objects' own properties.
- **Floors** are hard boundaries for vertical position. Stairs connect floors
  within the same live zone; a connection (such as an elevator) can be flagged to
  teleport instead.
- **Areas** are not engine boundaries. They are regions that logic asks about
  ("is the player in area X?"), for example for trespassing or wandering.
- Lighting follows geometry (walls, openings), not areas.

## Storage

- **Anchors** live inside the floor document they mark, each with its own
  permanent ID. A reference to an anchor names the floor and the anchor.
- A placement's presence can depend on logic, so one NPC can have placements in
  several zones and appear in whichever applies; only one zone is simulated at a
  time.

## Workflow terms

| Term | Meaning |
| --- | --- |
| **Workflow** | Behaviour authored as typed nodes and the connections between them, starting at its start node |
| **Node** | One step of a workflow; its kind declares its parameters and outputs |
| **Output** | One way out of a node, connected to the node it leads to; an output with nothing connected ends the branch |
| **Entry** | One link coming into a node, shown as its own dot on the node's left edge; any number of links can come in. ("Input" is kept for the values passed into a reusable workflow.) |
| **Time-bound container** | A group of nodes that repeats while its time lasts, then leaves by its exit |

## Editor terms

| Term | Meaning |
| --- | --- |
| **Orchestrator** | The editor's headless central process, the only writer of project data |
| **Window process** | One editor window, each its own process |
| **Command** / **Event** | A request to change project data / the notice that it changed |
| **Edit** | One kind of change, implemented by a plug-in, that knows how to undo itself |
| **Project data** / **Session state** | Saved, undoable content / live, unsaved, never-undone state |
| **Project index** | The orchestrator's list of every document; creating, renaming and deleting documents are edits on it |
| **Stand-in** | A typed placeholder tile (ground, wall, door, stairs, pickup) used until real art exists |
| **Wall family** | A kind of wall (for example "old brick"); art for a family supplies one variant per neighbour arrangement |
| **Ground** / **on top** | The two layers of a cell: its ground tile, and at most one tile sitting on it |
| **Draft** | A text field's unsaved text; Save, Cmd+S, or Enter in a one-line field saves it as an ordinary edit |
| **Save bar** | The strip at the foot of an inspector, or of the Text window, that says how many changes are unsaved and holds Revert and Save |
| **Section** | A group of one document type in the project browser, such as Zones or NPCs, with a count and a [+] |
| **On top** | A window kept above every other window, set with the toggle at the right of its status bar; session state for each window |
| **Clock window** | The preview's clock and its controls, on top by default; before a preview it sets the time the next one starts at |
| **Keys sheet** | The list of a window's keys, opened with its Keys button or ? |
| **Pick** | Choosing something in a Map window for another window, through the orchestrator |
| **Preview** | Playing a zone inside the editor, run by the Map window that started it (D15); it never changes the project |
| **Run** | One play of a zone from its start; a preview is a run, and each has its own ID |
