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
| 3. NPCs | NPC documents with a behaviour workflow; placing NPCs on floors; the inspector for an NPC placement | NPCs can be placed and given workflows | Built |
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

Settled by its author after step 3, for testing behaviour before step 4:

- Each link coming into a node gets its own **entry** dot, in the order of
  the nodes it comes from, top to bottom, and every node that can take a
  link shows a **spare** dot for the next one.
- **Random choice** has any number of options, each with a share; the node
  shows what each share works out to as a percentage, so adding an option
  never breaks the others.
- **Wait** counts seconds, minutes or hours, on the **game clock** or a
  **real clock**, chosen on each Wait. A real-time wait ignores the preview
  clock's pause and speed.
- A new **Move** node goes up, down, left or right by a number of tiles. If
  something is in the way, the NPC **stops there and carries on** with the
  next node.

Settled by its author for the Workflow window's lines:

- Each line turns in its **own lane**, and auto-arrange is **tidy**: it
  leaves loops out of the columns, puts a node after its longest way in,
  orders each column to cut crossings and lines nodes up with the outputs
  they come from.
- The selected node's lines are **lit blue**, as is the line under the
  pointer. **Loops are dashed**, and a time switch's "otherwise" line is
  grey; in step 4 the line an NPC just took will flash orange.
- Lines keep their **arrowhead** at the entry dot and get a **chevron**
  along each long stretch.
- A line can be **selected and deleted**, and **picked up by its arrow
  end**, the entry dot it goes into, and dropped on another node; dropped on
  empty canvas, it is removed. Nodes with a problem get a **mark**.
  Dropping a palette node onto a line to put it into the line was tried and
  taken out as too eager.
- **Moving lines by hand** is decided later. If it comes, a line moved by
  hand keeps its shape between its ends when a node moves, and auto-arrange
  resets it, with undo bringing it back.
- **Later:** zoom with fit and a minimap, node titles and frames that only
  group nodes, and search. The step 4 runner guards against loops that never
  wait.

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

- **NPCs:** `engine/npc` holds the NPC document: the workflow that is its
  behaviour and its private notes. Floors (format 4, upgraded from 3) hold
  NPC placements beside their anchors, each with its own ID, so one NPC can
  be placed many times. The NPC plug-in's edits (choose the behaviour, change
  the notes) and the floor's placement edits (add, move, remove) pass the
  shared undo test, and resizing a floor removes and restores placements the
  way it does anchors. Where-used finds the floors that place an NPC and the
  NPCs that follow a workflow.
- **In the project browser,** New NPC makes one; selecting it shows its
  behaviour, with Choose… (a list of the project's workflows) and Open, and
  its notes, saved with Enter. Opening an NPC opens its behaviour.
- **In the Map window,** the Place NPC tool (N) lists the project's NPCs on
  the left; clicking a cell places the chosen one, dragging a token moves it,
  and Delete or Remove takes it off the map. Tokens are dark discs with the
  NPC's initials and its name beside them; an NPC whose definition was
  deleted shows as a grey "?". Selecting a token shows the NPC, where it
  stands, its behaviour and Open behaviour. A resize that takes NPCs off the
  map says how many.
- Found while building: a window's documents cannot be read from inside
  another read without freezing the window, and the test stand-in allowed it,
  so the first real-window run froze. The stand-in now refuses such reads as
  the real client would, and the window and browser tests now draw.
- Chosen while building, for its author to check: placements live in the
  floor document, like anchors; only one NPC stands on a cell; behaviour and
  notes are edited in the project browser, while the Map window shows them
  and opens the behaviour; NPCs do not yet block the player in the preview,
  which arrives with step 4.

- **Test nodes and entry dots** (settled after step 3): a node draws a dot
  for each link coming in and a spare dot, washed blue while a link being
  drawn is over a node that can take it; a link can be dropped on the dot or
  anywhere on the node. Random choice lists its options with their
  percentages and "+ option" under them; the inspector edits each option's
  name and share, with Add option and Remove, and removing an option removes
  its link. Wait's inspector has the amount and buttons for the unit and the
  clock; Move's has a button for each direction and the tiles. Workflows are
  format 2: a version 1 random choice's chance becomes two options that keep
  its outputs and links ("chance 30" becomes "chance 30%" and "otherwise
  70%"), and a wait's minutes become minutes on the game clock. What Move and
  the clocks do arrives with the runner in step 4. A real-window test builds
  "move up or down at random, wait two seconds, repeat".
- Found while building: nodes added after outputs close together could land
  on top of each other, so a new node now moves down until it is clear. And
  a window left without a script in a real-window test opened like a normal
  window, able to take the keyboard; every window opened while scripts drive
  the editor now opens without taking focus.

- **Lines** (settled after step 3): lines are routed from the links and
  node positions, never saved. Lines leaving one column each turn in their
  own lane, ordered so they cross only where they must; lines going back run
  under the nodes between their ends, shorter ones inside longer ones, and
  rise in their own lanes. Auto-arrange finds loops with a walk from the
  start, gives each node its longest distance from the start as its column,
  orders columns by where their lines come from over five sweeps, lines each
  node up with the output it comes from, and widens gaps for their lanes; it
  breaks ties by node, so it always gives the same layout. The selected
  node's lines and the line under the pointer are blue; loops are dashed; a
  time switch's "otherwise" line is grey; long stretches have a chevron.
  Clicking a line selects it, and Delete or Backspace, or Delete line in the
  inspector, removes it. Pressing a line's filled entry dot picks it up by
  its arrow end: dropped on another node it moves there, leaving the same
  output; dropped back where it was it changes nothing; dropped on empty
  canvas it is removed. While it moves, the line is drawn faint where it
  was. A Go to with no destination
  or a broken one, a container with no first step, and a node nothing links
  into get a warning mark; pointing at it says what is wrong, and so does
  the inspector. The window now lays the workflow out only when it changes,
  not every frame.
- Found while building: the test stand-in for a window's documents never
  raised a document's revision, which the window now relies on, so it does
  when a document changes, as the orchestrator does.

## Not in this build

Reusable workflows with input parameters, workflow inheritance, nested
containers, conversations, objects and placements other than NPCs, areas, the
Clock window (the preview's clock controls live in the Map window for now),
and the Problems view.
