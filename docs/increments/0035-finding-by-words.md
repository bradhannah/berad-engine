# Increment 35: Finding by words

**Status:** Planned

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

To choose while building, for its author to check:

- **What a line matches:** its typed text with markup's tags left out, so
  "good morning" finds `[b]Good morning[/b]`; not the paths of the values
  in it, which their constants' rows find.
- **How a match's text is cut** to fit its row, around the words found.
- **What the inspector shows** for a constant or a line selected in
  Matches.
- **What a search starting with `#`** finds: tags only, as now, so no
  constants or lines.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Constants found | Find a constant's matching shared with the Project window; a row per constant found, under a Constants group in Matches, with its value or the text that matched and whose it is | Unit tests find a constant by its path, its default and an NPC's own text, and none for a search of tags | Planned |
| 2. Opened at a constant | The Constants window opening with a constant, or a holder's own value, selected and in view, as a Workflow window opens at a node | Unit tests open a constant and an NPC's own value from Matches and find them selected | Planned |
| 3. Lines found | The typed text of every workflow's Says, Show messages and Logs searched, markup's tags left out; a row per node, under a Lines group, naming its workflow and kind; opening it opens the workflow at the node | Unit tests find a Say in a workflow not open, skip a match only in a tag, and open the workflow at the node | Planned |
| 4. Tests and checks | A real-window test that searches the sample for a constant's words and a line's, opens each and finds it selected; EDITOR_FEATURES updated | `make check` passes, and so does the real-window suite | Planned |

## Step results

None yet.

## Not in this increment

- **Values found by their names.** The Values window's own search finds
  them.
- **Lines found in a node's other text,** such as a note or a Check
  value's text.
- **Replacing what was found,** across constants and lines.
