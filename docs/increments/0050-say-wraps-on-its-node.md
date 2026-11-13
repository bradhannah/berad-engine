# Increment 50: A Say wraps its line on the node

**Status:** Built

A Say or a Show message now wraps its line on its node, and its field has
room in the inspector, as agreed on 2026-11-11 under "a line does only what
its author did" in [the editor features](../EDITOR_FEATURES.md#workflows):
"A Say or Show message wraps its line on the node, and its field gets the
room in the inspector". It was left out of
[increment 40](0040-lines-do-what-was-done.md) and
[increment 48](0048-inspector-writes-links.md). It builds on
[increment 49](0049-pixel-editor-names-what-is-in-hand.md), and is held to
the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **The line wraps on the node:** a Say or a Show message grows a line for
   each further line its message takes, its values as chips among the
   words, keeping its own line breaks.
2. **It stops at four lines:** a longer message ends its fourth line with
   an ellipsis, and the inspector holds the whole of it.
3. **Nothing else moves:** the nodes under a Say stay where they were put.
4. **Its field has room:** in the inspector, a Say's or a Show message's
   field takes three rows before anything is typed, and grows to ten. A
   Log's still starts at one and grows to six.

## Decisions

Chosen while building, for its author to check:

- **Four lines at most** on the node, so one long line cannot push a node
  far down the canvas.
- **A node that grows can overlap the node under it,** in a workflow laid
  out before. Nothing is moved to make room, as "explicit over clever"
  asks; Auto-arrange, or a drag, makes room.
- **A Log stays one line,** since the agreed direction names a Say and a
  Show message, and a Log's line is read in Watch.
- **A one-line message is the height it was,** so a workflow whose Says
  are short looks as it did.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The wrapped line | Laying the message out in lines with its chips; the node's height; drawing it | A unit test | Built |
| 2. The field's room | Three rows at least, and up to ten, for a Say and a Show message | A unit test | Built |
| 3. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **The wrapped line:** `TestASayWrapsItsLineOnTheNode` types a line that
  takes three lines on the node, finds the Wait under it where it was, and
  a longer line shown in four.
- **The field's room:** `TestASaysFieldHasRoom` finds a Say's field taller
  than a Log's before anything is typed, and growing past six rows.
  `TestALongLineGrowsItsField` now holds a Log's field to one row growing
  to six.
- In the real window, `TestASayWrapsOnItsNode` adds a Say after a new
  workflow's start and types it a long line; the Say shows it in three
  lines, and its screenshot shows the field and the node together.
- **Checks:** `make check` passes, and so does `TestLoggingInWatch`, whose
  workflows hold Says. All 74 real-window walks pass on the night's last commit, which carries
  this one, run from a frozen copy.
