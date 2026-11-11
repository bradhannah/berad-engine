# Increment 36: Bugs from the review

**Status:** Built

The thirty-sixth increment adds nothing an author can do. It fixes
twenty-one bugs found by a review of the editor as it stood after
[increment 35](0035-finding-by-words.md), on 2026-11-11: first the six that
lose or damage an author's work, then fifteen more from the same list. Each
was first shown by a test that failed, then fixed. The numbers in brackets
are the review's.

It builds on:
- folders kept as real folders on disk, from
  [increment 3](0003-npc-workflows.md);
- undo steps from [increment 1](0001-foundation.md);
- the single window and its quit dialog as of
  [increment 13](0013-single-window.md), and the folding inspector from
  [increment 29](0029-lines-state-and-picking.md).

It is held to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

The six that lose work:

1. **A folder renamed only in its capitals keeps what it holds (1):** on
   a file system that ignores case, Town renamed to town still holds its
   documents on disk, an empty folder stays, the directory takes the new
   spelling, and all of it reads back.
2. **A refused undo or redo changes nothing and keeps its step (3):** a
   stroke across two floors, one of them deleted from another window
   meanwhile, leaves the other floor as it was, and undoes once the floor
   is back.
3. **The quit dialog takes its clicks where it is drawn (8):** with the
   inspector folded, a click on Cancel cancels.
4. **Clearing a folder's name does not delete the folder (9):** nothing is
   sent, and the Project says a folder needs a name.
5. **`Cmd+Shift+K` does nothing for an author (12):** it crashes a tab
   only when a test turns the key on.
6. **Dragging in a field selects text (15):** from the press to the
   pointer, until the button is let go.

The fifteen more:

7. **A local value's unsaved changes are asked about (10):** selecting a
   node, a line or several nodes, or adding another local, asks Save,
   Don't save or Cancel first.
8. **A save that fails is told (6):** every window shows why, and that
   the changes are kept in the journal, until a save succeeds.
9. **A file that cannot be read is named (5):** a project opened with
   such a file says which file and why.
10. **A field's limit counts characters (16):** accented text fits as
    the counter says, and text that would not fit is refused with the
    reason in the status bar.
11. **A time can move to start where it ended (17):** 17:00–18:00
    becomes 18:00–20:00 in one Save.
12. **A paint stroke paints only where the pointer went (19):** leaving
    the floor and coming back in elsewhere joins nothing up.
13. **A place can be picked on any floor (20):** the floor list and
    PageUp and PageDown work while the Map picks.
14. **Any number of walls change family (21):** more than one paint
    command holds, still as one undo step.
15. **The Map's hint for no NPCs clears the floor list (33).**
16. **A step that cannot end gives up and says why (36).**
17. **The folder shown is followed (23):** renamed or moved it is still
    shown; deleted, what held it is.
18. **A pasted Return is the way out its name says (24):** never one
    more Return of a way out with another name.
19. **A guide's unsaved name stays with its guide (26)** when another
    guide is removed.
20. **A constant has a mark the editor can draw (31),** on the button
    beside a number field.
21. **Watch shows values by name and cut to fit (32).**

## Decisions

Asked for on 2026-11-11, before building:

- **The six that lose work first,** then the rest of the review's list
  that needs no decision.
- **Explicit over clever,** in its author's words that day: a fix never
  makes the editor do something it was not asked to do, and when in doubt
  it refuses with a message that says why.
- **Twelve findings wait for their author's decision** and are not here:
  2, 4, 7, 11, 13, 14, 18, 22, 25, 27, 29 and 30.
- **A fix that turns out to need a choice** joins them instead of
  choosing.

Chosen while building, for its author to check:

- **A renamed folder's directory takes the new spelling (1).** Leaving
  the files where they were would have stopped the loss, but the folder
  would have come back as Town the next time the project opened.
- **Old and new paths are compared by what is on disk, not by spelling
  (1).** A file is moved to the backups only if no live document's file is
  that same file. On a file system that tells Town from town, nothing
  changes.
- **A refused step is put back by doing its actions again (3).** Windows
  are told of each action taken back and each put back, as of any edit.
  Should putting back be refused too, as when the journal cannot be
  written, the step is split between the undo and redo lists to match the
  documents, and the failure is logged.
- **A cleared folder name is refused in the Project (9),** before
  anything is sent, with "A folder needs a name". The cleared name stays
  in its field, unsaved.
- **The crash key is turned on by `BERAD_TEST_CRASH_KEY` (12),** read
  once when a pane opens. The README says so instead of offering the key.
