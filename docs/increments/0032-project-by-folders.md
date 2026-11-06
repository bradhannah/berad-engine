# Increment 32: The Project by folders, with Home

**Status:** Planned

The thirty-second increment rebuilds the Project window, after its author
tried increment 30's facets. Two things felt wrong there: two columns side by
side both read as trees, and a tick in one swapped the other for a flat table.
That table did not scale, since a narrowed list of hundreds was a scroll.

The window keeps the hierarchy instead:
- **Home** opens first, with what you were doing.
- **A tree of folders** sits on the left.
- **The main pane** shows the folder you pick, in a view that suits what the
  folder holds: pictures for art, rows for workflows, cards for NPCs and
  zones, waveforms for sounds.
- **The facets** become a bar of dropdowns. They narrow the tree to the
  folders that hold matches, and nothing swaps.

It builds on [increment 30](0030-project-facets.md) and keeps its facets,
counts, saved views, change dates and problem checks. It is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **Most work starts without browsing:** Home shows what you opened and
   changed lately across every kind, what you pinned, and what needs
   attention.
2. **A project of thousands stays a short tree of folders:** the folder tree
   holds folders, never things, and the main pane holds one folder at a
   time.
3. **Each kind is seen the way it is best judged:**
   - sprites and palettes as pictures;
   - workflows as rows with their details;
   - NPCs and zones as cards;
   - sounds as waveforms with their loudness.
4. **Narrowing never changes what the list is:** a tick narrows the tree and
   the pane, and clearing puts everything back.

## Decisions

Agreed by its author on 2026-11-06, from the canvas "Workflow canvas
wireframes", in the rows "The Project, again" and "The Project's main pane",
and in answer to the planning questions:

- **L, Home, from the top left, is kept.**
  - It is the Project's first entry and what the window opens on.
  - It lists recent things across every kind, pinned things, what needs
    attention, and saved views, under a search.
- **P, folders on the left and what's in them on the right, is chosen.**
  - The left is a tree of folders, never things, each saying how many it
    holds.
  - The main pane shows what is in the folder you pick.
- **The main pane's views are as drawn.**
  - Pictures for sprites, tiles and palettes, with a size slider and
    subfolders first.
  - Rows with columns for workflows and templates, subfolders opening in
    place, sorted by a column's heading.
  - Cards for NPCs and zones, and for workflows drawn small.
  - Rows for sounds, with a waveform, length and loudness against the
    target, playing in place.
  - Preview, one large and the rest in a strip, a click away for any folder.
  - A switch picks the view, and each folder remembers it.
- **Narrowing, in this layout, is as drawn.**
  - The facets become a bar of dropdowns over the main pane, each value with
    its count.
  - A tick narrows the folder tree to folders that hold matches, each saying
    how many.
  - Matches, at the top, shows them all, grouped by folder, each group in its
    kind's view.
  - This replaces increment 30's facets column and the table a tick switched
    to.
- **Recents and pins are kept per person,** in the project's `.berad`
  folder, which is not committed. Saved views stay in the project.
- **It is one increment,** with every view built.

To be chosen while building, for its author to check:

- **What the tree holds:**
  - Home first, then World state (Values and Signals) and Settings.
  - Then the kinds as roots, each with its folders: Zones, Workflows, NPCs,
    Sprites, Sounds, Palettes and Templates.
  - Sounds' folders are their categories.
  - Floors in no zone are a folder of Zones of their own.
  - Saved views are listed under the tree as well as on Home.
- **A folder shows what it holds directly:** its subfolders first, then its
  things, never everything below it. Matches and Everywhere are how you see
  across folders.
- **A zone's floors belong to the zone:**
  - In Rows, a zone opens in place to show them.
  - In Cards and Preview, the zone shows them as tabs over its map.
- **What counts as recent:**
  - Every document opened in a tab, which the orchestrator notes as it opens
    one.
  - Every document changed, from the change dates the project already keeps.
  - Today, yesterday and earlier this week, the last 50 kept.
- **How `.berad` stays out of version control:** the editor writes a
  `.gitignore` inside `.berad` that ignores the folder's contents. It never
  touches the project's own `.gitignore`. The architecture has always meant
  `.berad` to be ignored.
- **Pins are not undone with Cmd+Z,** as they are not project data. Unpinning
  is one click.
