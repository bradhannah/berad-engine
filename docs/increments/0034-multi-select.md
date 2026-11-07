# Increment 34: Multi-select

**Status:** Planned

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

To choose while building, for its author to check:

- **What a box selects:** the nodes it touches, and a container only when
  it holds the container whole, so a box crossing a container's edge
  doesn't take everything in it.
- **Where a box can start:** only on empty canvas. A drag inside a
  container moves the container, as now.
- **How far a nudge goes,** and how far with Shift.
- **Where a copy lands:** a duplicate a little below and right of the
  originals; a paste under the pointer when it is over the canvas, and in
  the middle of the view otherwise.
- **The Start node** moves with a selection, but is never deleted, copied
  or duplicated.
- **A local of the same name but another type** in the workflow pasted
  into.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Several selected | The selection as a set: Cmd+click adds or removes a node or container; a plain click, a click on empty canvas or Escape goes back to one or none; each selected node drawn selected; the inspector's "N nodes", colour row and Delete N nodes; `selection=` for input scripts | Unit tests add and remove nodes with Cmd+click, clear the selection three ways, and find the inspector's count | Planned |
| 2. The box | A box dragged from empty canvas, drawn as it is dragged; what it touches selected, a container only whole; Cmd with a box adds to the selection | Unit tests box three nodes, box across a container's edge without taking it, and add with Cmd | Planned |
| 3. Moved together | Dragging any selected node moves them all, with containers' contents, stopping at a container's wall; arrow keys nudge, further with Shift; one undo step per drag, and per run of nudges | Unit tests drag three nodes, nudge them, stop at a wall, and undo each in one step | Planned |
| 4. Coloured and deleted together | The colour row colouring all of them; Delete, Backspace or Delete N nodes removing all but Start; right-click on a selection offering Delete N nodes; one undo step each | Unit tests colour and delete three nodes, keep Start, and undo each in one step | Planned |
| 5. Duplicated | Cmd+D copies the selection with new IDs, the lines between them, and containers' contents, a little below and right; the copies become the selection | Unit tests duplicate Go to → Wait → Say and a container, find the lines between the copies and none into them, and undo in one step | Planned |
| 6. Copied and pasted | Cmd+C writes the selection to the clipboard as text; Cmd+V in any workflow pastes it with new IDs, declaring the locals it needs, as one undo step; text that isn't a copied selection is ignored | Unit tests paste into another workflow, declare a missing local, reuse one already there, ignore other text, and undo in one step | Planned |
| 7. Arranged | With several selected, Auto-arrange lays out the selection alone, where it is, and leaves the rest | A unit test arranges three nodes and finds the others unmoved | Planned |
| 8. Tests and checks | A real-window test that boxes and Cmd+clicks nodes, drags and nudges them, colours them, duplicates them, copies them into another workflow and saves; EDITOR_FEATURES and VOCABULARY updated | `make check` passes, and so does the real-window suite | Planned |

## Step results

None yet.

## Not in this increment

- **Cut (Cmd+X).** Copy then delete does the same.
- **Selecting lines,** which stay out of a selection as agreed.
- **Select all (Cmd+A),** unless it comes for free with the selection set.
- **Copying between projects.**
