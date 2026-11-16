# Increment 59: Keys in a field do only what the field shows

**Status:** Built

Esc, Cmd+Z and Tab in a field act on the field alone, and the keyboard is
where the text cursor is, as agreed on 2026-11-13 in "keys in a field do
only what the field shows" in
[the editor features](../EDITOR_FEATURES.md#principles). It settles the
review's findings 7, 44, 28 and 29. It builds on
[increment 58](0058-nothing-used-goes-unseen.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

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

## Decisions

Chosen while building, for its author to check:

- **Esc in a search box still clears the search,** since a search waits
  for no Save; everywhere else Esc keeps what was typed.
- **Tab moves among the fields a view keeps together,** in the order they
  were made, skipping one not shown; the node inspector scrolls the field
  into view. Tab from the last field stays there and says "This is the last
  field. Esc leaves it."; Shift+Tab from the first says so too.
- **Cmd+Shift+Z stops at the field as Cmd+Z does,** saying "Nothing more to
  redo in this field".
- **A press outside the clock's panel closes it and still reaches what is
  under it,** as the panel is not a dialog. A press on the window bar
  leaves it open, so its hour steps can be used while it shows. Esc closes
  it only while no field has the keyboard; otherwise Esc leaves the field.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Esc | Esc leaves the field and keeps the draft | Unit tests; a real-window walk | Built |
| 2. Cmd+Z | A field's undo stops at the field | Unit tests; a real-window walk | Built |
| 3. Tab | Tab and Shift+Tab move through a panel's fields | Unit tests | Built |
| 4. The clock's keys | Keys follow the text cursor; Esc and a click outside close the panel | Unit tests | Built |
| 5. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Esc:** `TestEscapeKeepsWhatWasTyped`; `TestDraftLifecycle` now holds
  that Esc leaves the field with its draft. Two tests that used Tab or Esc
  to leave a field now use Esc and Revert.
- **Cmd+Z:** `TestClipboardAndUndoKeys` holds that undo with no typing left
  stops at the field and says so.
- **Tab:** `TestTabMovesBetweenFields`.
- **The clock's keys:** `TestKeysStayWithTheTabInFront`.
- **In the real window:** `TestKeysInAFieldStayWithIt` types a Wait's
  duration, presses Esc, which says "Left the field. What you typed waits
  for Save.", then presses Cmd+Z until it says "Nothing more to undo in
  this field. Esc leaves it, to undo saved changes." with the Wait as it
  was. Tab and the clock's panel were left to their unit tests.
- **The whole suite:** `make check` passes, and all 79 real-window walks pass on increment 60's commit, `60c74c9`, run from a frozen copy as `GOFLAGS=-timeout=40m make gui-test`.
