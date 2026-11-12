# Increment 45: A line in History selects what it names

**Status:** Built

A click on a line in History now selects what the line names, as agreed in
"a history that can be read" in
[the editor features](../EDITOR_FEATURES.md#principles): "a click on a line
selects what it names". A change says what it touched, a view finds the
thing it can still select, and the line says what a click will select. It
builds on [increment 44](0044-workflow-names-what-is-in-hand.md), and is
held to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A change says what it touched:** the orchestrator answers a command,
   an Undo and a Redo with the document the change was made to and the
   parts of it the change touched, beside its sentence.
2. **A line says what a click selects:** while History is open, each line
   whose change touched something the view can still select says
   "selects Wait" before its time. A line that names nothing selectable
   says nothing more, and a click on it does nothing.
3. **A click selects it:**
   - in a Workflow, the first node the change touched, brought into view;
   - in a Map, the NPC placement or anchor the change touched, on its
     floor, with the NPC or Anchors tool in hand.

   History closes, and the status bar says "Selected Wait".
4. **Nothing waiting for Save is lost:** a pane whose changes wait for Save
   keeps its selection, and says so, as Back does.

## Decisions

Chosen while building, for its author to check:

- **A line names the first thing its change touched that is still there.**
  A node taken out, or an Undo that took a node out, names nothing, so its
  line offers nothing to select.
- **What a line can select is found while History is open,** not when the
  line was said, so it is never something already gone.
- **A change keeps at most 16 of the parts it touched,** enough to find
  something to select without sending every cell of a long stroke.
- **Painted cells name nothing to select.** A Map's selection of cells is
  for walls, and selecting cells a stroke painted would be a new kind of
  selection.
- **Lines from another view's changes,** such as Undo in a pane showing
  another document, name nothing in this pane.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. What a change touched | The document and parts in each result of a command, an Undo and a Redo | A unit test | Built |
| 2. Lines that select | The line's "selects …", the click, and the refusal while changes wait for Save | Unit tests | Built |
| 3. The views | The Workflow's nodes, and the Map's placements and anchors | Unit tests; a real-window walk | Built |
| 4. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **What a change touched:** `TestChangesAreSaidAndUndoSaysWhich` finds the
  floor and its cells with a paint, and with its Undo.
- **Lines that select:**
  - `TestALineInHistorySelectsWhatItNames` clicks a line that names
    nothing, which does nothing, and then one that names a Wait, which
    chooses it once and closes History.
  - `TestAClickOnAHistoryLineAsksTheViewToShowIt` clicks the line while
    the pane has changes waiting for Save, which keeps the selection and
    says so, and again once nothing waits, which shows the Wait and says
    "Selected Wait".
- **The views:**
  - `TestAHistoryLineNamesANode` names a Log while it is in the workflow,
    and nothing for another workflow or once it is taken out.
  - `TestAHistoryLineNamesAPlacementOrAnAnchor` names Ada Clerk's
    placement and the Desk anchor, selects each with its tool when asked,
    and names nothing for painted cells or another zone's floor.
  - In the real window, `TestClickingAHistoryLineSelectsTheWait` adds a Wait
    from the node menu, selects the start, and clicks the line that said
    the Wait was added; the Wait is selected again.
- **Checks:** `make check` passes. All 67 real-window walks pass on the
  increment's code commit, run from a frozen copy. A screenshot of the open
  History shows "selects Start" and "selects Wait" on the lines.
