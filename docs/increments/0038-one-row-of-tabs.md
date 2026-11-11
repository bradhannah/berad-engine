# Increment 38: One row of tabs

**Status:** Built

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

Chosen while building, for its author to check:

- **Tabs stay in the order they opened,** the Project's first. A tab is at
  most 220 points wide and at least 120. Past that the last tabs go under a
  "3 more" tab, which lists them and stands in front while one of them is.
- **"live" is written in orange** on a tab whose Map previews.
- **Launch shows `Cmd+R`** in the subdued grey, in the type for keys.
- **`Cmd+1` to `Cmd+9` count from the Project's tab;** `Cmd+9` is the ninth
  tab, not the last.
- **Neighbours stop at the first and last tab** and do not wrap round; the
  status bar says "This is the last tab".
- **`Ctrl+Tab` goes to the tab in front before this one;** pressed again it
  comes back.
- **The window's tab keys match every modifier,** so `Ctrl+Tab` is never
  read as Tab, and `Cmd+Alt+1` is left to the view.
- **A place on the trail is a tab and what its focused pane has selected.**
  - Selecting nothing makes no place of its own.
  - A place with nothing selected takes the first selection made in it.
  - Closed tabs leave the trail, which keeps at most 100 places.
  - So far only a Workflow says what is selected, its node. Other views
    come back as tabs.
- **Back puts a selection back only where nothing waits for Save.**
  Otherwise the tab comes forward as it is and its status bar says the
  selection was kept.
- **Quick open's search lists nothing until something is typed,** and at
  most 200 rows.
  - Settings and folders are not listed, since they open no tab.
  - Enter with nothing selected opens the first row.
  - An NPC with no behaviour is refused, with the reason in the sheet.
- **Quick open leaves the Project's own search and selection as they
  were.** While the sheet shows, the window's keys (`Cmd+S`, `Cmd+Q`,
  `Cmd+R` and the tab keys) do nothing.
- **`Cmd+Enter` is refused in the sheet** when the tab in front already
  shows two windows. A window already open in a tab of its own moves beside
  the tab in front, and its old tab goes.
- **A document that opens in no tab now says so** in the Project's status
  bar, where before nothing happened.
- **The Map's key sheet lists the new keys.** No other view has a key sheet
  yet.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. One row | The bar as one row of tabs in the order they opened: icon, tint, name, unsaved dot, live mark and close box; tabs that shrink to a least width; a "more" tab and its list; Launch's key | Unit tests of the layout at several widths and counts; screenshots | Built |
| 2. Tab keys | `Cmd+1`–`9`, `Ctrl+Tab` and `Cmd+Shift+[`/`]` from any tab, refused with a reason where there is nowhere to go | Unit tests; an input script that moves between tabs by key | Built |
| 3. Back and Forward | A trail of where the author has been: the tab, and the selection where a view can say it; `Cmd+[` and `Cmd+]` | Unit tests; an input script that steps back to a workflow's node | Built |
| 4. Quick open | The Project window's search in a sheet over any tab; Enter opens, `Cmd+Enter` opens beside the tab in front, Esc closes | Unit tests; an input script that opens a workflow from a Map's tab | Built |
| 5. Checks | `make check` and the real-window walks; screenshots at 1512 × 916 | All pass | Built |

## Step results

- **One row:**
  - `TestOneRowOfTabs` and `TestTabsThatDoNotFitGoUnderMore` replace the
    tests of tabs grouped by kind.
  - Screenshots at 1512 × 916 and 900 × 700 show five tabs on one row and,
    at 900, two under "2 more", with "live" on a previewing Map.
  - The real-window test once named for grouping is now
    `TestEveryDocumentHasATab`.
- **Tab keys:** `TestKeysMoveBetweenTabs` covers the keys in a unit test.
  A real-window test of the same name moves between a workflow, a Map and
  the Project by key from the workflow's tab.
- **Back and Forward:**
  - `TestBackAndForwardStepThroughWhereTheAuthorHasBeen` steps along a
    trail of tabs and nodes. It keeps a selection that waits for Save and
    forgets a closed tab.
  - In the real window, `TestBackReturnsToANode` selects two nodes of
    Clerk's day, goes to the Project, and steps back to each and forward
    again.
- **Quick open:**
  - `TestQuickOpenFindsAndOpens` covers the sheet in the Project.
  - `TestQuickOpenShowsOverTheTabInFront` covers the window's part: where
    the sheet shows, who takes the input, and a window put beside the Map,
    new or already open.
  - In the real window, `TestQuickOpenFromAMap` opens Clerk's day from a
    Map's tab with Enter and Fetch stock beside the Map with `Cmd+Enter`.
    Its screenshot shows the sheet over the Map.
- **Checks:** `make check` passes on every step. All 64 real-window walks
  pass on the last step's commit (the 61 there were and the three above),
  run as `GOFLAGS=-timeout=30m make gui-test` from a frozen copy, in 13½
  minutes. Not checked: a Retina panel by eye, and the "more" tab's list
  with a mouse rather than a test.

## Not in this increment

The clock's controls in the bar come with "starting a preview opens
nothing". Panels that fold, the window reopening as it was left, and
drawing at the screen's own resolution come with "every panel folds".
