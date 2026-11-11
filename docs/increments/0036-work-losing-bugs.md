# Increment 36: Bugs that lose work

**Status:** Built

The thirty-sixth increment adds nothing an author can do. It fixes six bugs
that lose or damage an author's work, found by a review of the editor as it
stood after [increment 35](0035-finding-by-words.md), on 2026-11-11. Each
was first shown by a test that failed, then fixed.

It builds on:
- folders kept as real folders on disk, from
  [increment 3](0003-npc-workflows.md);
- undo steps from [increment 1](0001-foundation.md);
- the single window and its quit dialog as of
  [increment 13](0013-single-window.md), and the folding inspector from
  [increment 29](0029-lines-state-and-picking.md).

It is held to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A folder renamed only in its capitals keeps what it holds:** on a
   file system that ignores case, Town renamed to town still holds its
   documents on disk, an empty folder stays, the directory takes the new
   spelling, and all of it reads back.
2. **A refused undo or redo changes nothing and keeps its step:** a stroke
   across two floors, one of them deleted from another window meanwhile,
   leaves the other floor as it was, and undoes once the floor is back.
3. **The quit dialog takes its clicks where it is drawn:** with the
   inspector folded, a click on Cancel cancels.
4. **Clearing a folder's name does not delete the folder:** nothing is
   sent, and the Project says a folder needs a name.
5. **`Cmd+Shift+K` does nothing for an author:** it crashes a tab only
   when a test turns the key on.
6. **Dragging in a field selects text:** from the press to the pointer,
   until the button is let go.

## Decisions

Asked for on 2026-11-11, before building:

- **These six first,** of the review's findings: the ones that lose or
  damage work.
- **Two larger findings wait for their author's decision** and are not
  here: noticing files changed on disk by git or another program, and
  whether Escape in a field should drop what was typed.

Chosen while building, for its author to check:

- **A renamed folder's directory takes the new spelling.** Leaving the
  files where they were would have stopped the loss, but the folder would
  have come back as Town the next time the project opened. The directory
  is renamed on disk, so town reads back as town.
- **Old and new paths are compared by what is on disk, not by spelling.**
  A file is moved to the backups only if no live document's file is that
  same file. This covers capitals, and any other two spellings a file
  system treats as one name. On a file system that tells Town from town,
  nothing changes: the documents move to the new directory as before.
- **A refused step is put back by doing its actions again.** Windows are
  told of each action taken back and of each put back, as of any edit, so
  what they show ends as it began.
- **Should putting back be refused too,** as when the journal cannot be
  written, the step is split: what is still done stays on the undo list,
  and what was taken back goes on the redo list. Both lists then match the
  documents. The failure is logged.
- **A cleared folder name is refused in the Project,** before anything is
  sent, with "A folder needs a name" in the status bar. The cleared name
  stays in its field, unsaved, to type into or revert.
- **The crash key is turned on by `BERAD_TEST_CRASH_KEY`,** read once when
  a pane opens. The README says so instead of offering the key.
- **A field keeps a drag to itself:** while the button pressed in it is
  held, the selection follows the pointer past the field's edge, and
  nothing the pointer passes over takes the held button for its own.
- **Input scripts find the quit dialog's buttons where they are drawn
  too.** They had been found where the clicks landed, so the real-window
  tests passed while a person's click missed.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Capitals of a folder | Saving compares the files of renamed and moved documents with the live ones by what is on disk; a directory that is a live folder's under another spelling is renamed to it, not removed | A test renames Town to town and Empty to empty in a project saved before, and finds both on disk, respelled, holding their documents, and read back | Built |