- **A field keeps a drag to itself (15):** while the button pressed in it
  is held, nothing the pointer passes over takes the held button for its
  own.
- **Input scripts find the quit dialog's buttons where they are drawn
  (8).** They had been found where the clicks landed, so the real-window
  tests passed while a person's click missed.
- **Add local asks too (10).** With a local's name typed and unsaved,
  Add local replaced the form and dropped it, as selecting a node did.
- **A failed save is a lasting notice (6),** as a crashed tab is, and
  takes the notice's place over any other while saves go on failing. It
  gives the first reason and says the changes are kept in the journal. The
  first save that succeeds ends the notice and says "Saved" in the status
  bar for a few seconds; a tab opened later hears nothing of it.
- **Files left out are a lasting notice too (5),** naming every file and
  why. It tells of a file that is not JSON and of one that is JSON but no
  document. A document of a format no plug-in knows is still skipped
  without a word, as [increment 31](0031-no-text-documents.md) decided
  for old text documents.
- **Text that will not fit is refused whole, not cut (16),** typed or
  pasted, and the status bar says how many characters the field holds and
  how many that would have made. Every field's limit now counts
  characters, as the engine's limits do; a name the engine limits in
  bytes is refused by it at Save, with its reason.
- **A time is judged by both its ends as typed (17)** when both wait for
  the same Save.
- **Where a stroke comes back onto the floor, that cell alone is painted
  (19).** The cells between the last one on the floor and the edge it
  left by are not guessed.
- **While picking, the floor list is drawn over the fade (20)** and
  takes clicks; the tools still rest.
- **A step gives up after a day of ticks (36),** or ten minutes of them
  while the clock is stopped, when nothing that waits on it can end. An
  NPC with no behaviour is not stepped at all. Each says why in the Map's
  status bar.
- **The folder shown gives way to the nearest folder that held it (23)**
  when it is deleted, or when it goes by an undo or from another tab,
  where its new path cannot be known.
- **A pasted Return joins the way out of its name (24),** as naming a
  Return does; with no such way out it keeps its port if free and takes a
  new one if not. The status says when a paste adds a way out.
- **A guide's row is matched by what the guide was (26):** when one guide
  goes or comes back, the first row that differs is the one. Two guides
  alike in every part cannot be told apart this way.
- **A constant's mark is a filled diamond (31),** in the colour of
  constants, drawn by the editor. An icon alone on a button sits in its
  middle.
- **A value takes at most three fifths of its row in Watch (32),** its
  path the rest. A value that refers to an NPC, a place, a zone, a
  workflow or a sprite is sent by name.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Capitals of a folder (1) | Saving compares the files of renamed and moved documents with the live ones by what is on disk; a directory that is a live folder's under another spelling is renamed to it, not removed | A test renames Town to town and Empty to empty in a project saved before, and finds both on disk, respelled, holding their documents, and read back | Built |
