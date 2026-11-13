# Increment 58: Nothing used goes unseen

**Status:** Planned

Before anything is removed, what goes with it is listed, and what is still
in use is refused, as agreed on 2026-11-13 in "nothing used goes unseen" in
[the editor features](../EDITOR_FEATURES.md#principles). It settles the
review's findings 11, 13, 22 and 25. It builds on
[increment 57](0057-files-are-shared.md), and is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it is to prove

1. **A zone and its floors go together (11):** the orchestrator refuses
   deleting a floor a zone lists, "Cellar is a floor of Harbour. Take it
   out of Harbour first", and a zone that still has floors unless they go
   in the same step. The Project window's question names the floors that go.
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

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Zones and floors | The orchestrator's refusal, and the floors named in the question | Unit tests | Planned |
| 2. Other tabs' changes | The delete question naming them; a closed tab saying why | Unit tests | Planned |
| 3. A floor that shrinks | Everything in the cells listed; the player start refused | Unit tests | Planned |
| 4. Local values in use | Refused with the uses listed | Unit tests; a real-window walk | Planned |
| 5. Checks | `make check` and the real-window walks | All pass | Planned |
