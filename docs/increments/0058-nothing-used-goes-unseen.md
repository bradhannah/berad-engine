# Increment 58: Nothing used goes unseen

**Status:** Built

Before anything is removed, what goes with it is listed, and what is still
in use is refused, as agreed on 2026-11-13 in "nothing used goes unseen" in
[the editor features](../EDITOR_FEATURES.md#principles). It settles the
review's findings 11, 13, 22 and 25. It builds on
[increment 57](0057-files-are-shared.md), and is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it proves

1. **A zone and its floors go together (11):** the orchestrator refuses
   deleting a floor a zone lists, "Cellar is part of Harbour; take it out
   of Harbour first", and a zone that still lists floors. The Project
   window takes each floor out of the zone before deleting it, and the
   zone last, in one step, and its question names the floors that go.
2. **Another tab's unsaved changes are named (13):** the delete question
   lists each tab holding unsaved changes to what is deleted, "They go
   too", and a tab closed because its document went, by a delete, an undo
   or a redo, says why in a notice.
3. **A floor that shrinks lists all that goes (22):** anchors and their
   uses, NPCs, ambient sounds, stairs, and stairs on other floors that lead
   into the cells removed. A shrink that would leave the player start off
   the floor is refused until the start is moved.
4. **A local value in use is refused (25),** as a world value is, with its
   uses listed and a click selecting each.
5. **Nothing else behaves differently:** the real-window walks pass.

## Decisions

Chosen while building, for its author to check:

- **The rule is for any part and what holds it,** not zones and floors
  alone, since the engine is shared; today a zone's floors are the only
  parts. Its refusal reads "a part and what holds it go together".
- **Other tabs' changes are named by asking the orchestrator,** which
  knows each window's unsaved changes; the window asking is not told of
  its own.
- **A tab closed by a delete says so in a notice with Dismiss,** in every
  tab, and says how many of its changes that waited for Save went with it.
- **A shrink with nothing but NPCs in its way now asks too,** where before
  it took them off the map and said so afterwards.
- **The floors above and below are looked at for stairs** that lead into
  the cells given up: stairs up on the floor below, and stairs down on the
  floor above.
- **A local in use is refused with a list of its nodes** in a dialog;
  Select selects the node chosen, so each can be changed first.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Zones and floors | The orchestrator's refusal, and the floors named in the question | Unit tests; a real-window walk | Built |
| 2. Other tabs' changes | The delete question naming them; a closed tab saying why | Unit tests | Built |
| 3. A floor that shrinks | Everything in the cells listed; the player start refused | Unit tests | Built |
| 4. Local values in use | Refused with the uses listed | Unit tests | Built |
| 5. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Zones and floors:** `TestAZoneAndItsFloorsGoTogether` is refused
  deleting a floor its zone lists and a zone that lists floors, then takes
  each out, deletes them and the zone in one gesture, and one undo brings
  all back with the floors in order. `TestDeleteZoneNamesAllThatGoes` in
  the Project window, and the real-window walk
  `TestDeletingAZoneNamesItsFloors`: "Its 2 floors go with it: Ground
  floor, Upper floor."
- **Other tabs' changes:** `TestOtherTabsUnsavedChangesAreNamed`.
- **A floor that shrinks:** `TestAShrinkListsAllThatGoes` and
  `TestAShrinkKeepsThePlayerStart`. Reading the floors above and below
  while the floor itself was read would have locked the window, so what
  the resize gives up there is read just after.
- **Local values in use:** `TestALocalInUseIsRefused`. The real-window walk
  planned for it was not written: the unit test covers the refusal and the
  list of nodes.
- **The whole suite:** `make check` passes, and all 79 real-window walks pass on increment 60's commit, `60c74c9`, run from a frozen copy as `GOFLAGS=-timeout=40m make gui-test`.
