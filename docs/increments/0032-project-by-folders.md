# Increment 32: The Project by folders, with Home

**Status:** Built

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

Chosen while building, for its author to check:

- **What the places column holds:**
  - Home first, then World state and Settings.
  - Then the kinds, each with its folders: Zones, Workflows, NPCs, Sounds,
    Sprites, Palettes and Templates.
  - Sounds' folders are their categories, and floors in no zone are a folder
    of Zones of their own.
  - Saved views are listed under the places as well as on Home.
- **A folder shows what it holds directly:** its folders first, then its
  things. Matches is how you see across folders: a search or a tick shows
  it, and picking a place while narrowed shows what is kept there.
- **A zone's floors belong to the zone:**
  - In Rows, a zone opens in place to show them.
  - In Cards, each floor is a card of its own after its zone's.
  - In Preview, the zone shows them as tabs over its map.
- **What counts as recent:**
  - Every document opened in a tab, which the orchestrator notes as it opens
    it, the last 50 kept.
  - Every document changed, from the change dates the project already keeps.
  - Home lists the 20 newest, under Today, Yesterday, Earlier this week and
    Earlier, each saying where it lives.
- **A thing is listed once on Home:** pinned, then needing attention, then
  recent.
- **Pins:**
  - Right-click a thing for Pin to Home or Unpin from Home.
  - On Home and in Matches, the menu adds Show in folder.
  - Pins are not undone with Cmd+Z, as they are not project data.
- **How `.berad` stays out of version control:** the editor writes a
  `.gitignore` inside `.berad` that ignores the folder's contents. It never
  touches the project's own `.gitignore`. The architecture has always meant
  `.berad` to be ignored.
- **Columns, kind by kind:**
  - Workflows: Runs as, Runs in, Used by, Changed and Problems.
  - NPCs: Lives in, Follows and Changed.
  - Zones: Floors, People and Loop.
  - Sprites: Used by, the NPCs showing it, and Changed.
  - Templates and palettes: Changed.
  - Sounds: Play, Waveform, Length and Loudness.
  - Columns that do not fit beside a name 200 pixels wide are left out, the
    later ones first. A heading sorts, again for the other way, and folders
    stay first.
- **The facets in the bar:** Kind, Zone, Runs as, Changed, Problems and
  Tags. Folder leaves the facets, since the places are the folders. They go
  on a second line when they do not fit beside the search.
- **Narrowing leaves the selection alone:** Matches shows as it starts, and
  the place shown before comes back as it ends.
- **Pictures:**
  - Sprites are drawn as the map draws them, palettes as rows of their
    swatches, and templates as their marks.
  - A sprite folder shows four of its sprites.
  - Three sizes, S, M and L, buttons rather than a slider; M by default.
- **Cards:**
  - A zone's ground floor is drawn small, one colour per cell, from its
    tiles.
  - A workflow is drawn small from its nodes' places, joined by straight
    lines.
- **Sounds say what the Audio window says.** A sound's loudness uses the
  Audio window's own wording and rules, such as "Not measured yet" and
  "Evened out", rather than a band of its own.
  - Waveforms are asked for two at a time.
  - Loudness is asked for once for the whole library, and again when it
    changes.
- **Dragging:** a row dragged out of the main pane drops on a folder or a
  kind among the places.
- **Making things:** a kind's [+] makes a thing in the folder picked, when
  it is that kind's, and shows the kind otherwise. A document just made, or
  selected anywhere, shows its place.
- **Scripts name things as before.** A script finding a row by its name
  shows the place holding it first. The real-window tests found things this
  way, and most needed no change.
- **The browser is in parts:** `places`, `content`, `bar`, `home`, `rows`,
  `grid`, `views`, `pictures`, `cards`, `sounds` and `preview`. Each is a
  file of its own, so the window stays within the strict checks.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Home's memory | The orchestrator keeps, per person in `.berad`, what was opened lately, the pins, and each folder's view and sort, with requests to read and change them and a `.gitignore` that keeps `.berad` out of version control | Opening a document is noted, a pin and a folder's view are kept across reopening the project, and the file is ignored | Built |
