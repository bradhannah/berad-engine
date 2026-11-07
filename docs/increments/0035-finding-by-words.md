# Increment 35: Finding by words

**Status:** Built

The thirty-fifth increment lets the Project window's search find what a
game says. Today it matches documents' names, folders and tags. A line of
dialogue can only be found from inside the Constants window, and a line
typed into a Say only by opening workflows until it turns up. Searching
for "tide" now lists each constant whose name or text says it, and each
line typed into a workflow that says it. Opening one goes straight to it:
the constant selected in the Constants window, or the node selected in its
workflow.

It builds on:
- the Project window's search and Matches from
  [increment 32](0032-project-by-folders.md);
- constants and Find a constant from [increment 33](0033-constants.md);
- messages, with their markup, from
  [increment 26](0026-speech-and-boxes.md).

It is held to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A line is found by what it says, wherever it was written:** "tide"
   finds `world.tide_minutes` by its name, Marta's own greeting by its
   text, and a Say typed in Clerk's day.
2. **Opening a match goes to it:** the Constants window opens with the
   constant, or Marta's own value, selected; a line's workflow opens with
   its node selected.

## Decisions

Agreed by its author on 2026-11-06, before building:

- **Matches lists each constant found,** under a Constants group, with
  what matched: its value, or the text that says the words.

  ```
  MATCHES  "tide"

  Constants
    ◆ world.tide_minutes        30
    ◆ npc.shop.greeting         Marta: "…before the tide turns"

  Lines
    Clerk's day · Say           "…we close early, the tide…"
  ```

- **A constant is found as Find a constant finds it:** by its path, its
  default text, or any NPC's or zone's own text.
- **Lines typed into workflows are found too:** the text typed into a Say,
  a Show message or a Log, in every workflow, opened or not. A row per
  node opens its workflow with the node selected.
- **Opening an NPC's or zone's own value that matched** selects that
  holder's row in the Constants window, the text that matched. Otherwise
  the constant is selected.

Chosen while building, for its author to check:

- **What a line matches:** its typed text with markup's tags left out, so
  "good morning" finds `[b]Good morning[/b]`, and a word only in a tag
  finds nothing. A value's place parts the words either side of it, and
  its path isn't searched: its constant's row finds it.
- **A constant's text is matched without its tags too,** in its default
  and in each holder's own.
- **A row per text that matched:** a template's constant found in its
  default and in Marta's own text is listed twice, each opening at that
  text. One found by its path is listed once, with its value.
- **How a match's text is cut:** about 44 characters around the words
  found, on one line, in quotes, with an ellipsis where it is cut.
- **What the inspector shows:** what it is (a constant, whose own, or a
  line and its kind), its path or workflow, Open in Constants or Open in
  its workflow, and the whole text, a value's place in a line shown as
  ‹value›.
- **Counting:** Matches' number counts the constants and lines found, and
  the bar adds them after its count of documents, as "0 of 19 · 1
  constant".
- **A search starting with `#`** finds tags only, as before, so no
  constants or lines.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Constants found | Find a constant's matching shared with the Project window; a row per constant found, under a Constants group in Matches, with its value or the text that matched and whose it is | Unit tests find a constant by its path, its default and an NPC's own text, and none for a search of tags | Built |
| 2. Opened at a constant | The Constants window opening with a constant, or a holder's own value, selected and in view, as a Workflow window opens at a node | Unit tests open a constant and an NPC's own value from Matches and find them selected | Built |
| 3. Lines found | The typed text of every workflow's Says, Show messages and Logs searched, markup's tags left out; a row per node, under a Lines group, naming its workflow and kind; opening it opens the workflow at the node | Unit tests find a Say in a workflow not open, skip a match only in a tag, and open the workflow at the node | Built |
| 4. Tests and checks | A real-window test that searches the sample for a constant's words and a line's, opens each and finds it selected; EDITOR_FEATURES updated | `make check` passes, and so does the real-window suite | Built |

## Step results

- **Constants found:** the Constants window's Find and the Project's
  search share their matching, now exported from the World state window
  as `FindConstants`. A constant's row in Matches sits under Constants,
  after Settings and World state. Unit tests find constants by a path, a
  default, a holder's own text and a second line, show tide_minutes with
  its value and Ada's greeting without its tags, count them in the bar,
  and find none for a search of tags. Commit 8a8f36f.
- **Opened at a constant:** the Constants window takes a constant as an
  item to open at, as a Workflow window takes a node. It opens what holds
  the constant and selects it; for a holder's own value, it opens that
  NPC's or zone's template and selects its row. Unit tests open the
  window at the letter and at Guard 1's own greeting. Commit b49312c.
- **Lines found:** the Project window reads the workflows it already
  keeps loaded. A line's row, named by its workflow and kind, sits under
  Lines, and opens the workflow at its node. A unit test finds a Say by
  its words in a workflow not open, not by a word only in a tag nor across
  a value's place, and opens the workflow at it. Commit f9eeb21.
- **Tests and checks:** see below. Commit 39acab2.

Tests and checks:

- The real-window test searches the sample for "open", finds the shop's
  greeting, "Shop's open!", and opens the Constants window with it
  selected; then searches for "no stock", finds the customer's Say, and
  opens Customer shops with it selected. Its screenshots show Matches, the
  inspector, and each window opened at what was found.
- All 61 tests of the full real-window run passed, and `make check`
  passed from a clean test cache.

## Not in this increment

- **Values found by their names.** The Values window's own search finds
  them.
- **Lines found in a node's other text,** such as a note or a Check
  value's text.
- **Replacing what was found,** across constants and lines.