- **Columns, kind by kind:**
  - Workflows: Runs as, Runs in, Used by, Changed and Problems.
  - NPCs: Lives in, Follows and Changed.
  - Zones: Floors, People and Loop.
  - Sprites: Used by and Changed.
  - Templates and palettes: Changed.
  - Sounds: Length, Loudness and Used by.
- **The facets in the bar:** Kind, Zone, Runs as, Changed, Problems and Tags.
  Folder leaves the facets, since the tree is the folders.
- **Pictures:**
  - Sprites are drawn as the map draws them.
  - Palettes are drawn as strips of their swatches.
  - Templates are drawn as their marks.
  - The slider has three sizes.
- **Cards:**
  - A zone's map is drawn small, one colour per cell, from its ground
    floor's tiles.
  - A workflow is drawn small from its nodes' places, joined by straight
    lines.
- **Sounds:**
  - Waveforms are asked for a few at a time, like the problem checks.
  - Loudness is asked for once for the whole library.
  - A sound's flag shows when it measures outside the target band.
- **The browser is split into parts:** the tree, Home, the bar and each view,
  so it stays within the strict checks.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Home's memory | The orchestrator keeps, per person in `.berad`, what was opened lately, the pins, and each folder's view and sort, with requests to read and change them and a `.gitignore` that keeps `.berad` out of version control | Opening a document is noted, a pin and a folder's view are kept across reopening the project, and the file is ignored | Planned |
| 2. Folders on the left | The tree of Home, World state, Settings and the kinds with their folders and counts; the main pane showing the folder picked; the inspector as before; the browser in parts | Picking a folder shows its subfolders and things; counts include what is below; folders still make, rename, move and delete | Planned |
| 3. Home | Search, recent things by day, pinned things, what needs attention, and saved views | Home lists what was opened and changed, pins and unpins, and opens and reveals what it lists | Planned |
| 4. Rows | Columns kind by kind, a heading row that sorts, subfolders opening in place, zones showing their floors | A folder's rows show their columns and sort; a subfolder opens in place | Planned |
| 5. Narrowing | The bar of facet dropdowns with counts; the tree narrowed to folders with matches, each saying how many; Matches grouped by folder; searching a folder or everywhere; saving and applying views from the bar; increment 30's column and table removed | Ticks narrow the tree and the pane and never offer nothing; clearing puts everything back; a view saves and applies again | Planned |
| 6. Pictures | Sprites drawn as on the map, palettes as swatches, templates as their marks; three sizes; subfolders shown with four of their pictures | A sprite folder shows its sprites at each size, and a subfolder its pictures | Planned |
| 7. Cards | NPCs with their sprite, behaviour, floor and problems; zones with a small map, floors, people and loop; workflows drawn small | Each card shows what it says it does, and a problem shows on its card | Planned |
| 8. Sounds | Play in place, waveforms asked for a few at a time, length, and loudness against the target with too loud and too quiet flagged | A sound plays from its row, its waveform and loudness show, and an offender is flagged | Planned |
| 9. Preview | One large with the rest in a strip, the arrow keys walking it, a zone's floors as tabs | Preview shows the selected thing large and walks the strip | Planned |
| 10. Tests and checks | Unit tests for each step; the real-window tests moved to finding things through folders; a new real-window test of Home, Pictures and narrowing | `make check` passes, and so does the real-window suite | Planned |

## Risks

- **Size:** this is about twice increment 30. The browser is already 8,000
  lines with one large state; splitting it is part of step 2.
- **The real-window tests:** about 50 find things by their row in today's
  tree. Once Home opens first, each must pick a folder, or search, before it
  finds a row. Step 10 moves them all.
- **Subscriptions:**
  - Pictures need each shown sprite's tile and palettes, and zone maps their
    floors.
  - The editor has no way to stop listening to a document, so the window
    asks only for what it shows.
  - It frees pictures it no longer shows.
- **Sounds:**
  - A waveform decodes its file, two at a time, with no copy kept on disk.
  - A folder of a hundred sounds fills in over a few seconds the first time.

## Not in this increment

- **Constants,** which come after this.
- **Sprite frames.** The editor has no animated sprites yet.
- **Recents shared between people,** and anything that knows who a person is.