| 2. A refused undo | Undo and redo apply a step as one change: a refusal reverts what was applied and the step goes back on its list | A test paints two floors in one stroke, deletes one from another window, and finds undo, then redo, refused with the other floor unchanged and the step still there once the floor is back | Built |
| 3. The quit dialog, folded | The quit dialog laid out for clicks and for input scripts on the part of the window that shows, as it is drawn | A test folds the inspector and finds Cancel where it is drawn, and a click there cancelling | Built |
| 4. A cleared folder name | The Project refuses an empty folder name before building the edit | A test clears a folder's name, saves, and finds no folder change sent, the status bar saying why and the name still unsaved | Built |
| 5. The crash key | `Cmd+Shift+K` used only while the frame's switch is on, which a pane sets from `BERAD_TEST_CRASH_KEY` | A test presses the key with the switch off, then on; the real-window crash report test sets the variable and passes | Built |
| 6. Dragging in a field | A field reads whether the button is held before taking the press, and keeps the drag until the button is let go | A test presses in a field and drags: the selection grows, reaches the end past the field's edge, shrinks, and stays after release | Built |
| 7. Tests and checks | The whole suite, and the real-window suite | `make check` passes, and so does `make gui-test` | Built |

## Step results

- **Capitals of a folder:** on a file system that ignores case, Town
  renamed to town left each document's old and new paths naming one file.
  The save wrote the file, then moved it to the backups as the old one,
  and removed the emptied directory as the old folder's. The orchestrator
  now asks the disk whether an old path is a live document's file before
  moving it, and renames a directory that is a live folder's under
  another spelling. `TestRenamingAFoldersCapitalsKeepsWhatItHolds` failed
  with no plan left in its folder, and passes. Commit 5f17e82.
- **A refused undo:** when one action of a step was refused, the step was
  dropped and the actions already taken back stayed taken back.
  `TestRefusedUndoAndRedoKeepTheirStep` failed with the second floor left
  unpainted, and passes. Commit cd785c1.
- **The quit dialog, folded:** the dialog took its clicks in the size the
  view lays out for, 256 pixels wider than the window's part while the
  inspector is folded, so every button was 128 pixels right of where it
  was drawn: a click on Cancel landed on Quit, or on Don't save.
  `TestQuitDialogIsWhereItIsDrawnWhileTheInspectorIsFolded` failed with
  Cancel drawn at 641 and found at 769, and passes. Commit fbf6d04.
- **A cleared folder name:** a folder change with nowhere to go deletes
  the folder, and a cleared name sent one.
  `TestAFolderKeepsItsNameWhenItIsCleared` failed with the change sent,
  and passes. Commit 95bd7d5.
- **The crash key:** `TestCrashKeyNeedsItsSwitch` failed with the key
  crashing the frame unasked, and passes. The real-window test
  `TestCrashedTabLeavesAReport` now sets the variable. Commit cc3ca83.
- **Dragging in a field:** taking the press clears the held button from
  the frame's input, which the field then read as the button let go, so
  every drag ended in the frame it began. `TestDraggingSelectsText`
  failed with nothing selected, and passes. Commit b92a4ca.
- **Tests and checks:** see below.

Tests and checks:

- `make check` passes on the six fixes: formatting, lint with no findings,
  vet, and every test with the race detector.
- All 61 tests of the real-window suite pass on the six fixes, run from a
  copy of that commit (b92a4ca) so that nothing else was compiled in.
- The suite took 13 minutes 41 seconds. `make gui-test` as written stops
  after Go's default 10 minutes, at about the 49th test, so it was run as
  `GOFLAGS=-timeout=30m make gui-test`.
- Two earlier runs are not counted. One was given a folder for its
  screenshots that did not exist, so three tests could not read theirs.
  One ran in the folder being worked in: the suite builds the editor
  again for each test, so it compiled later, unfinished work.

## Not in this increment

- **Noticing files changed on disk** by git or another program while the
  editor is open. It waits for its author's decision.
- **Whether Escape in a field drops what was typed.** It waits for its
  author's decision.
- **The review's other findings.**
- **Dialogs a view shows while the inspector is folded** are drawn, and
  take their clicks, in the same place, but 128 pixels right of the
  middle of what shows. Noticed while fixing the quit dialog; nothing is
  lost by it.
