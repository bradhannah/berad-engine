# Increment 51: Views say their changes and refusals

**Status:** Built

The views now say their own changes and refusals, completing "a history
that can be read" in
[the editor features](../EDITOR_FEATURES.md#principles): "Every change is a
sentence saying what kind of change, to what, and where it went", and "a
refusal stays in it with its reason, and stays in the status bar until
dismissed". [Increment 39](0039-readable-history.md) gave the orchestrator
a sentence for each kind of change, and left the views' own sentences and
refusals for later. It builds on
[increment 50](0050-say-wraps-on-its-node.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A view says its change in its own words,** which the orchestrator
   says in place of its own, and Undo and Redo answer with:
   - a Map, "Painted 6 cells of Road on Ground floor", "Cleared 2 cells on
     Ground floor", "Placed Ada Clerk at 9, 8 on Ground floor", "Added the
     anchor Anchor 3 at 5, 5 on Ground floor", "Moved Ada Clerk to 10, 8";
   - a Workflow, "Added a Wait to Clerk's day", "Removed 2 nodes from
     Clerk's day", "Joined Start · then to Wait", "Unlinked Start · then
     from Wait", "Changed Wait's How long", "Moved 3 nodes in Clerk's day";
   - the Pixel editor, "Painted 6 pixels of gloomy / grey 3 on Lantern",
     "Filled 20 pixels of …", "Erased 4 pixels on Lantern".
2. **A stroke is said whole:** each command of a stroke says the stroke so
   far, its undo step keeps the latest, and History holds one line for it.
3. **A refusal a view makes stays:** what a view will not do, and why, is a
   refusal, in red in the status bar until dismissed and marked in History:
   a cell the player cannot start on, an NPC placed twice, a fill where the
   template does not allow painting, a Step that cannot run or gave up, a
   line that cannot lead to a node, and the others listed below.
4. **The newest message shows:** a refusal takes the status bar from a
   message said before it.

## Decisions

Chosen while building, for its author to check:

- **A step keeps the first command's sentence** when its commands are of
  different kinds, as the node menu's adding a node and joining it are:
  "Added a Wait to Clerk's day". A later command's own sentence replaces
  it only for the same kind of change, as a stroke's.
- **Views that are not yet given words** (the Project window, World state,
  palettes, templates, sounds) still say the orchestrator's sentence.
- **The refusals made to stay** are those that say the author's action was
  not carried out: in a Map, a preview already running, a start cell the
  player cannot stand on, stairs to a missing floor, a second ambient
  sound, anchor or NPC on a cell, a room not closed in, walls to select
  first, an NPC to choose first, placed already, not loaded, or living
  elsewhere, an NPC with no behaviour, and Step refusing or giving up; in
  a Workflow, a line that cannot lead to a node, words to select first, and
  a paste with nowhere to go; in the Pixel editor, a shade to choose, a
  template that does not allow painting, and a colour that cannot be
  picked; in World state, a name already used and a value that does not
  fit; in the Project window, a folder to empty first, a thing every
  project holds, a palette in use, and choices that do not fit; and no
  zones to pick from. Failures to read files or measure sounds stay
  messages.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Own sentences | A command's own sentence, said and kept for its step; a stroke kept as one line | Unit tests | Built |
| 2. The views' words | The Map's, the Workflow's and the Pixel editor's sentences | Unit tests; real-window walks | Built |
| 3. Refusals | The views' refusals made to stay, and the newest message shown | Unit tests | Built |
| 4. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Own sentences:** `TestAViewSaysItsOwnSentence` paints a stroke of two
  cells with the view's words; Undo answers "Painted 2 cells of Wall on
  Floor 1". `TestAStrokeKeepsOneLine` keeps one line in History.
- **The views' words:** `TestAStrokeIsSaidInTheMapsWords`,
  `TestAWorkflowSaysWhichNode` and
  `TestAStrokeIsSaidInThePixelEditorsWords` read each command's sentence.
  In the real window, `TestUndoSaysWhatItTookBack` now reads "Undid:
  Painted 2 cells of Road on Ground floor", and its screenshot shows
  History with one line for the stroke; `TestClickingAHistoryLineSelectsTheWait`
  clicks "Added a Wait to Workflow 1".
- **Refusals:** the tests that read these messages now find them marked
  "Refused:", and a refusal made after another message shows.
- **Checks:** `make check` passes, and so do the walks above and
  `TestTheInspectorWritesOutLinks`. The whole real-window suite runs again
  on the last increment.
