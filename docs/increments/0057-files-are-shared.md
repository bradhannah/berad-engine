# Increment 57: The files are shared

**Status:** Built

The editor never writes over a file it has not seen, and says what it
cannot read rather than refusing to open, as agreed on 2026-11-13 in "the
files are shared" in [the editor features](../EDITOR_FEATURES.md#principles).
It settles the review's findings 2, 4 and 14, fixes two losses found
while tracing them, 53 and 54, and builds 55, notices that wait their turn,
since the questions it asks are notices with buttons. It builds on
[increment 55](0055-reopens-as-left.md), and is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it proves

1. **A file changed outside is never written over (2):**
   - before writing a document, the editor checks the file is as it read
     it, by size, modified time and content;
   - a document with nothing waiting in the editor takes in the file's
     version, and History says "Took in Clerk's day as changed on disk";
   - a document changed in both places asks Keep mine, Take the file's or
     Later. The version not kept goes to the project's backups, and Later
     writes nothing, keeping the changes in the journal under a lasting
     notice.
2. **A project that cannot open as it is says why (4):** a window names
   each file that cannot be read and why, and a journal that no longer fits
   the files with how many changes it holds, and offers Open without them
   or Quit. A file left out is never written over; a journal set aside is
   moved whole to the backups.
3. **What the editor does not know is said (14):** on opening, a lasting
   notice names each file and field the next save would drop, and each such
   file is copied to the backups before that save.
4. **Journal changes to a left-out document are kept (53),** not lost when
   the next save empties the journal.
5. **A required document that cannot be read is not written over (54):** a
   fresh one stands in for it while the editor is open, said in a notice,
   and is never written.
6. **Notices wait their turn (55):** several last at once, each under its
   own key; windows show the newest, with how many there are, a button that
   opens History, where each is kept. A notice can carry buttons, such as
   Keep mine, and Dismiss on one that tells of something already done.
7. **Nothing else behaves differently:** the real-window walks pass.

## Decisions

Chosen while building, for its author to check:

- **The question for a file changed in both places is a notice,** not a
  dialog, with Keep mine and Take the file's: it shows in every tab and
  lasts until answered, and leaving it is Later. A file removed outside
  the editor while it has changes here offers Keep mine alone, which writes
  it back; removing it here too is done in the Project window.
- **Changes in a window's field count as changes here,** as the document's
  own do, so a file changed outside while a field holds typing is asked
  about, not taken in.
- **The editor looks at the files at each save,** every two seconds while
  it runs, by size and time first and by their bytes only when those
  differ. A file written again with the same bytes is not changed.
- **A file taken in loses its undo steps,** which were for the version it
  replaced, and Take the file's writes the file again, so the journal's
  changes to the editor's version are never laid over it.
- **The opening question's Quit is its first button,** so Enter changes
  nothing. Until it is answered no change is taken and nothing is written,
  and quitting writes nothing either.
- **Unknown fields are found by encoding each document again** and
  comparing its keys with the file's, leaving out keys whose value is
  empty, since the editor may leave those out itself. The notice names
  three files and four fields of each at most.
- **The versions kept go to folders of their own in the backups:**
  `changed`, `journal` and `unknown-fields`.
- **Not in this increment:** files added outside the editor while it is
  open appear when the project is opened again.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Seen files | What each document's file was when read or written, checked before each write | Unit tests | Built |
| 2. Taking in and asking | A file changed outside taken in, or asked about | Unit tests; a real-window walk | Built |
| 3. Opening as it is | Bad files and a journal that does not fit named in a window, Open without them or Quit | Unit tests; a real-window walk | Built |
| 4. Unknown fields | Said at opening, backed up before the save that drops them | Unit tests | Built |
| 5. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Taking in and asking:**
  - `TestAFileChangedOutsideIsTakenIn` renames a floor in its file: the
    window is sent it again saying "Took in Cellar as changed on disk", its
    undo steps go, and the project index names it Cellar, one revision on.
  - `TestAFileChangedInBothPlacesKeepsMineWhenAsked` and
    `TestAFileChangedInBothPlacesTakesTheFilesWhenAsked` hold a file while a
    window has typing for it: nothing is written until the notice is
    answered, and the version not kept is in `backups/changed`.
  - The real-window walk `TestAFloorChangedOnDiskIsTakenIn` renames the
    sample's ground floor on disk while a Map shows it; its status bar says
    "Took in Cellar floor as changed on disk". Its first run found the
    index rebuilt from nothing under the windows showing it, which started
    its revisions again; it is now brought up to date in place.
- **Opening as it is:** `TestAJournalThatDoesNotFitIsAskedAbout`,
  `TestANewerFileIsLeftOutWhenItsAuthorSays`,
  `TestAFileThatCannotBeReadIsNamed` and
  `TestARequiredDocumentThatCannotBeReadStandsIn`; and the real-window walk
  `TestAProjectThatCannotOpenAsksFirst`, a file with conflict markers in the
  sample, asked about in the Project tab, told after Open without them, the
  notice ended by its Dismiss, and the file never written over.
- **Unknown fields:** `TestFieldsTheEditorDoesNotKnowAreToldAndKept`.
- **Notices:** `TestNoticesWaitTheirTurn`.
- **The whole suite:** `make check` passes, and all 79 real-window walks pass on increment 60's commit, `60c74c9`, run from a frozen copy as `GOFLAGS=-timeout=40m make gui-test`.
