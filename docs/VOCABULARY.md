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
| **Cell** | One grid position on a floor. Every map coordinate names a cell. |
| **Zone origin** | Cell 0,0, the fixed point every floor of a zone is positioned from. Columns count to the right and rows downwards; cells left of or above it have negative numbers. |
| **Floor bounds** | The rectangle of cells a floor covers, around the zone origin. Resizing a floor changes its bounds. |
| **Placement** | An object or NPC put on a map. One definition can have many placements. |

Retired synonyms: "meta map" and "major map area" (use zone), "site" and "space"
(use area), "instance" (use placement), "vertical slice" (use floor), "tile"
for a grid position (use cell).

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

## Art terms

| Term | Meaning |
| --- | --- |
| **Tile** | One cell-sized piece of art, drawn in the pixel editor; the game sets its size |
| **Sprite** | What a definition shows: one or more tiles. A static sprite has one frame; a sprite bigger than one cell has a grid of tiles in each frame |
| **Frame** | One picture of a sprite, a tile or a grid of tiles, shown for its own duration |
| **Sequence** | A sprite's frames played in order; when it ends, weighted choices pick the sequence that plays next |
| **Alternative** | One of a sprite's interchangeable tiles, such as one of three grasses; each cell picks one by weight from its position |
| **Linked tile** | A tile shared by several places, so a change to it follows everywhere; reuse copies a tile unless it is pasted linked |
| **Base row** | The row of a sprite where it touches the ground; what stands on the map is drawn in order of base row, lower on screen in front |
| **Overlay** | A sprite drawn over a definition's own, shown when a state or flag is set, such as moss |
| **Palette** | A document of named colours in ramps that art is painted with: Default, built in and read only, and a game's own palettes, such as "gloomy" |
| **Default** | The engine's built-in palette of 256 colours, the same in every game; its colours are copied, never changed |
| **Ramp** | A palette's named list of colours for one use, numbered dark to light, such as `default / grass 2` |
| **Label** | A colour's optional short name, at most 15 characters, unique within its palette |
| **Gradient** | A tool that fills a ramp from a left colour, a right colour and a number of steps |
| **Loose colour** | A pixel's colour that names no palette, left when a palette is removed from a tile; flagged until repainted |
| **Opacity** | How see-through a palette colour is, such as glass at 40%; one colour is fully clear |
| **Template** | Guides to draw a tile to, and parts of it kept clear, shown while drawing and never part of the tile |
| **Drawing layer** | One layer of a tile while it is painted, such as shading; the game receives the layers flattened into one tile |

Retired synonyms: "cel" and "raw tile" (use tile). The engine's `tiles` package
was named before these terms; it holds definitions and stand-ins, not art.

## Workflow terms

| Term | Meaning |
| --- | --- |
| **Workflow** | Behaviour authored as typed nodes and the connections between them, starting at its start node |
| **Node** | One step of a workflow; its kind declares its parameters and outputs |
| **Output** | One way out of a node, connected to the node it leads to; an output with nothing connected ends the branch |
| **Entry** | One link coming into a node, shown as its own dot on the node's left edge; any number of links can come in. ("Input" is kept for the values passed into a reusable workflow.) |
| **Time-bound container** | A group of nodes that repeats while its time lasts, then leaves by its exit |

## Sound terms

| Term | Meaning |
| --- | --- |
| **Audio library** | A project's sounds, its cues and recordings, kept in berad-audio's own formats in the project's `audio` folder |
| **Cue** | A sound something asks for by its ID, such as `door/open`; it plays one recording, or cycles through several |
| **Recording** | One sound file of the audio library, which cues play; an intentional silent slot has none |
| **Category** | Whether a cue is music, SFX or ambient |
| **Audio window** | The window where sounds are added and cues edited |
| **Attribution** | Who made a recording, where it came from and what kind of source that was, its licence and its processing steps, as its author recorded them |
| **Processing step** | One thing done to a recording before it came into the project, such as trimming it, with the tool used and who did it |
| **Cleared** | A recording its author has marked as allowed in a commercial game; until then it is not cleared |
| **Region** | The part of a recording that plays, from its start to its end; all of it when none is set |
| **Waveform** | A picture of how loud a recording is from moment to moment |
| **Gain** | How loud a cue, or a layer of the soundscape, plays, as a percentage; a mixing choice |
| **Level** | How much louder or quieter a recording plays than as recorded, in dB, so recordings sound equally loud; every cue that plays it gets it. It is measured to reach its target, set by hand, or not set |
| **Target** | The loudness a measured recording's level aims at; its family's unless its author sets one |
| **Acoustic family** | A group of recordings that sound alike, which sets the target measuring aims at |
| **Profile** | What kind of moment a cue plays for, such as a door starting to open, which a search finds it by |
| **Distance** | How many cells away an ambient cue is heard, louder closer; 0 is everywhere, as a background |
| **Soundscape** | Up to four cues the Audio window plays together, two ambient, one music and one SFX, to hear how they sit together; never saved |
| **Layer** | One cue of the soundscape, with its own gain, mute and solo |
| **Monitor level** | How loud the Audio window plays a category while its author listens, standing in for a player's volume settings; never saved |

## Editor terms

| Term | Meaning |
| --- | --- |
| **Orchestrator** | The editor's headless central process, the only writer of project data |
| **Window process** | One editor window, each its own process |
| **Command** / **Event** | A request to change project data / the notice that it changed |
| **Edit** | One kind of change, implemented by a plug-in, that knows how to undo itself |
| **Project data** / **Session state** | Saved, undoable content / live, unsaved, never-undone state |
| **Project index** | The orchestrator's list of every document; creating, renaming and deleting documents are edits on it |
| **Stand-in** | A typed placeholder (ground, wall, door, stairs, pickup), drawn in a flat colour until it has a sprite |
| **Wall family** | A kind of wall (for example "old brick"); a family supplies one sprite, its variant, per neighbour arrangement |
| **Ground** / **on top** | The two layers of a cell: its ground, and at most one thing standing on it |
| **Draft** | A text field's unsaved text; Save, Cmd+S, or Enter in a one-line field saves it as an ordinary edit |
| **Save bar** | The strip at the foot of an inspector, or of the Text window, that says how many changes are unsaved and holds Revert and Save |
| **Section** | A group of one document type in the project browser, such as Zones or NPCs, with a count and a [+] |
| **On top** | A window kept above every other window, set with the toggle at the right of its status bar; session state for each window |
| **Clock window** | The preview's clock and its controls, on top by default; before a preview it sets the time the next one starts at |
| **Keys sheet** | The list of a window's keys, opened with its Keys button or ? |
| **Folder** | A folder in a section of the project browser, holding documents of that type and other folders; a real folder on disk |
| **Tag** | A word an author gives a document to find it by, such as "town"; tags group documents across types and mean nothing to a game |
| **Search** | The project browser's box that shows only the documents whose name, folder or tag holds what is typed; "#town" looks at tags only |
| **Pick** | Choosing something in a Map window for another window, through the orchestrator |
| **Preview** | Playing a zone inside the editor, run by the Map window that started it (D15); it never changes the project |
| **Run** | One play of a zone from its start; a preview is a run, and each has its own ID |

## Working terms

| Term | Meaning |
| --- | --- |
| **Increment** | One planned piece of work on the engine and editor, with its own record in `docs/increments/`: what it proves, its decisions, its steps and their results |
| **Increment status** | Planned until an increment's steps are built; Built once they are and the checks pass; Judged once its author has tried it and accepted it |
