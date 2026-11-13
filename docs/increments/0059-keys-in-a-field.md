# Increment 59: Keys in a field do only what the field shows

**Status:** Planned

Esc, Cmd+Z and Tab in a field act on the field alone, and the keyboard is
where the text cursor is, as agreed on 2026-11-13 in "keys in a field do
only what the field shows" in
[the editor features](../EDITOR_FEATURES.md#principles). It settles the
review's findings 7, 44, 28 and 29. It builds on
[increment 58](0058-nothing-used-goes-unseen.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it is to prove

1. **Esc keeps what was typed (7):** it leaves the field, and what was typed
   waits for Save, as a click elsewhere leaves it. The status bar says
   "Left the field. What you typed waits for Save."
2. **Cmd+Z stops at the field (44):** with nothing left in the field's own
   history it changes nothing and says "Nothing more to undo in this field.
   Esc leaves it, to undo saved changes."
3. **Tab moves between fields (28):** Tab to the next field in the panel,
   Shift+Tab to the one before, scrolling it into view; at the last or first
   field it stays and says so. What was typed waits for Save.
4. **Keys stay with the tab in front (29):** the clock's panel takes keys
   only once its field is clicked, wherever the pointer rests; Esc, or a
   click outside, closes the panel.
5. **Nothing else behaves differently:** the real-window walks pass.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Esc | Esc leaves the field and keeps the draft | Unit tests | Planned |
| 2. Cmd+Z | A field's undo stops at the field | Unit tests | Planned |
| 3. Tab | Tab and Shift+Tab move through a panel's fields | Unit tests; a real-window walk | Planned |
| 4. The clock's keys | Keys follow the text cursor; Esc and a click outside close the panel | Unit tests; a real-window walk | Planned |
| 5. Checks | `make check` and the real-window walks | All pass | Planned |
