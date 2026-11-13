# Increment 60: Braces, a first colour, and a crashed tab

**Status:** Built

A line can hold a brace, a sprite opens with the colour last used in it,
and a crashed tab reopens when told, as agreed on 2026-11-13 in
[the editor features](../EDITOR_FEATURES.md#principles). It settles the
review's findings 18, 27 and 30. Notices that wait their turn, 55, came
with [increment 57](0057-files-are-shared.md). It
builds on [increment 59](0059-keys-in-a-field.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **`{{` writes a brace (18):** "Use the {{red} door" reads "Use the {red}
   door" in the game, and reads back as typed in the field.
2. **A sprite opens with the colour last used in it (27):** kept per person.
   One never painted opens with none, and the Pencil and Fill refuse:
   "Choose a colour to paint with".
3. **A crashed tab reopens when told (30):** its notice offers Reopen.
   Opening documents and Launch keep working while the Project tab is
   closed, since the editor window is asked to show panes through a
   connection of its own. A Project tab reopened is the Project tab again,
   first in the row, and closing it quits the editor.
4. **Nothing else behaves differently:** the real-window walks pass.

## Decisions

Chosen while building, for its author to check:

- **A brace written as two is the only new syntax;** `{` alone still
  begins a value's name, and an opening brace with no closing one is still
  kept as text. The field shows a line's braces doubled, and the game one.
- **The colour last used is kept for each sprite, for this person,** in
  `.berad/colours.json` beside the window's arrangement. Choosing clear is
  a colour like any other, and the Pencil then erases, as before. The
  status bar says "no colour" while none is chosen.
- **Quick open still needs the Project tab,** as it is the Project tab's
  search; while that tab is closed Cmd+P says so, and the crash notice's
  Reopen brings it back. Opening a document and Launch do not need it.
- **The crash notice names the tab,** "The Project tab closed
  unexpectedly", and offers Reopen and Dismiss. Reopen opens the same view
  of the same document again; nothing reopens by itself.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Braces | `{{` in a line's text, both ways | Unit tests | Built |
| 2. A first colour | The colour last used per sprite; none refused | Unit tests | Built |
| 3. Reopen | Reopen in a crash's notice; opening and Launch without the Project tab | Unit tests; a real-window walk | Built |
| 4. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Braces:** `TestABraceInALine` types "Use the {{red} door,
  {local.count}": the line holds "Use the {red} door, " as text, reads
  "Use the {red} door, 0", and shows as typed.
- **A first colour:** `TestAFirstColourIsRememberedNeverGuessed` is refused
  painting with no colour, chooses grey 3, which is kept, and a new Pixel
  editor of the sprite opens with it; `TestColoursLastUsedAreKept` keeps
  the colours across opening the project again.
- **Reopen:** `TestACrashedTabReopensWhenTold`, and the real-window walk
  `TestACrashedProjectTabReopensWhenTold`, which crashes the Project tab
  with the key only tests turn on: the Map's status bar offers Reopen, the
  Project tab comes back, and it quits the editor as before.
