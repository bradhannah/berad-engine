# Increment 48: The inspector writes out a node's links

**Status:** Built

A Workflow's inspector now writes out what joins to the selected node and
where each of its outputs goes, with Unlink beside each line. This was
left out of [increment 40](0040-lines-do-what-was-done.md), under "a line
does only what its author did" in
[the editor features](../EDITOR_FEATURES.md#workflows). It builds on
[increment 47](0047-play-from-here.md), and is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it proves

1. **The links are written out:** under the node's fields, LINKS lists
   each line that joins to it, as "From Start", with the output named too
   when the node it comes from has several; then each of its outputs, as
   "then → Say".
2. **An output joined to nothing says where the flow goes from it,** as
   the canvas does: "then → start", "→ repeat" in a container, "→ lane
   done" in a Together's lane. It is in the muted ink, with no Unlink.
3. **Unlink is beside each line:** it takes the line away as Unlink on the
   canvas does, saying "Unlinked Wait · next from Say. Undo joins it
   again."

## Decisions

Chosen while building, for its author to check:

- **Lines joining to the node come first,** then its outputs in their
  order on the node.
- **A line from a node with one output is named by the node alone;** one
  from a node with several names the output as well.
- **The links scroll with the rest of the inspector,** and an Unlink works
  only while it shows whole.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The links | The links written out under the fields, each line with Unlink | A unit test | Built |
| 2. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- `TestTheInspectorWritesOutANodesLinks` selects a Wait between the start
  and a Say, reads "From Start" and "then → Say", unlinks the Say, reads
  "then → start" and the status bar's sentence, and unlinks the start from
  the Wait's inspector too.
- In the real window, `TestTheInspectorWritesOutLinks` adds a Wait after
  a new workflow's start, reads "From Start" and "then → start", and
  unlinks the start from the Wait's inspector. Its screenshot shows the
  links under the Wait's fields, matching the "→ start" on the canvas.
- **Checks:** `make check` passes, and so does
  `TestALineDroppedOnEmptyCanvasOffersANode`. The whole real-window suite
  runs on the last increment of the night.

## Not in this increment

**A Say or Show message wrapping its line on the node,** and its field
getting the room in the inspector, change how nodes are sized and laid
out, and come on their own.