| 2. Folders on the left | The tree of Home, World state, Settings and the kinds with their folders and counts; the main pane showing the folder picked; the inspector as before; the browser in parts | Picking a folder shows its subfolders and things; counts include what is below; folders still make, rename, move and delete | Built |
| 3. Home | Search, recent things by day, pinned things, what needs attention, and saved views | Home lists what was opened and changed, pins and unpins, and opens and reveals what it lists | Built |
| 4. Rows | Columns kind by kind, a heading row that sorts, subfolders opening in place, zones showing their floors | A folder's rows show their columns and sort; a subfolder opens in place | Built |
| 5. Narrowing | The bar of facet dropdowns with counts; the tree narrowed to folders with matches, each saying how many; Matches grouped by folder; searching a folder or everywhere; saving and applying views from the bar; increment 30's column and table removed | Ticks narrow the tree and the pane and never offer nothing; clearing puts everything back; a view saves and applies again | Built |
| 6. Pictures | Sprites drawn as on the map, palettes as swatches, templates as their marks; three sizes; subfolders shown with four of their pictures | A sprite folder shows its sprites at each size, and a subfolder its pictures | Built |
| 7. Cards | NPCs with their sprite, behaviour, floor and problems; zones with a small map, floors, people and loop; workflows drawn small | Each card shows what it says it does, and a problem shows on its card | Built |
| 8. Sounds | Play in place, waveforms asked for a few at a time, length, and loudness against the target with too loud and too quiet flagged | A sound plays from its row, its waveform and loudness show, and an offender is flagged | Built |
| 9. Preview | One large with the rest in a strip, the arrow keys walking it, a zone's floors as tabs | Preview shows the selected thing large and walks the strip | Built |
| 10. Tests and checks | Unit tests for each step; the real-window tests moved to finding things through folders; a new real-window test of Home, Pictures and narrowing | `make check` passes, and so does the real-window suite | Built |

## Step results

- **Home's memory:**
  - The orchestrator keeps `.berad/home.json` and its `.gitignore`.
  - It shares them as two session topics: what was opened, which only it
    sets, and what is kept, which the Project window sets.
  - Unit tests open a floor twice and find it noted once and saved. They
    find the pins and a folder's view kept across reopening the project,
    and a window's attempt to set what was opened ignored.
- **Folders on the left and narrowing,** built together, since the facets
  column could not stay beside the places:
  - Unit tests pick places, fold folders, narrow by a tick and by a search,
    and find the counts, Matches and the places kept.
  - They save, clear and apply a view, and drop a row onto a folder among
    the places.
- **Home:** unit tests find today's things newest first with where they
  live, earlier ones under their day, and a pin listed first. They show a
  thing in its folder and apply a view.
- **Rows:** a unit test finds a workflow's columns, opens a folder in place,
  sorts by a heading the folder keeps, and keeps folders first.
- **Pictures:** a unit test finds sprites as pictures, opens a folder's tile
  and asks for a sprite's tile and palettes. It keeps a size and a view for
  the folder.
- **Cards:** a unit test finds NPCs and zones as cards and asks for a zone's
  ground floor.
- **Sounds:** a unit test asks for the library's loudness once and a sound's
  waveform. It finds a sound never evened out says so, and plays one from
  its row.
- **Preview:** a unit test previews a zone with its floor tab and walks the
  strip with the arrow keys.
- **Tests and checks:**
  - A real-window test opens on Home and pins a workflow there.
  - It shows Sprites as pictures and in Preview.
  - It narrows to Matches, saves a view, clears it and applies it again.
  - It finds the view saved with the project, and the pin and Sprites' view
    in this person's state.
  - Increment 30's real-window test of the facets is replaced by it.
- **Checks:**
  - `make check` passes with nothing cached.
  - The real-window suite passes, 58 tests.
  - In the first full run, the call test (`TestClerkCallsFetchStock`)
    failed once. It waits up to forty seconds of game time for the clerk
    to reach Fetch stock, while the customer interrupts the clerk at
    random. On its own it failed 2 more times in 14 on this branch, and
    passed 8 times in 8 on the main branch. Frame times were the same on
    both. The second full run passed all 58.

## Not in this increment

- **Dragging a tile** in Pictures or Cards: rows drag, tiles do not yet.
- **Home's kind filters,** All, Workflows, NPCs and the others, as drawn;
  the bar's search finds across every kind.
- **Constants,** which come after this.
- **Sprite frames.** The editor has no animated sprites yet.
- **Recents shared between people,** and anything that knows who a person is.
