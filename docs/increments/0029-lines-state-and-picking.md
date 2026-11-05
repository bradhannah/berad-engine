# Increment 29: Lines by hand, World state's parts and picking signals

**Status:** Built

The twenty-ninth increment comes from its author trying increment 28 in
the sample. It stops the editor from being clever about lines. A node
dropped from the palette now lands where it falls and connects to nothing,
and back lines no longer route under nodes that are nowhere near them.
World state becomes a section of the Project window. Its Values and
Signals each open in their own window, keep a readable width on a wide
monitor, and have the inspector right beside them. The inspector can fold
away on a small one. A signal is chosen in a picker, as a value is, since a
project soon has too many signals for a row of buttons. It builds on the
palette of [increment 25](0025-palette.md), the value picker of
[increment 19](0019-value-picker.md) and the signals of [increment
28](0028-signals.md). It is held to the same strict checks as [increment
1](0001-foundation.md).

## What it proves

1. **The author draws the lines:** a node dropped anywhere, even on a
   line, connects to nothing, and existing lines stay as they were.
2. **A line moves only for what is in its way:** a node placed far below
   a back line, or outside its container, leaves the line where it is.
3. **World state has room to grow:** its parts are rows of a section,
   each with its own window. The two windows are filed together on the
   window bar and share one undo list.
4. **A wide monitor is read without crossing it:** World state's lists
   keep a readable width, with details close to names and the inspector
   beside them.
5. **Choosing a signal scales:** the picker finds one among many by any
   part of its path, and puts the workflow's own signals first.

## Decisions

Asked for by its author on 2026-11-04, after trying increment 28: drops
that kept going into lines far away, and lines that rerouted around a new
node for no reason; signals and values as parts of World state in a tree;
a way to choose among many signals; less empty space on a wide monitor;
and a collapsible inspector.

Agreed by its author on 2026-11-04, in answer to the planning questions:

- **A drop lands where it falls,** connected to nothing. Nothing is put
  into a line and no line lights up. (Rejected: putting it into a line
  only while Alt is held.)
- **Add after connects to a free output only.** With none free, the new
  node sits beside the selected one, connected to nothing; it is never put
  into a line.
- **A back line clears only what is in its way:** it runs just under its
  own two nodes and dips below a node only where it would run through it,
  and only for nodes in its own container. (Rejected: never routing round
  anything.)
- **World state is a section of the Project window,** holding Values and
  Signals, each opening its own window. (Rejected: one World state window
  whose top level is VALUES and SIGNALS.)

Agreed by its author on 2026-11-04, from the wireframes on the canvas
"Workflow canvas wireframes":

- **The inspector only folds.** The ideas in the row "The inspector on
  small and large monitors" are set aside: floating beside the selection,
  quick settings on the node, docked along the bottom, auto-hide and
  fitting the layout to the window's width.