| 2. A refused undo (3) | Undo and redo apply a step as one change: a refusal reverts what was applied and the step goes back on its list | A test paints two floors in one stroke, deletes one from another window, and finds undo, then redo, refused with the other floor unchanged and the step still there once the floor is back | Built |
| 3. The quit dialog, folded (8) | The quit dialog laid out for clicks and for input scripts on the part of the window that shows, as it is drawn | A test folds the inspector and finds Cancel where it is drawn, and a click there cancelling | Built |
| 4. A cleared folder name (9) | The Project refuses an empty folder name before building the edit | A test clears a folder's name, saves, and finds no folder change sent, the status bar saying why and the name still unsaved | Built |
| 5. The crash key (12) | `Cmd+Shift+K` used only while the frame's switch is on, which a pane sets from `BERAD_TEST_CRASH_KEY` | A test presses the key with the switch off, then on; the real-window crash report test sets the variable and passes | Built |
| 6. Dragging in a field (15) | A field reads whether the button is held before taking the press, and keeps the drag until the button is let go | A test presses in a field and drags: the selection grows, reaches the end past the field's edge, shrinks, and stays after release | Built |
| 7. An unsaved local (10) | The question asked before the inspector leaves a local with unsaved changes, and before Add local | A test types a local's name, selects a node and finds the question; Cancel, Don't save and Save each do as they say | Built |
| 8. A failed save (6) | A notice to every window when a timed save fails, kept while saves fail and ended by the first that succeeds | A test makes a floor's folder unwritable, paints, and finds the notice in a window open and one opened after; then makes it writable and finds the save told and the file written | Built |
| 9. A file that cannot be read (5) | The files skipped on loading kept with their reasons, and told in a notice | A test opens a project with a file holding conflict markers and one that is no document, and finds both named with why | Built |
| 10. Limits in characters (16) | A field's limit counted in characters; a refusal kept by the field and said by the window | Tests fill a field of five with five characters, one accented, and find a sixth and a paste too long refused with the reason | Built |
| 11. A time switch's times (17) | A time's start or end judged with its other end as typed | A test moves a time to start at its old end in one Save, and finds a start typed to meet its end still refused | Built |
| 12. The Map (19, 20, 21, 33, 36) | A stroke that marks leaving the floor; the floor list and its keys while picking; a family change in as many commands as it needs; room for the hint's two lines; a bound on a step | Tests for each: the cells painted, a place picked upstairs, 4,900 walls changed in two commands of one gesture, the hint ending above the heading, a step refused or given up with its reason | Built |
| 13. The folder shown (23) | The place shown following a folder renamed or moved from it, and settling on what held one that is gone | A test shows a folder, then renames, moves and deletes it, and finds what it holds, then what held it | Built |
| 14. A pasted Return (24) | A pasted Return given the port its name has here, or one of its own | A test pastes sold out into a workflow whose found it has the same port, and finds two ways out; pasted again, still two | Built |
| 15. A guide's name (26) | A row kept for each guide, matched by what the guide was | A test types a name for the second of three guides, removes the first, saves, and finds the name on the right guide | Built |
| 16. A constant's mark, and Watch (31, 32) | A diamond icon; values sent by name and written to fit | Tests find the button with the icon and no label, the clerk's zone and behaviour by name, and a long value cut beside its path | Built |
| 17. Tests and checks | The whole suite, and the real-window suite | `make check` passes, and so does the real-window suite | Built |

## Step results

The six that lose work:

- **Capitals of a folder (1):** on a file system that ignores case, Town
  renamed to town left each document's old and new paths naming one file.
  The save wrote the file, then moved it to the backups as the old one,
  and removed the emptied directory as the old folder's.
  `TestRenamingAFoldersCapitalsKeepsWhatItHolds` failed with no plan left
  in its folder, and passes. Commit 5f17e82.
- **A refused undo (3):** when one action of a step was refused, the step
  was dropped and the actions already taken back stayed taken back.
  `TestRefusedUndoAndRedoKeepTheirStep` failed with the second floor left
  unpainted, and passes. Commit cd785c1.
- **The quit dialog, folded (8):** the dialog took its clicks in the size
  the view lays out for, 256 pixels wider than the window's part while
  the inspector is folded, so every button was 128 pixels right of where
  it was drawn: a click on Cancel landed on Quit, or on Don't save.
  `TestQuitDialogIsWhereItIsDrawnWhileTheInspectorIsFolded` failed with
  Cancel drawn at 641 and found at 769, and passes. Commit fbf6d04.
- **A cleared folder name (9):** a folder change with nowhere to go
  deletes the folder, and a cleared name sent one.
  `TestAFolderKeepsItsNameWhenItIsCleared` failed with the change sent,
  and passes. Commit 95bd7d5.
- **The crash key (12):** `TestCrashKeyNeedsItsSwitch` failed with the
  key crashing the frame unasked, and passes. The real-window test
  `TestCrashedTabLeavesAReport` now sets the variable. Commit cc3ca83.
- **Dragging in a field (15):** taking the press clears the held button
  from the frame's input, which the field then read as the button let go,
  so every drag ended in the frame it began. `TestDraggingSelectsText`
  failed with nothing selected, and passes. Commit b92a4ca.

The fifteen more:

- **An unsaved local (10):** the question was only asked for a node's
  form. `TestAnUnsavedLocalIsAskedAbout` failed with no question and the
  typed name no longer counted, and passes. A click on empty canvas was
  found to keep the name already. Commit f600d25.
- **A failed save (6):** `TestAFailedSaveIsToldUntilASaveSucceeds` failed
  waiting for a notice, and passes; `TestAFailedSaveStaysInTheStatusBar`
  holds the window's part. Commit 8a6fd8b.
- **A file that cannot be read (5):**
  `TestAFileThatCannotBeReadIsNamed` failed waiting for a notice, and
  passes. Commit 18d6a59.
- **Limits in characters (16):** `TestALimitCountsCharacters` failed
  with five characters refused by a field of five, and passes;
  `TestARefusedPasteSaysWhy` holds the window's part. Commit 19ba1e2.
