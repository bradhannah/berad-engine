# Increment 34: Multi-select

**Status:** Built

The thirty-fourth increment lets an author work on several nodes of a
workflow at once. Today every node is moved, coloured, deleted or rebuilt
one at a time, so tidying a workflow or reusing part of one means doing the
same thing node by node. Cmd+click or a box dragged on empty canvas
selects several nodes and containers. They then move, nudge, colour,
delete, duplicate and arrange together, and copy into another workflow.
Each of these is one undo step.

It builds on:
- the canvas and its colours from [increment 17](0017-reading-the-graph.md);
- the workflow's locals from [increment 15](0015-values-and-sensing.md);
- lines drawn by hand from [increment 29](0029-lines-state-and-picking.md).

The edits to add, move, colour and remove nodes already take several nodes
at once. Most of the work is on the canvas and in the inspector. It is held
to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **Part of a workflow is reused in a few gestures:** Go to → Wait → Say
   is boxed, copied and pasted into another workflow, still linked to each
   other, with the local it sets declared there.
2. **Tidying is done once, not node by node:** three nodes are moved,
   coloured and arranged together, and one Undo puts each change back.
3. **Selecting never surprises:** Cmd+click adds or removes one node, a box
   adds what it touches, and a plain click or Escape goes back to one node
   or none.

## Decisions

Agreed by its author on 2026-11-01, and changed on 2026-11-06 from
Shift-click (recorded in [EDITOR_FEATURES](../EDITOR_FEATURES.md)):

- **Cmd+click or a box dragged on empty canvas** selects several nodes and
  containers. Shift suggests a range, a path from one node to another,
  which a graph does not have, and Ctrl+click is a Mac's right-click.
- **Together, each as one undo step:** move, colour, duplicate (Cmd+D, or
  copied and pasted into another workflow) and delete.
- **Lines are never part of a selection.**
- **A container brings its contents,** and a dragged selection stops at a
  container's wall, as one node does.
- **Arrow keys nudge the selection,** and Arrange can lay out the selection
  alone.

Agreed by its author on 2026-11-06, before building:

- **All of it in this increment,** paste into another workflow included.
- **A copy keeps the lines between the nodes copied.** Go to → Wait → Say
  copies as Go to′ → Wait′ → Say′. The line from Start into Go to stays
  behind, so the copy waits to be linked.
- **A paste declares the locals it needs.** A Set value of the workflow's
  own `count`, pasted into a workflow with no `count`, adds `count` to that
  workflow's locals with the same type and default. The status bar says so,
  and one Undo removes both the nodes and the local.
- **With several selected, the inspector shows the count, Colour and
  Delete:** "3 nodes", one colour row that colours them all, and Delete 3
  nodes. Their own fields are hidden.

Chosen while building, for its author to check:

- **What a box selects:** the nodes it touches, and a container only when
  it holds the container whole, so a box crossing a container's edge
  doesn't take everything in it. Cmd with a box adds to the selection.
- **Where a box can start:** only on empty canvas. A drag inside a
  container moves the container, as before.
- **A click on a selected node** without moving it selects that node
  alone; dragging it moves them all.
- **Nudges:** an arrow moves the selection 8 pixels, and 40 with Shift.
  Nudges of the same selection less than a second apart are one undo step.
  An arrow nudges one node selected alone too.
- **Where a copy lands:** a duplicate 24 pixels below and right of the
  originals; a paste with its top left under the pointer when it is over
  the canvas, and in the middle of the view otherwise.
- **The copies are selected** after Duplicate or a paste; of a container
  copied, only the container.
- **Copies keep their colours.** Their lines start straight, without the
  bends of the originals'.
- **A pasted node whose container wasn't copied** goes to the top level.
  A duplicate stays in the container its original is in.
- **The start** moves and is arranged with a selection, but is never
  deleted, copied or duplicated: Delete N nodes counts the rest, and its
  own right-click menu shows Duplicate and Copy greyed.
- **A local of the same name and type** in the workflow pasted into is
  used by the pasted nodes, and nothing is declared. **One of the same
  name but another type** leaves the pasted nodes' local declared with a
  number, as `count_2`. A local declared by a paste is the workflow's own,
  not an input or output, and sits outside any group.
- **A paste refuses nodes the workflow doesn't offer,** such as a Go to in
  a zone loop, and says so; nothing is pasted.
- **Copy uses the system clipboard,** as text. Other text pasted is left
  alone.
- **The right-click menu** on a node or a selection offers Duplicate,
  Copy and Delete. A right-click on a selected node keeps the selection.
- **Auto-arrange keeps its name** while several nodes are selected; the
  inspector's hint says it lays out just these. It keeps their top left
  where it was, leaves notes placed by hand where they are, and
  straightens the lines out of the nodes it arranges.
- **Cmd+click away from a node with unsaved changes** asks first, as a
  click does.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Several selected | The selection as a set: Cmd+click adds or removes a node or container; a plain click, a click on empty canvas or Escape goes back to one or none; each selected node drawn selected; the inspector's "N nodes", colour row and Delete N nodes; `selection=` for input scripts | Unit tests add and remove nodes with Cmd+click, clear the selection three ways, and find the inspector's count | Built |
