# Increment 31: Text documents removed

**Status:** Built

The thirty-first increment takes Text documents out of the editor. They were
a placeholder from the early increments: a body of text and a place on a
map, kept to prove the editor's text field and picking a place from another
window. Real documents prove both now. A game's text will be held by
constants instead, agreed on the same day: named strings, from a sign to long
dialogue, and whole numbers, which the designer sets and a game never
changes. Constants wait for the Project window's new design. It builds on
[increment 30](0030-project-facets.md) and is held to the same strict checks
as [increment 1](0001-foundation.md).

## What it proves

1. **No project gains a text document, and none already saved is lost:** a
   new project or sample has none, and a project that has some opens,
   skipping them and leaving their files where they are.
2. **What they proved is still proved:**
   - typing an accent in a real window, on a zone's name;
   - folders, tags and search, on a workflow;
   - folders, dates and where-used in the orchestrator's tests, on a
     document kind of the tests' own.

## Decisions

Asked for by its author on 2026-11-06, in the branch with the work in
progress: remove Text documents, as they are not helpful.

Agreed by its author on 2026-11-06, alongside, and recorded in
[the editor's features](../EDITOR_FEATURES.md):

- **Constants replace them,** a list of their own beside values. Building
  them waits for the Project window.

Chosen while building, for its author to check:

- **A project's old text documents are skipped, not deleted.** The editor
  already skips a file whose format no plug-in knows, and says so in its
  log. The files stay in `texts/` for their author to keep or delete.
- **The orchestrator's tests have a note of their own:** a document kind
  defined only in the tests, with a body and a place. They test folders,
  dates and where-used on it, without depending on a plug-in a game uses.
- **The text icon and its brown are gone,** as nothing else used them.
- **The real-window tests moved to other documents:**
  - They wait for the Sample zone's row, not the notes', to know the Project
    has loaded.
  - Renaming a zone types an accent the way press-and-hold sends it, as
    writing in the notes did.
  - Deleting and undoing uses a new workflow.
  - Folders, tags and search use the sample's Fetch stock.
  - Tabs open a workflow where they opened the notes.
- **The test of writing in a text document and picking its place is gone.**
  Picking a place on a map is still tested by Go to and by Call workflow's
  place input.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Out of the editor | The plug-in and the Text window go; so do the sample's notes, the Project's Text section, its New menu items and right-click entries, the inspector's and facets' Text kind, and the text icon | The editor builds, with no lint findings | Built |
| 2. Tests moved | Unit tests on other documents and on a note of the tests' own; real-window tests on a zone, a new workflow and Fetch stock | Every unit test passes with the race detector, and so does the real-window suite | Built |
| 3. Records | The editor's features, its README and this record | The records say what is built and what is agreed | Built |

## Step results

- **Out of the editor:** `editor/plugins/textdoc` and `editor/views/textview`
  are gone, and so is everything that named them.
- **Tests moved:**
  - The Project window's unit tests use Ada Clerk and Clerk day where they
    used the notes. The window's tests use a workflow tab.
  - The orchestrator's tests use their own note, saved under `notes/`.
  - The real-window tests changed as listed above.
- **Checks:** `make check` passes with nothing cached. It covers the
  format, lint and vet checks and every unit test with the race detector.
  The real-window suite passes: 58 tests, the 59 of increment 30 less the
  one removed.

## Not in this increment

- **Constants,** after the Project window's new design.
- **Deleting old `texts/` folders** from projects. Their author can delete
  them by hand.
