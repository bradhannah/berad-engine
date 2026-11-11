# Increment 38: One row of tabs

**Status:** Planned

Going from a map to a workflow becomes one click or one key. Every open
document gets a tab of its own on one row, as agreed on 2026-11-11 in
[the editor features](../EDITOR_FEATURES.md#principles) ("one row of tabs").
Keys choose a tab by position, the one last used and its neighbours. Back
and Forward step through where the author has been. Quick open, from any
tab, finds and opens anything the Project window's search finds. It builds on
[increment 37](0037-desk-and-stage.md) and
[increment 35](0035-finding-by-words.md), and is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it proves

1. **One row, every document a tab:** the bar keeps one row at every
   count of tabs and kinds. Each tab shows its type's icon, its tint, its
   name, a dot while it holds changes that wait for Save, and "live" in
   orange while it previews. When tabs no longer fit at their least width,
   the last of them go under a "more" tab that lists them.
2. **Keys move between tabs:**
   - `Cmd+1` to `Cmd+9` choose a tab by position;
   - `Ctrl+Tab` chooses the tab last used;
   - `Cmd+Shift+[` and `Cmd+Shift+]` choose its neighbours.

   A key with nowhere to go changes nothing and says why.
3. **Back and Forward:** `Cmd+[` and `Cmd+]` step through the tabs the
   author has been in, and what was selected in each where the view can
   say. A selection is put back only where nothing waits for Save.
4. **Quick open:** `Cmd+P` in any tab opens the Project window's search in
   a sheet over the window. Enter opens what is chosen, as a double-click
   in the Project window does. `Cmd+Enter` opens it beside the tab in
   front. Esc, or a click outside the sheet, closes it and changes nothing.
5. **Nothing else behaves differently:** the real-window walks pass.

## Decisions

Agreed by its author on 2026-11-11 (see the editor features, "explicit over
clever" and "one row of tabs"):

- **One row of tabs.** Every open document is a tab. A tab keeps its tint,
  shows its type by its icon, a dot for waiting for Save and "live" in
  orange for previewing. Tabs shrink together, and the last go under a
  "more" tab.
- **Keys:** `Cmd+1` to `9`, `Ctrl+Tab`, `Cmd+Shift+[` and `]`, and `Cmd+[`
  and `Cmd+]` for back and forward.
- **Quick open** is `Cmd+P` from any tab. `Cmd+Enter` opens a result beside
  the tab in front.
- **Launch shows its key.**
- **Explicit over clever:** nothing opens, closes or switches except by a
  key or button the author pressed.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. One row | The bar as one row of tabs in the order they opened: icon, tint, name, unsaved dot, live mark and close box; tabs that shrink to a least width; a "more" tab and its list; Launch's key | Unit tests of the layout at several widths and counts; screenshots | Planned |
| 2. Tab keys | `Cmd+1`–`9`, `Ctrl+Tab` and `Cmd+Shift+[`/`]` from any tab, refused with a reason where there is nowhere to go | Unit tests; an input script that moves between tabs by key | Planned |
| 3. Back and Forward | A trail of where the author has been: the tab, and the selection where a view can say it; `Cmd+[` and `Cmd+]` | Unit tests; an input script that steps back to a workflow's node | Planned |
| 4. Quick open | The Project window's search in a sheet over any tab; Enter opens, `Cmd+Enter` opens beside the tab in front, Esc closes | Unit tests; an input script that opens a workflow from a Map's tab | Planned |
| 5. Checks | `make check` and the real-window walks; screenshots at 1512 × 916 | All pass | Planned |

## Not in this increment

The clock's controls in the bar come with "starting a preview opens
nothing". Panels that fold, the window reopening as it was left, and
drawing at the screen's own resolution come with "every panel folds".
