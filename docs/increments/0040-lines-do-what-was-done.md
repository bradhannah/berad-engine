# Increment 40: Lines do only what their author did

**Status:** Built

A line in a workflow changes only when its author means it to, as agreed on
2026-11-11 in [the editor features](../EDITOR_FEATURES.md#workflows) ("a line
does only what its author did"). A line dropped on empty canvas no longer
disconnects: it offers a node to join it to. A line about to be replaced
shows it before it is let go. Unlinking is a command of its own, and Add
after says what it will do. It builds on
[increment 39](0039-readable-history.md), and is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it proves

1. **A missed drop changes nothing:**
   - a line dropped on empty canvas leaves the line as it was and opens a
     menu of nodes;
   - the menu's first line says what will be joined to what, and which
     line it would replace;
   - choosing a node adds it there, joined by that line, as one undo step;
   - Esc or a click elsewhere closes the menu and changes nothing.
2. **The same menu without a line:** Tab, or a double-click on empty
   canvas, opens it to add a node joined to nothing.
3. **A replacement is shown before it happens:** while a line is held over
   another node, the line it would replace is drawn red and dashed. A tag
   by the pointer says "Replaces the line to Wait".
4. **Unlinking is a command:** the line's menu and the inspector's button
   say Unlink, and Backspace unlinks a selected line. The status bar says
   what was unlinked from what.
5. **Add after says which it will do:** "Add after Wait · next" names the
   output it joins from; "Add beside Wait" says no output is free.
6. **Nothing else behaves differently:** the real-window walks pass.

## Decisions

Agreed by its author on 2026-11-11 (see the editor features, "a line does
only what its author did" and "explicit over clever"):

- **A line dropped on empty canvas opens a menu of nodes** whose first line
  says what will be joined to what. Choosing one adds it there, joined by
  that line alone, as one step. Esc or a click elsewhere leaves the line as
  it was.
- **Tab, or a double-click on empty canvas,** opens the same menu with
  nothing joined.
- **A line held over a dot that already has one** shows the old line red
  and dashed, with what would be replaced, before it is let go.
- **Unlinking is a command of its own.**
- **Add after says which it will do.**

Chosen while building, for its author to check:

- **The menu lists the palette's groups,** each opening its nodes beside
  it. The first line is a heading that cannot be chosen, as "Join Start ·
  next to a new:".
- **A node joined from a container's inside output is put in that
  container;** any other goes beside the node it is joined from.
- **A node that nothing can lead to,** a start, a receiver or a note, is
  refused when chosen for a line, with the reason.
- **Over empty canvas the old line is also red and dashed,** since a node
  chosen there would replace it. Its tag then says "A node chosen here
  replaces the line to Wait".
- **Unlink is no longer drawn in red,** since undo joins the line again.
  Its status reads "Unlinked Start · next from Wait. Undo joins it again."
- **Two tests that held the old behaviour now hold the new one:**
  `TestPortsConnectAndDisconnect` and `TestPickingUpALineByItsArrowEnd`.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The node menu | A drop on empty canvas, Tab and a double-click opening the menu; a choice adding a node joined by the line as one step | Unit tests; a real-window walk | Built |
| 2. A replacement shown first | The old line red and dashed, with a tag by the pointer | A unit test of the state | Built |
| 3. Unlink and Add after | Unlink named and said; Add after naming its output or saying the node goes beside | Unit tests | Built |
| 4. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **The node menu:**
  - `TestPortsConnectAndDisconnect` drops a line on empty canvas, finds
    the line unchanged and the menu open; Esc changes nothing, and Wait
    from the menu is added and joined.
  - In the real window, `TestALineDroppedOnEmptyCanvasOffersANode` does
    the same in a new workflow. Its screenshot shows the menu with the
    Action group open.
- **The rest:** `TestLinesDoOnlyWhatTheirAuthorDid` covers these:
  - the replaced line is marked over another node and not over the node
    it leads to;
  - Tab and a double-click open the menu, and nothing is joined;
  - Add after joins a third Wait from the first Wait's free output.
- **Checks:** `make check` passes. All 66 real-window walks pass on the
  increment's code commit, run from a frozen copy. Not checked by eye: the
  red, dashed line while a line is held, which no screenshot catches
  mid-drag.

## Not in this increment

- **The inspector writing out what joins to a node** and where each output
  goes, with Unlink beside each link.
- **A Say or Show message wrapping its line on the node,** and its field
  getting the room in the inspector. That changes how nodes are sized and
  laid out, and comes on its own.