- **A signal is chosen in a picker,** the first board of the row
  "Choosing a signal among many". With nothing typed, the signals this
  workflow sends or answers come first, then every signal under folding
  groups, each with how often it is sent and answered. (Rejected: columns
  of groups and signals, and type-ahead in the inspector's box.)
- **Values and Signals take option A** of the row "Wide windows with less
  empty space": a list of readable width, details in a column beside the
  names, the inspector next to the list. The Project window itself takes
  none of the options yet (see Not in this increment).

Chosen while building, for its author to check:

- **The inspector folds from the status bar.** An Inspector toggle sits
  beside On top. Turned off, every tab's inspector folds together to a
  strip at the right edge, and clicking the strip opens it again. Like On
  top, it lasts for the session and is not saved.
- **Folding needs no change to a view.** The window gives a folded view
  more width than it shows, so the view's inspector falls past the edge.
  A view that places its inspector itself, as World state's do, is told
  the inspector is folded.
- **The readable width is 720 pixels,** with details 320 pixels from the
  list's left. The inspector stands beside the list once the window is
  wider than that and an inspector; on a narrower window it is at the
  right edge, as before.
- **The two World state windows are titled World state: Values and World
  state: Signals.** The window bar shows them as Values and Signals under
  World state, each in its own colour.
- **Signals have an icon of their own,** a dot with waves going out, in
  the workflow colour, on the Signals row of the Project window.
- **The picker does not declare signals.** That was the second board of
  its row; signals are still declared in World state's Signals window,
  which the picker says when nothing matches.
- **Choosing an NPC is unchanged:** a button for each NPC. It will need a
  picker of its own as NPCs multiply.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Drops land free | A palette drop adds the node where it falls, connected to nothing; Add after uses a free output or places the node beside, unconnected | A drop on a line leaves the line and the new node unconnected, and Add after with no free output connects nothing | Built |
| 2. Back lines | A back line runs under its own two nodes and dips only below nodes in its container that it would run through | A node far below a loop leaves its back line where it was, and a node where the line runs puts the line under it | Built |
| 3. World state's parts | The World state section with Values and Signals; a window for each, filed together on the bar, sharing an undo list; the signal icon | Values and Signals open from the Project window into their own windows, each showing only its part | Built |
| 4. The inspector folds | The Inspector toggle, the folded strip, and the window widening a folded view | Folding in one tab folds every tab's inspector, and the strip opens them again | Built |
| 5. Readable lists | World state's lists keep a readable width, details in a column beside the names, the inspector beside the list | On a wide window the inspector sits after the list, and at the right edge while folded | Built |
| 6. The signal picker | A signal's box opens a picker that searches paths, lists the workflow's own signals first and folds groups | A signal is found by part of its name and chosen with Enter or a click; groups open and close | Built |

## Step results

- **Drops land free:** dropping onto a line and Add after no longer put a
  node into a line. The help line in the inspector no longer says to drag
  onto a line. Unit tests check a drop on a line and on empty canvas, and
  Add after with no free output. The palette's real-window test drops a
  Wait on the line from the start and finds it unconnected.
- **Back lines:** a back line's depth starts under its two nodes and below
  any shorter back line over the same stretch. It then steps below each
  node in its container whose height it would run through. A unit test
  places a node far below a loop and then where the loop's line runs.
- **World state's parts:** the Project window's World state section shows
  Values with its count of values and Signals with its count of signals.
  Each has its own inspector and opens its own window. Values keeps Add
  value, Add group, Add template and Find a value; Signals has Add signal
  and Add group. Messages that pointed at "the World state window" now
  name the window meant. Unit tests cover the section, the two windows'
  parts and the bar's grouping. The World state real-window test opens
  both windows and adds a signal in Signals.
- **The inspector folds:** a real-window test folds the inspector from a
  workflow's status bar, sees the Project tab fold with it, and opens both
  again from the strip.
- **Readable lists:** lists can now put tree rows' details in a column. A
  unit test finds a selected signal's name field beside the list, and at
  the right edge while folded.
- **The signal picker:** a new package, signalpicker, with its own unit
  test of groups and picking. In the Workflow window, Send signal's and On
  signal's Signal settings are boxes that open it. How often a signal is
  sent and answered is now worded in one place, the signals package. Unit
  tests choose signals through the picker and find one by typing. The
  signals real-window test opens the picker from the clerk's receiver.
- **Found by the full real-window suite:**
  - World state's new rows pushed the Project list's Palettes and Templates
    below the rows a test window shows. A newly selected row now scrolls
    into view once it is listed, as does a row an input script locates.
  - The pixel editor's Template menu opened down under the status bar,
    where the Inspector toggle now sits, so its last item could not be
    seen. A menu opened below a button now opens above it when there is
    no room below, and every menu stays above the status bar.
  - Scripts that put a node into a line with Add after now do it by hand,
    as an author does. "<kind>:last" names the newest node of a kind, and
    Watch's "log-line:<text>" the newest line reading so, since the newest
    line can be another NPC's.
- **Checks:** no lint findings, and `make check` passes. The real-window suite
  passes, 58 tests, among them the new one that folds the inspector.

## Not in this increment

- **The Project window at a large scale.** Its author asked on 2026-11-04
  for research-led options for projects of thousands of things. Seven are
  wireframed on the canvas, in the row "The Project at scale", and
  summarised in [research notes](../RESEARCH_NOTES.md): facets, a working
  set, stepping along relations, an overview, an atlas by place, columns,
  and quick open. None is chosen yet.
- **Declaring a signal from the picker,** the second board of its row.
- **Remembering a folded inspector** between sessions.
- **A picker for NPCs.**
