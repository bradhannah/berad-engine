# Increment 39: A history that can be read

**Status:** Built

Every change becomes a sentence the author can read back, and Undo says
which sentence it undid, as agreed on 2026-11-11 in
[the editor features](../EDITOR_FEATURES.md#principles) ("a history that can
be read"). A refusal stays in the status bar until it is dismissed, and every
tab keeps a list of what it was told. The Map's status bar always names the
tool in hand and what the pointer is over. It builds on
[increment 38](0038-one-row-of-tabs.md), and is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it proves

1. **Every change is a sentence:**
   - an undo step keeps a sentence saying what kind of change, to what,
     and where;
   - every kind of edit has words of its own, and a change says its kind
     and its document.
2. **Undo and Redo say what they did:** `Cmd+Z` answers "Undid: Joined Say
   to Check value" in place of "Undone", and `Cmd+Shift+Z` "Redid: …".
3. **A refusal stays:** a change, undo or redo the orchestrator refuses
   stays in the status bar, in red with its reason, until Dismiss is
   pressed. Other messages still fade.
4. **Each tab keeps what it was told:** History in the status bar opens a
   list of every message and refusal the tab was given, newest first, with
   when.
5. **The Map names the tool in hand:** its status bar reads the floor, the
   tool and the tile, and what the pointer is over, at all times.
6. **Nothing else behaves differently:** the real-window walks pass.

## Decisions

Agreed by its author on 2026-11-11 (see the editor features, "a history
that can be read" and "explicit over clever"):

- **Every change is said afterwards in words, and kept.** The sentence
  Undo answers with is the one the change was said with.
- **The list opens from the status bar,** one for each pane, as undo is.
- **A refusal stays until dismissed;** other messages still fade.
- **The status bar always names the tool in hand** and what the pointer
  is over.

Chosen while building, for its author to check:

- **The orchestrator says every change.** It builds the sentence from the
  kind of edit and the document's name, as "Painted Ground floor", "Added
  nodes to Clerk's day" or "Renamed Town to Harbour". It says it before
  applying the change, so a rename can name what it renamed. A test checks
  that every kind of edit the plug-ins make has words of its own.
- **Views do not yet say their own sentences.** A sentence says the kind of
  change and the document, not which node or how many cells; that needs
  each view's commands to carry a sentence of their own.
- **A gesture keeps the sentence of its first command.**
- **Every change a tab makes is said in its status bar and kept in
  History.** The view's own answer, when it gives one, comes after.
- **"Nothing to undo" and "Nothing to redo" are plain messages,** not
  refusals.
- **What counts as a refusal so far:** a command, undo or redo the
  orchestrator refuses. Messages a view writes itself stay plain for now,
  even those that refuse, such as "This floor is in no zone".
- **The status bar shows, in order:** a fresh message, then a refusal until
  it is dismissed, then a notice.
- **Dismiss is a button with its word,** not a ×.
- **History keeps the last 200 messages of each tab,** newest first, each
  with its time. History opens it; History again, Esc or a click outside
  closes it. A line cannot yet be clicked to select what it names.
- **Only the Map names the tool in hand so far.** While it does, that line
  takes the place of the command-to-event readout at the status bar's
  right.
- **A notice goes into History** when it changes.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Sentences | A sentence for every change, kept with its undo step; Undo and Redo answering with it | Unit tests in the orchestrator and the frame | Built |
| 2. Every kind of edit | Words for each of the 66 kinds of edit, and for the project's documents and folders by what they are about | A test that every kind has words | Built |
| 3. Refusals that stay | A refusal kept in the status bar until Dismiss | Unit tests of the frame | Built |
| 4. The list | History in the status bar, and the list it opens | Unit tests; a real-window walk and its screenshot | Built |
| 5. The tool in hand | The Map's status bar naming the floor, tool, tile and what the pointer is over | Unit tests; the same walk | Built |
| 6. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Sentences:** `TestChangesAreSaidAndUndoSaysWhich` paints a stroke of
  two cells as one gesture. The paint is said "Painted Floor 1", Undo and
  Redo answer with it, and with nothing left to undo nothing is said.
- **Every kind of edit:** `TestEveryEditHasWords` builds the editor's
  plug-ins and finds words for every kind of edit they make.
- **Refusals and the list:** `TestARefusalStaysAndHistoryKeepsWhatWasSaid`
  covers these:
  - a refusal outlasts a later message, and Dismiss takes it away;
  - the log keeps all three messages, the refusal marked;
  - History opens the list, and Esc closes it without the key going
    further.
- **The tool in hand:** `TestTheStatusBarNamesTheToolInHand` reads
  "Ground · Ground · Grass" and the cell under the pointer, and no tile
  for Anchors.
- **In the real window:** `TestUndoSaysWhatItTookBack` paints a stroke of
  road and undoes it. Its screenshot shows History holding "Painted Ground
  floor" and "Undid: Painted Ground floor", and the status bar reading
  "Ground floor · Ground · Road".
- **Checks:** `make check` passes. All 65 real-window walks pass on the
  increment's code commit, run as `GOFLAGS=-timeout=30m make gui-test` from
  a frozen copy. Not checked by eye: a refusal in the status bar with its
  Dismiss button, and History in a narrow window.

## Not in this increment

- **Sentences said by the views themselves:** which node and how many
  cells.
- **A line in History selecting what it names.**
- **Refusals the views write themselves.**
- **The tool in hand in the Pixel editor and the Workflow.**
