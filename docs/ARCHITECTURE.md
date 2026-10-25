# Editor architecture

How the berad-engine editor is built: the orchestrator, window processes,
protocol, undo, saving and failure handling. Each decision is numbered so code,
tests and discussions can refer to it (for example "D3").

These decisions are settled unless marked open. Change one only deliberately,
and update this document in the same change. Terms follow
[the vocabulary](VOCABULARY.md).

## Established context

- The editor runs as **one Ebitengine process per editor window**, coordinated
  by a central **orchestrator** over IPC. Ebitengine supports one window per
  process ([ebiten#1205](https://github.com/hajimehoshi/ebiten/issues/1205),
  closed as won't fix), and editor windows are real operating-system windows
  rather than panels inside one window. A browser-based editor was rejected.
- The window that starts a preview runs the simulation in-process and publishes
  its live state through the orchestrator.
- SOLID principles are applied deliberately at the seams (views, layouts,
  plug-ins, encodings) so the editor can change without large rewrites.
- A game contains no editor functionality. A playtest of a real game may later
  run as an Ebitengine VM guest inside an editor window.

| Term | Meaning |
| --- | --- |
| **Orchestrator** | The headless central process, one per open project |
| **Window process** | One editor window, each its own Ebitengine process |
| **Command** | A request from a window to change project data |
| **Event** | The orchestrator's notice that something changed |

## D1 — Authority and data flow

The orchestrator is the only writer of project data. Window processes never read
or write project files; they send commands and receive events. Updates are
**strict**: a window shows a change only when the orchestrator's event arrives,
with no optimistic local change or rollback. Strict is the starting point:
optimise for speed, and adapt only where measurements show it is too slow.
Optimistic updates, and local previews for gestures, were considered and not
chosen.

**Text fields:** typing changes only a local draft; pressing Enter saves the field
as one edit, so project data stays strict. In a multi-line field, Cmd+Enter or
Ctrl+J inserts a new line and Enter still saves. *Open:* what happens when focus
leaves a field without saving.

A gesture such as a paint stroke is sent step by step (strict) but grouped into
one undo step (D3).

**As built for audio (protocol version 11):** to add a sound, a window asks
the author for a file of their own with the system's Open dialog and sends
only its path; the orchestrator copies the file into the project through
berad-audio's authoring package. That chosen file is the only file outside
the project the editor reads.

## D2 — Project data versus session state

- **Project data:** authored content. Saved, undoable, written only by the
  orchestrator.
- **Session state:** selection, pick requests, the clock, preview state, open
  windows. Shared live between windows; never saved and never undone.

## D3 — Undo and redo scope

Each window undoes its own recent edits, in order, whatever they touched. Views
of the same map share one undo list: undo in any of them takes back the last
change to that map, whichever view made it. A Map window shows a zone, so the
views of one zone share an undo list across all its floors. An undo is itself a command; the
orchestrator holds the histories.

If something an undo would revert was changed afterwards from a different
window, the undo is refused with a message rather than overwriting that change.
The first build compares individual items (for a floor, each layer of each
cell and each anchor, plus its bounds, which a resize changes along with every
cell it adds or removes). Undoing the creation of a document is refused once
another window has changed that document. *Open:* whether other kinds of
documents need a broader rule.

Closing a window ends its undo list (unless another view shares it). Undo does
not survive quitting the editor; versions (D9) cover older changes. Each undo
list keeps 100 steps by default.

**As built for audio (protocol version 11):** the Audio window has its own
undo list. An audio undo step holds the values its edit changed, before and
after: each cue, recording, classification, ambient setting and category it
touched. An undo is refused when another window changed one of them since,
as above, and also when a program outside the editor did, since the audio
files can be edited elsewhere too. Undoing never deletes a file that was
copied in.

## D4 — Identity and references

Every entity has a permanent **ULID**, assigned by the orchestrator when it is
created. References store IDs; names are labels, so renaming never breaks a
reference. IDs are shown read-only and visually subdued, which helps debugging.

Names are unique within their scope (for example no two anchors with the same
name in one zone). Deleting something that other content references shows where
it is used and lets the author cancel or proceed; proceeding leaves broken
references flagged in a Problems view, and the deletion can be undone.

**How the first build does it:**

- Creating, renaming and deleting documents are edits on the **project index**,
  an in-memory list of every document that the orchestrator rebuilds from the
  files on load. They are journaled, broadcast and undoable like any edit, and
  windows subscribe to the index for names.
- An edit can need values only the orchestrator has. Before applying it, the
  orchestrator **prepares** it: a new document or anchor gets its permanent ID,
  and a deletion gets the document's full contents, so undo recreates exactly
  what was deleted. The prepared edit is what is journaled and broadcast, and
  the result returns the new ID.
- Names are unique per kind of document; a floor's name only within the zone
  that lists it. Anchor names are unique per floor; the zone-wide check waits
  for the Problems view.
- Where-used is a question windows ask the orchestrator, answered from the
  references each document declares. The Problems view is not built yet: a
  broken reference shows as broken where it is displayed.

**Exception (protocol version 10):** a cue parameter stores the cue's ID as
the audio library spells it, such as `door/open`, not a ULID, because the
library keeps berad-audio's own format (D11). A cue whose ID changes outside
the editor shows as missing, as a broken reference does.
**As built (protocol version 11):** a cue's ID is chosen when the cue is made,
in lowercase letters, digits, `_` and `-`, in parts joined by `/`, and the
editor never changes it; its name can change freely. A workflow's Play sound
nodes declare their cues as references to the audio library, which goes by a
fixed ID of its own (`model.AudioLibraryID`), so where-used lists the
workflows that play a cue before it is deleted.

## D5 — What each window holds

A window subscribes only to what it shows (a floor, a workflow and the names of
what it references). Project-wide views subscribe to indexes such as names and
where-used, not full content. No window holds the whole project.

## D6 — Transport and message format

- **Transport:** Unix domain sockets, behind Go's connection interface. Go
  supports Unix domain stream sockets on Windows 10 and later
  ([go#26072](https://golang.org/issue/26072)); named pipes or local TCP could
  be substituted.
- **Encoding:** every message is defined once as Go types in the protocol
  package, encoded through a small codec interface. JSON first, for readability;
  hot paths may move to a binary codec without changing the message types.
- **Shape:** one connection per window carries everything:
  1. a handshake, in which the orchestrator refuses a different build;
  2. a snapshot of what the window shows;
  3. a subscription to its changes;
  4. numbered commands, each answered with a result, and change events with the
     document's new revision number, broadcast to every subscriber;
  5. session messages.

Revision numbers let a window detect a missed event and reload from a fresh
snapshot instead of drifting. A wire tap records all traffic readably; recorded
traffic supports reproducing bugs.

## D7 — High-frequency streams

Live preview state (entity positions, current workflow steps, the clock) travels
as rate-limited "latest value wins" streams through the orchestrator. It is
session state: never journaled, undone or saved. Each update carries a sequence
number, the simulation tick together with a preview-run ID; a receiver discards
anything older than what it holds, and a restarted preview is never mistaken for
an old one. Game-clock jumps are separate from the tick.

**As built (protocol version 3):** three preview messages. *Start* is answered
with the run's ID, and refused while another preview runs. *State* goes from
the window running the preview to the zone's other Map windows; that window
sends at most 30 a second and only when something changed, the orchestrator
keeps the latest for a view that opens later, and a window whose queue is half
full skips a state rather than falling behind. *Stop* comes from the window
running the preview, or from the orchestrator when that window closes. States
are kept out of a crash report's recent traffic, which they would crowd out,
but stay in the wire tap.

**As built (protocol version 5):** a state also carries every NPC (its NPC,
workflow, node, what it is doing, why a node could not do its work, its
latest change of node and a count of changes), the game clock (time of day,
rate, stopped) and the watched NPC. A fourth message, *Control*, asks the
window running the preview to watch an NPC, or pause, resume or step its
NPCs; the orchestrator passes it only to that window, and only from a window
that shows the preview. Watch windows of the zone receive starts, states and
stops as its Map windows do.

**As built (protocol version 7):** a change of node also says why the node it
left carried on without doing its work, if it did, and an NPC's status says
only what keeps it from following its workflow now. A state says whether a
step is running, and carries the watched NPC's changes in the latest step.
The window running the preview runs a step as fast as it can, up to a minute
of ticks a frame, until the watched NPC changes node.

**As built (protocol version 8):** a control can also set the clock's rate,
jump it to a time of day, and stop or start it. Clock windows receive every
preview's starts, states and stops, whatever its zone, and may send
controls; when a preview starts and no Clock window is open, the
orchestrator opens one. The time the next preview starts at is session state
under its own topic, set by the Clock window: the orchestrator keeps it and
gives it to every window, including ones that open later, and until it is
set a preview starts at 08:00.

## D8 — Large data

Art is served by the orchestrator. Windows cache assets by content fingerprint
and are never sent an asset they already hold; change notices carry the new
fingerprint. Pixel edits are patches (the changed rectangle) sent as ordinary
strict, undoable commands, grouped per stroke, and other windows update live as
the author draws. Pixel data is the first candidate for a binary codec.

**As built for audio (protocol version 10):** a window asks the orchestrator
for the audio library, the cues it can play, and for a recording's bytes, at
most 8 MiB a reply, never reading a file itself. Each recording carries a
version made from its file's size and modification time, and a window keeps
the bytes it holds until the version changes. Recordings travel in JSON
first; a window opens its audio device only when it first plays.
**Protocol version 11:** after every change to the audio library, and when the
orchestrator notices its files changed outside the editor, every window is
told the library changed and asks for it again. The library also lists the
recordings no cue plays, and each recording's file, note and attribution.

## D9 — Saving and versions

- **Always saved:** every applied command is written to a journal before it is
  acknowledged. There is no unsaved state and no Save command. The journal is
  folded into the document files continuously.
- **Versions:** the project folder is a git repository; the editor's "create
  version" and "restore version" use it, presented in editor terms. Ordinary git
  tools still work.
- *Open:* how restoring a version interacts with open windows and undo.

**As built (2026-10-23):** text typed into a field stays in its window as a
draft until Save, Cmd+S or Enter in a one-line field sends it as a command;
from then on it is saved like every other command. A draft is never project
data, so the windows, not the orchestrator, ask about unsaved drafts before
they are lost.

**As built for audio (protocol version 11):** an audio edit is written to the
audio files before it is acknowledged, so it is not journaled. Edits run one
at a time on a worker, so copying a large file never holds up the
orchestrator; each opens the library afresh and saves it at once.

## D10 — Lifecycle and failure

- One orchestrator per project, protected by a lock. *Open:* a second launch on
  an open project should join the running editor; the first build refuses it
  with directions instead.
- The orchestrator starts every window process; windows never start each other.
- The editor quits when its last window closes (later: when the project browser
  closes); no orchestrator is left running without windows.
- **As built (protocol version 6):** Cmd+W closes one window; Cmd+Q in any
  window quits the editor. Each window tells the orchestrator how many
  unsaved drafts it holds and what they belong to (*Unsaved*). Cmd+Q asks
  the orchestrator what quitting would close (*Quit ask*), then asks the
  author: Quit or Cancel, or, with unsaved drafts anywhere, which it names,
  Save and quit, Don't save or Cancel. *Quit* tells every window to close,
  saving its drafts first when asked; a window whose drafts cannot be saved
  stays open and says why, and the orchestrator stops when the last window
  has gone. On macOS, Cmd+Q reaches a window as a close request while Cmd is
  held, which is how it is told apart from the close button.
- **Orchestrator failure:** windows get the chance to clean up, then close
  themselves. A hard crash cannot send a signal, so a lost connection also means
  "close". Because the project is always saved, relaunching loses nothing.
- **Window failure:** the orchestrator notices, tells the remaining windows, and
  saves a crash report with the window's output and recent protocol traffic.
- **Restoring windows:** on launch, the editor asks whether to restore the last
  window set. That layout is per-user session data, kept outside the project.

## D11 — On-disk format

Readable JSON with stable key order, one file per document, floors stored
separately so small edits make small diffs, folders mirroring the project
browser, art as PNG. File names combine the name and a short ID
(`floor-1--PJY7525V.json`), so renaming shows as a git rename. Editor state
(journal, backups, logs, lock) lives in a git-ignored `.berad` folder. Files of
deleted, renamed or moved documents move to `.berad/backups/removed` rather
than being destroyed. Two files holding one document, which a crash during a
rename can leave, resolve to the newer one on load. Each kind of document has
its folder (`tilefloors`, `zones`, `texts`, `workflows`, `npcs`).

**As built (protocol version 9):** inside a kind's folder, the project
browser's folders are real folders, so a workflow in Town › Guards is
`workflows/Town/Guards/<name>--<id>.json`, and every directory there is read
as a folder on load, empty ones included (git keeps no empty directory). The
project index lists each kind's folders and each document's folder and tags;
moving a document, changing a folder and setting tags are journaled,
undoable index edits like a rename. Files move at the next save; a folder's
directory goes once nothing but Finder's `.DS_Store` is left in it. A
document's tags are an optional list in its envelope. Floors stay with their
zone and take no folder.
**Exception (protocol version 10):** the project's `audio` folder keeps
berad-audio's own formats rather than the envelope, so the terminal audio
editor opens the same files: `audio/library.json`, an audio library, and
`audio/project.json`, its version-1 descriptor, beside the sound files they
name. Only berad-audio's authoring package reads and writes them, and loading
the project skips the folder.
**As built (protocol version 11):** the Audio window writes them. The editor
starts the library when the first sound is added, writes it in schema 4,
which records whether each recording may be used in a commercial game, and
moves a schema-3 library to schema 4 the first time it changes it. A cue's
category is kept in the descriptor, made when one is first needed. Sounds
added are copied into `audio/imported/`.
A document may keep its layout in a second file beside its own,
`<name>--<id>.layout.json`, whose envelope carries no revision, so the file
changes only when the layout does (D17). A workflow's layout holds where
each node sits and the bends of lines moved by hand. *Open:* chunking very large floors.

## D12 — Testing

- The orchestrator is tested headless, by scripted clients speaking the protocol.
- Every message kind round-trips through every codec.
- Recorded traffic and crash reports become regression tests once fixed.
- Windows are tested with scripted input and self-captured screenshots; window
  behaviour lives in view packages tested without a screen.
- **Latency budget:** edit-to-screen within one frame (about 16 ms at 60 fps),
  with a command round trip well under a millisecond locally, measured
  continuously.

## D13 — Windows working together

- **Inspector:** every window embeds the same inspector component on its right
  side, showing that window's selection; it can collapse, and each window
  remembers that. There are no separate inspector windows.
- **Selection stays local:** selecting in one window changes only that window.
- **Opening something already open** brings its window to the front. A
  process cannot raise another process's window, so the window asks for
  attention instead (on macOS its Dock icon bounces). Opening can also name
  an item to show, such as a cue in the Audio window; the window is told it
  once open, or at once if it is open already (protocol version 11).
- **Several views of one map** are allowed through an explicit "New view"; all
  views show every change and share undo (D3). During preview, every view of the
  map shows the live entities.
- **Pick on map:** a parameter decides what can be picked (a tile, an anchor, an
  area, an object, another NPC). If no Map window is open, the editor asks which
  zone and opens one; with one open, it is used; with several, the editor asks
  which, marking the maps that show the thing being picked. Picking uses four
  messages through the orchestrator (ask for candidate maps, start picking,
  answer, cancel); the requesting window then makes an ordinary edit.
  Contents, as built: *ask* names what to pick and is answered with the open
  Map windows, their zones and whether each holds something pickable; *start*
  names a Map window, or a zone for the orchestrator to open one on, and is
  forwarded to that window; *answer* gives the floor and item picked; *cancel*
  can come from either side. A second pick in the same Map window replaces the
  first, and a window closing cancels its picks.
- *Not built yet:* collapsing the inspector.

## D14 — How edits work

Every change is an **edit**. Each edit kind is a small, self-contained piece of
code that knows what it changes, which documents it touches and how to undo
itself; new kinds are added without changing the orchestrator. The orchestrator
checks every edit and applies it completely or not at all; a refused edit comes
back with a short reason the window shows.

## D15 — Preview

- Start by pressing Preview, then clicking the start tile. One preview at a time.
- State from a preview (items picked up, flags set) is kept when it stops, until
  Reset. Preview never changes the project.
- Editing during preview is allowed; entities using the edited thing restart
  their routine.
- Leaving the zone stops the preview, for now.
- The player's stats and skills come from a chosen test profile, the same
  profiles used for conversation testing.
- The clock runs only during preview.
- Conversations appear over the map, as in a game.
- The world-state view can change flag values for the running preview only.
- *As built:* `P` or the Preview button, then a click on a cell the player can
  stand on, starts a preview in the Map window. The engine's `play` package
  runs it in that window's process (one tick per frame); the arrow keys walk,
  the view follows the player and switches floor when they take stairs, and
  `P` stops it. The other views of the zone draw the player from the stream
  (D7). See [the second build](SECOND_BUILD.md) for the movement rules.
- *Not built yet:* keeping state until Reset (there is none to keep yet), test
  profiles, the clock, conversations and the world-state view.

## D16 — Code layout

- **berad-sdk:** pieces any Ebitengine game can use (audio, graphics, input,
  RNG, text, and later pathfinding).
- **berad-engine:** an `engine` module (what games run) and an `editor` module
  (the orchestrator and windows). The engine never imports the editor; games
  never use the editor.
- **Each game:** its content, settings (tile size, base resolution) and its own
  game modes, such as an intro. The engine runs the game loop and lets a game
  start and stop its own modes.
- **Grouping:** the framework is organised by layer; features (walls, doors,
  schedules, conversation nodes) are plug-ins that register edits, node types
  and inspector sections.
- **One shared editor program** for every game. Games extend it through data
  (reusable workflows, characteristics defined as data, presets); new abilities
  go into berad-engine for all games, since a shared Go program cannot load a
  game's code. *Open:* previewing a game's own modes, perhaps by running the
  game as a VM guest.

## D17 — Rules from the first-pass review

From the [first-pass review](FIRST_PASS_REVIEW.md):

- **Format versions from day one:** every saved file and journal entry carries a
  format version; old files are backed up, then upgraded step by step on load;
  files are written to a temporary file and renamed into place.
- **Need-driven scope:** nothing goes into berad-engine unless a consuming game
  needs it now.
- **Explicit plug-in list:** each program registers its plug-ins in one list;
  duplicates fail at startup; no import side effects.
- **Shared plug-in tests:** every edit kind passes the apply-then-undo test;
  every codec round-trips every message.
- **Workflow layout apart from logic:** node positions are saved separately from
  workflow logic, keeping diffs readable. *As built:* a workflow's layout lives
  in its own file (D11); the orchestrator reads and writes the two together,
  moves both on renames and deletions, and sends the layout with snapshots
  (protocol version 4) and with a deleted document's contents, so undo brings
  it back.
- **Orchestrator from small services** behind small interfaces, not one object.