- **A time switch's times (17):** `TestATimeMovesLaterInOneSave` failed
  with "A time needs to end after it starts", and passes. Commit 33ffbe5.
- **A stroke off the floor (19):**
  `TestAStrokeThatLeavesTheFloorDoesNotCutAcrossIt` failed with nine
  cells painted for four, and passes. Commit 6044708.
- **Picking on another floor (20):** `TestPickingOnAnotherFloor` failed
  with PageUp ignored, and passes. Commit 362f556.
- **Many walls (21):** `TestFamilyOfManyWalls` failed with one command of
  4,900 cells, and passes. Commit c6fc0a0.
- **The hint for no NPCs (33):** `TestNoNPCsHintClearsTheFloors` failed
  with the hint ending six pixels into the heading, and passes. Commit
  fc5acd9.
- **A step that cannot end (36):** `TestAStepThatCannotEndGivesUp` failed
  with an NPC with no behaviour stepping for ever, and passes. Commit
  44d4f30.
- **The folder shown (23):** `TestTheFolderShownIsFollowed` failed with
  an empty list after a rename, and passes. Commit e18717f.
- **A pasted Return (24):** `TestAPastedReturnKeepsItsOwnWayOut` failed
  with one way out for two names, and passes. Commit 0108e77.
- **A guide's name (26):** `TestAnUnsavedGuideNameStaysWithItsGuide`
  failed with the name on the third guide, and passes. Commit 86115bc.
- **A constant's mark (31):**
  `TestTheButtonBesideANumberShowsAConstantsMark` failed with the label
  the font lacks, and passes. That the diamond shows was not checked by
  eye. Commit 982be99.
- **Watch (32):** `TestWatchedValuesNameWhatTheyReferTo` failed with a
  behaviour's ID, and `TestALongValueIsCutNotWrittenOverItsName` with a
  value written whole; both pass. Commit 1da6942.

Tests and checks:

- `make check` passes on the last fix (1da6942): formatting, lint with no
  findings, vet, and every test with the race detector. It also passed on
  the six alone (b92a4ca).
- All 61 tests of the real-window suite pass on the last fix, and passed
  on the six alone. Each run was from a copy of its commit, so that
  nothing else was compiled in.
- The suite took 13 minutes 15 seconds, and 13 minutes 41 on the six.
  `make gui-test` as written stops after Go's default 10 minutes, at about
  the 49th test, so it was run as `GOFLAGS=-timeout=30m make gui-test`.
- Two earlier runs are not counted. One was given a folder for its
  screenshots that did not exist, so three tests could not read theirs.
  One ran in the folder being worked in: the suite builds the editor
  again for each test, so it compiled later, unfinished work.
- No real-window test was added. What the fixes change on screen was not
  looked at by eye.

## Not in this increment

Waiting for their author's decision, from the review:

- **2.** Files changed outside the editor while it is open are
  overwritten.
- **4.** One bad file, or a journal that no longer fits, stops the whole
  project opening.
- **7.** Escape in a field drops what was typed, without asking.
- **11.** Deleting a zone leaves its floors; deleting a floor its zone
  lists is allowed.
- **13.** Deleting a document closes another tab's unsaved draft of it.
- **14.** Anything in a file the editor does not know is dropped at the
  next save.
- **18.** A line cannot hold literal text in braces.
- **22.** Shrinking a floor asks only about anchors.
- **25.** Deleting a local value that nodes use gives no warning.
- **27.** The Pixel editor opens with "clear" chosen.
- **29.** The clock's panel takes the keyboard while the pointer is over
  it.
- **30.** A crashed Project tab blocks opening anything until a restart.

Moved to them while building:

- **28. Tab between fields.** Tab leaves a field. Moving to the next one
  needs choices: what Tab does at the last field, whether it reaches a
  field scrolled out of view and scrolls to it, and whether Shift+Tab
  goes back.
- **What Undo says.** Undo and redo answer "Undone" and "Redone". A step
  has no name to say: it holds a gesture and its edits. Naming changes
  belongs to a history that can be read, a direction that is not built.

Noticed while fixing, and left:

- **`make gui-test` stops at Go's default 10 minutes;** the real-window
  suite takes about 14 on the computer used.
- **Dialogs a view shows while the inspector is folded** are drawn, and
  take their clicks, in the same place, but 128 pixels right of the
  middle of what shows. Nothing is lost by it.
- **The quit dialog says "Everything is saved"** while saves to disk are
  failing; it speaks of what waits for Save.
