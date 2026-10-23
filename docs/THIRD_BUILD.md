# Third build: NPC workflows

The third implementation step towards the
[first useful authoring milestone](EDITOR_FEATURES.md#first-useful-authoring-milestone):
authoring NPC behaviour as workflows, placing NPCs on the map, and watching
them follow their workflows in the preview while the player walks around. It
also gives every editor window the look of the editor wireframes. It is held
to the same strict checks as [the first build](FIRST_BUILD.md).

## What it proves

1. **Behaviour is authored, not coded:** a daily routine ("get ready in AM, go
   to work, work, come home, eat, go to bed") can be built from typed nodes in
   the Workflow window, without new engine code.
2. **The editor builds its controls from node declarations:** an anchor
   parameter gets "Pick on map", a time range gets time fields, and so on.
3. **NPCs follow their workflows in the preview,** on the game clock, and the
   Workflow window shows where each NPC is.
4. **Editing during a preview applies at once:** NPCs using an edited
   workflow restart it.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The wireframe look | IBM Plex Sans and Mono; dark 1.5-pixel outlines and rounded boxes; small uppercase labels; blue for selection, orange for live; the dotted or lined canvas | Every window matches the wireframes' look, judged by its author | Built |
| 2. Workflows | Workflow documents, logic and layout kept apart (D17); node types declared by a plug-in with typed parameters; the Workflow window: palette, canvas, nodes, connections, containers, notes, an inspector built from the declarations, auto-arrange | A routine can be built, edited and undone in the Workflow window | Built |
| 3. NPCs | NPC documents with a behaviour workflow; placing NPCs on floors; the inspector for an NPC placement | NPCs can be placed and given workflows | Planned |
| 4. NPCs in the preview | The engine's workflow runner; NPC walking with berad-path; the preview clock; live state in the Map and Workflow windows | NPCs follow a daily routine in the preview, judged by its author | Planned |

Pathfinding comes from a new berad-sdk module, **berad-path**, built from
Ultima V Redux's A*; berad-engine pins it once it is merged.

## Decisions

Settled by its author before building:

- Every editor window takes the look of the wireframes, not only the new
  Workflow window.
- The build goes as far as NPCs following their workflows in the preview.
- The first node types: **Go to** (an anchor), **Wait**, **Time switch**,
  **Random choice** and the **time-bound container**, plus a start node and
  notes.
- When the flow reaches the end of a branch with nothing connected after it,
  it goes **back to the start**, so a time switch there picks the branch for
  the time now. Inside a time-bound container, flow repeats the container's
  contents until its time is up.
- When a container's time is up, the NPC **leaves at once** by the
  container's exit, even partway through a node.
- When the clock jumps, each NPC **restarts its workflow** from the start and
  **walks** to wherever its new branch sends it.
- Pathfinding lives in berad-sdk as berad-path, built from Ultima V Redux's
  A* after a review found the search generic once its map and unit types are
  removed.

Open:

- How fast game time runs in the finished game has not been discussed. The
  preview clock shows its speed as a rate (one game minute per second by
  default, real time, or ten game minutes per second) rather than calling one
  of them normal.

## Step results

- **The look:** IBM Plex Sans and Mono, under the SIL Open Font License in
  `editor/ui/fonts`, replace Go Regular. Buttons, fields, lists, toggles and
  dialogs are rounded boxes with 1.5-pixel ink outlines; headings are small
  spaced uppercase labels; ink lines divide the panes; selected rows are
  washed blue with a blue outline; the speed readout is monospaced; anchors
  are blue rings with blue names; the player is a white token with a blue
  ring; preview labels are orange. Layouts are unchanged: the Map window
  keeps its tool list on the left rather than the wireframe's toolbar.
- The new font exposed a caret fault: at the start of a wrapped line,
  Cmd+Left and Cmd+Right treated the caret as being on the line above. The
  caret now belongs to the line it is drawn on, and a test that does not
  depend on the font checks it.
- Windows driven by test scripts open without taking focus, so the
  real-window tests no longer take the keyboard from whoever is at the
  computer.

- **Workflows:** `engine/workflow` holds the document and the built-in node
  kinds, each declaring typed parameters (anchor, minutes, percentage, time
  range, times, text) and its outputs. The workflow plug-in's four edits (add
  nodes, remove nodes, replace a node, move nodes) pass the shared undo test,
  which now covers layouts too. A workflow's layout is saved in
  `<name>--<id>.layout.json` beside its logic, with no revision line, so
  moving a node never touches the logic file; the layout is read with its
  document, follows renames and deletions, travels in snapshots, and comes
  back when a deletion is undone. Document types can prepare a new document,
  so a new workflow's start node gets its permanent ID from the orchestrator
  (D4).
- **The Workflow window,** opened from the project browser (New workflow,
  then Open): the palette on the left groups the node kinds (Flow, Movement,
  Action, Annotate). Clicking an entry adds a node after the selected one,
  connected from its first free output, or as the first step inside a
  selected container; dragging an entry places it. Dragging from a port to a
  node connects them, dropping on empty canvas disconnects, and links that
  break the rules are refused with a reason. Nodes move by dragging, with a
  container's contents moving with it and staying inside it; a container's
  corner resizes it. Each free output says where the flow goes: "→ start", or
  "→ repeat" inside a container. The inspector is built from the node kind's
  declarations: number and text fields saved with Enter, time fields, a list
  of times with Add and Remove, and an anchor with Pick on map, through the
  pick component the Text window now shares. Auto-arrange lays the workflow
  out in columns from the start, leaving placed notes where they are. Views
  of one workflow share undo.
- Chosen while building, for its author to check: the start node is the hub,
  so there is no separate Hub node yet; new nodes go to the right of the node
  they follow, and notes above it; zoom, the minimap and search shown in the
  wireframe are not built yet.

## Not in this build

Reusable workflows with input parameters, workflow inheritance, nested
containers, conversations, objects and placements other than NPCs, areas, the
Clock window (the preview's clock controls live in the Map window for now),
and the Problems view.