| 2. The box | A box dragged from empty canvas, drawn as it is dragged; what it touches selected, a container only whole; Cmd with a box adds to the selection | Unit tests box three nodes, box across a container's edge without taking it, and add with Cmd | Built |
| 3. Moved together | Dragging any selected node moves them all, with containers' contents, stopping at a container's wall; arrow keys nudge, further with Shift; one undo step per drag, and per run of nudges | Unit tests drag three nodes, nudge them, stop at a wall, and undo each in one step | Built |
| 4. Coloured and deleted together | The colour row colouring all of them; Delete, Backspace or Delete N nodes removing all but Start; right-click on a selection offering Delete N nodes; one undo step each | Unit tests colour and delete three nodes, keep Start, and undo each in one step | Built |
| 5. Duplicated | Cmd+D copies the selection with new IDs, the lines between them, and containers' contents, a little below and right; the copies become the selection | Unit tests duplicate Go to → Wait → Say and a container, find the lines between the copies and none into them, and undo in one step | Built |
| 6. Copied and pasted | Cmd+C writes the selection to the clipboard as text; Cmd+V in any workflow pastes it with new IDs, declaring the locals it needs, as one undo step; text that isn't a copied selection is ignored | Unit tests paste into another workflow, declare a missing local, reuse one already there, ignore other text, and undo in one step | Built |
| 7. Arranged | With several selected, Auto-arrange lays out the selection alone, where it is, and leaves the rest | A unit test arranges three nodes and finds the others unmoved | Built |
| 8. Tests and checks | A real-window test that boxes and Cmd+clicks nodes, drags and nudges them, colours them, duplicates them, copies them into another workflow and saves; EDITOR_FEATURES and VOCABULARY updated | `make check` passes, and so does the real-window suite | Built |

## Step results

- **Several selected:** the selection is a set beside the one node
  selected, never both. Cmd+click adds or takes out a node; a click, a
  click on empty canvas or Escape goes back to one or none. Each selected
  node is ringed. With several, the inspector shows how many, with a hint,
  and neither a node's fields nor the locals. Unit tests add and take out
  nodes, clear the selection three ways, ask before leaving a node with
  unsaved changes, and draw the inspector. Commit 959bb4f.
- **The box:** a drag from empty canvas draws a light blue box and
  selects what it touches as it goes, a container only whole; with Cmd it
  adds. Unit tests box two nodes, add a third with Cmd, start afresh
  without it, leave a container a box crosses, take one it holds, and draw
  the box. Commit 55bc19a.
- **Moved together:** dragging any selected node moves them all, with
  what their containers hold, short of any container's wall, in one
  gesture; the arrow keys nudge. Unit tests drag two nodes and leave a
  third, nudge four times in one undo step, and stop two nodes at a wall.
  Commit f3aece7.
- **Coloured and deleted together:** one colour row colours all but
  notes, ringing the colour they share. Delete N nodes, the Delete key and
  the right-click menu remove all but the start in one edit. Unit tests
  colour two nodes in one edit, delete them with the start kept, and use
  the key and the menu. Commit b21663e.
- **Duplicated:** the workflow's add edit gained a copy: preparing it
  gives every node a new ID, and the parents, lines, boxes and tints among
  the copies follow, a line to a node not copied dropped. Cmd+D and
  Duplicate use it. Unit tests prepare and undo a copy, duplicate a green
  Wait → Say, and duplicate a container with a Wait in it. Commit 555340c.
- **Copied and pasted:** Cmd+C or Copy puts the selection on the
  clipboard with the locals its nodes read, by a value or a found NPC;
  Cmd+V pastes it into any workflow, declaring what is missing in the same
  undo step. Unit tests paste into a workflow with no count, one with its
  own count, one whose count is text and the same workflow; paste other
  text; refuse a Go to in a zone loop; and duplicate and copy from the
  menu. Commit f7b2f5a.
- **Arranged:** with several selected, Auto-arrange lays them out as a
  workflow of their own, from the nodes nothing among them leads to. A
  unit test arranges a Wait and a Say moved out of line, with the Log left
  alone. Commit 08ddf07.
- **Tests and checks:** see below. Commit 62567c4.

Tests and checks:

- The real-window test makes start → Wait → Say → Log, boxes the Wait and
  the Say, Cmd+clicks the Log, drags and nudges the three, colours them
  green, duplicates, copies and pastes them, and arranges the pasted ones.
  The saved workflow holds ten nodes in three green runs of Wait → Say →
  Log. Its screenshots show the inspector for three nodes and the arranged
  paste. It writes to the system clipboard while it runs.
- In the full real-window run, 57 of the 60 tests passed. The other three
  failed only to save their screenshots, as the run was given a relative
  folder; given a full path, all three passed. The call test that fails
  at random (`TestClerkCallsFetchStock`) passed.
- Formatting, lint and vet pass. `make check`'s tests failed four times
  in the orchestrator's audio tests, each time a different one timing out
  while adding a sound. Main fails the same way through `make test` on
  the same machine at the same time, and both pass when the same tests are
  run directly with the race detector, so it is not this increment. The
  workflow view's, the workflow plug-in's and the interface's tests pass.

## Not in this increment

- **Cut (Cmd+X).** Copy then delete does the same.
- **Selecting lines,** which stay out of a selection as agreed.
- **Select all (Cmd+A).**
- **Copying between projects.**
