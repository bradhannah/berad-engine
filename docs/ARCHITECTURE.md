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

## D2 — Project data versus session state

- **Project data:** authored content. Saved, undoable, written only by the
  orchestrator.
- **Session state:** selection, pick requests, the clock, preview state, open
  windows. Shared live between windows; never saved and never undone.

## D3 — Undo and redo scope

Each window undoes its own recent edits, in order, whatever they touched. Views
of the same map share one undo list: undo in any of them takes back the last
change to that map, whichever view made it. An undo is itself a command; the
orchestrator holds the histories.

If something an undo would revert was changed afterwards from a different
window, the undo is refused with a message rather than overwriting that change.
The first build compares individual items (for a floor, its cells). *Open:*
whether other kinds of documents need a broader rule.

Closing a window ends its undo list (unless another view shares it). Undo does
not survive quitting the editor; versions (D9) cover older changes. Each undo
list keeps 100 steps by default.

## D4 — Identity and references

Every entity has a permanent **ULID**, assigned by the orchestrator when it is
created. References store IDs; names are labels, so renaming never breaks a
reference. IDs are shown read-only and visually subdued, which helps debugging.

Names are unique within their scope (for example no two anchors with the same
name in one zone). Deleting something that other content references shows where
it is used and lets the author cancel or proceed; proceeding leaves broken
references flagged in a Problems view, and the deletion can be undone.

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

## D8 — Large data

Art is served by the orchestrator. Windows cache assets by content fingerprint
and are never sent an asset they already hold; change notices carry the new
fingerprint. Pixel edits are patches (the changed rectangle) sent as ordinary
strict, undoable commands, grouped per stroke, and other windows update live as
the author draws. Pixel data is the first candidate for a binary codec.

## D9 — Saving and versions

- **Always saved:** every applied command is written to a journal before it is
  acknowledged. There is no unsaved state and no Save command. The journal is
  folded into the document files continuously.
- **Versions:** the project folder is a git repository; the editor's "create
  version" and "restore version" use it, presented in editor terms. Ordinary git
  tools still work.
- *Open:* how restoring a version interacts with open windows and undo.

## D10 — Lifecycle and failure

- One orchestrator per project, protected by a lock. *Open:* a second launch on
  an open project should join the running editor; the first build refuses it
  with directions instead.
- The orchestrator starts every window process; windows never start each other.
- The editor quits when its last window closes (later: when the project browser
  closes); no orchestrator is left running without windows.
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
(journal, backups, logs, lock) lives in a git-ignored `.berad` folder.
*Open:* chunking very large floors.

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
- **Opening something already open** brings its window to the front.
- **Several views of one map** are allowed through an explicit "New view"; all
  views show every change and share undo (D3). During preview, every view of the
  map shows the live entities.
- **Pick on map:** a parameter decides what can be picked (a tile, an anchor, an
  area, an object, another NPC). If no Map window is open, the editor asks which
  zone and opens one; with one open, it is used; with several, the editor asks
  which, marking the maps that show the thing being picked. Picking uses four
  messages through the orchestrator (ask for candidate maps, start picking,
  answer, cancel); the requesting window then makes an ordinary edit. *Open:*
  the message contents, designed when the first window needs picking.

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
  workflow logic, keeping diffs readable.
- **Orchestrator from small services** behind small interfaces, not one object.
