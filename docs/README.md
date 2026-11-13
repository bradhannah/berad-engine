# Documentation

| Document | Holds |
| --- | --- |
| [Architecture](ARCHITECTURE.md) | The editor's numbered design decisions (D1-D17) |
| [Editor features](EDITOR_FEATURES.md) | What the editor lets an author do, with agreed and candidate directions |
| [Vocabulary](VOCABULARY.md) | One term per concept: spatial, art and editor terms |
| [First-pass review](FIRST_PASS_REVIEW.md) | The architecture checked against known problems, with sources |
| [Research notes](RESEARCH_NOTES.md) | Background research and unadopted proposals |
| [Workflow design](WORKFLOW_DESIGN.md) | Where workflows are heading: containers and Together, kinds of workflow, values, calls, speech, signals and conversations (draft) |

## Increments

Work on the engine and editor comes in increments. Each has its own record in
`increments/`, named by its number and topic and started from [the
template](templates/increment.md): what it proves, its decisions, its steps
and their results, and what changed after its author tried it.

An increment is **Planned** until its steps are built, **Built** once they
are and the checks pass, and **Judged** once its author has tried it and
accepted it.

| Increment | Holds | Status |
| --- | --- | --- |
| [1. Foundation](increments/0001-foundation.md) | The first implementation's steps and results | Built |
| [2. Walk the map](increments/0002-walk-the-map.md) | Walking the map: the preview's decisions, steps and results | Built |
| [3. NPC workflows](increments/0003-npc-workflows.md) | NPC workflows and the wireframe look: decisions, steps and results | Built |
| [4. Sounds in workflows](increments/0004-sounds-in-workflows.md) | Sounds in workflows: choosing and hearing cues, decisions, steps and results | Built |
| [5. The Audio window](increments/0005-audio-window.md) | The Audio window: adding sounds with their attribution and editing cues | Built |
| [6. Trimming and levels](increments/0006-trimming-and-levels.md) | Trimming and levels: a recording's waveform, region and measured gain | Built |
| [7. Ambient sounds and the soundscape](increments/0007-ambient-and-soundscape.md) | Ambient sounds and the soundscape: distance, profiles and families, monitor levels | Built |
| [8. Loudness](increments/0008-loudness.md) | Loudness: recordings evened out to a reference, how a cue is meant to sound | Judged |
| [9. Files and their pieces](increments/0009-files-and-pieces.md) | Files and their pieces: a file cut into recordings, its attribution, finding its sounds | Judged |
| [10. One window](increments/0010-one-window.md) | One window (spike): the views as tabs, a pinned clock and side-by-side panes | Built |
| [11. Drawing tiles](increments/0011-drawing-tiles.md) | The first pixel editor: the tile size, the shared palette's named ramps, drawing a tile and seeing its sprite on the map | Judged |
| [12. Palettes](increments/0012-palettes.md) | Palettes: Default built in, palettes of a game's own, the Palette Editor and a pick-only Pixel editor | Built |
| [13. Single window](increments/0013-single-window.md) | Single window: the editor as one window in one process, its views as tabs, the multi-window option removed | Built |
| [14. Launch](increments/0014-launch.md) | Launch: playing the project in a Play tab with sprites and sounds, the player start, ambient and zone sounds, one wall sprite | Built |
| [15. Values and sensing](increments/0015-values-and-sensing.md) | Values and sensing: declared values in four scopes, Set and Check value, and finding an NPC nearby | Built |
| [16. Templates](increments/0016-templates.md) | Templates: discouraged and not-allowed pixels and guides to draw to, chosen per sprite and edited in a Template editor | Built |
| [17. Reading the graph](increments/0017-reading-the-graph.md) | Reading the graph: a handle on every leg of a line, the status bar naming a line and a legend, colours on nodes, containers, outcomes and lines, the outcome's name at a long line's arrowhead | Built |
| [18. Comparisons by type](increments/0018-comparisons.md) | Comparisons by type: symbols for integers against a number or another value, true or false for a bool, any of or none of a list for a choice, is or is not for text | Built |
| [19. The value picker](increments/0019-value-picker.md) | The value picker: a dialog over the window with search, type chips, a scope rail, folding groups and declaring, opened by every chooser | Built |
| [20. Moving and finding values](increments/0020-moving-and-finding.md) | Moving and finding values: a right-click and dragging move a value into a group, asking first when it is used; Find a value opens the picker over World state to show or declare one; text on a node in curly quotes | Built |
| [21. Flow modules](increments/0021-flow-modules.md) | Flow modules: a Call workflow node runs another workflow with inputs, a place among them, and carries on by the Return it ends at; value outputs kept by the caller; Watch shows the path through the calls; a place chosen by name or picked on a map in a picking mode, marked by the anchor's ring | Built |
| [22. NPCs live in one zone](increments/0022-npc-zones.md) | NPCs live in one zone: an NPC's zone set in its details and chosen with a zone picker over the window; placed only in its zone; changing it asks first; places it can't reach flagged; NPCs take the stairs between floors | Built |
| [23. Kinds of workflow, and where they run](increments/0023-workflow-kinds.md) | Kinds of workflow, and where they run: NPC behaviours and zone loops, each with its own nodes; a zone's loop set in its details and run while the zone is alive; a workflow running in one zone or any, its places, followers and loops checked against it; choosing offers what fits; changing it asks first | Built |
| [24. Built-in values and the Log node](increments/0024-built-ins-and-log.md) | Built-in values and the Log node: an NPC's zone, behaviour and sprite as values; zone, workflow and sprite compared by identity; a workflow setting its own NPC's sprite; every value as text; a Log node writing lines to Watch | Built |
| [25. Finding and placing nodes](increments/0025-palette.md) | Finding and placing nodes: the palette's search by what a node does, its one line on hover, its help page in the inspector on a click with Add after, and only dragging to place | Built |
| [26. Speech, message boxes and screen shake](increments/0026-speech-and-boxes.md) | Speech, message boxes and screen shake: Say over the NPC's own head, Show message in a box that goes on its own or is closed, each in a queue and waited for or not; Shake screen moving the world | Built |
| [27. Together](increments/0027-together.md) | Together: a container of lanes that start at once and go on when every lane is done; one lane moves the NPC; Play sound waiting until it has played; every lane in Watch | Built |
| [28. Signals](increments/0028-signals.md) | Signals: declared signals sent to an NPC or everyone in its zone; receivers that interrupt at a safe moment, then Resume or take over; broken signal calls flagged and unheard receivers warned | Built |
| [29. Lines by hand, World state's parts and picking signals](increments/0029-lines-state-and-picking.md) | Lines by hand: drops land unconnected and back lines dip only for what is in their way; World state a section with Values and Signals windows, lists of readable width with the inspector beside them; a folding inspector; a signal picker | Built |
| [30. Lines over nodes, and the Project by facets](increments/0030-project-facets.md) | Lines over nodes: a line shows through a node it crosses, and a node nothing links to says so; the Project window's facets with counts, a table once narrowed, and views saved with the project; every document's last change and every workflow's problems known unopened | Built |
| [31. Text documents removed](increments/0031-no-text-documents.md) | Text documents removed: the placeholder goes, a project's old ones are skipped and kept on disk, and the tests that used them use real documents; constants will hold a game's text | Built |
| [32. The Project by folders, with Home](increments/0032-project-by-folders.md) | The Project by folders: Home with recent, pinned and troubled things; folders on the left and a folder's contents beside them as pictures, rows, cards, sounds or a preview; the facets as a bar that narrows the folders | Built |
| [33. Constants](increments/0033-constants.md) | Constants: text and whole numbers the designer sets once and a game never changes, in the world, templates and each NPC's and zone's own, each holder with its own value; written in a sheet with markup for colour, bold and italic; used in messages, checks, operands and number fields, with `const.` and `var.` where they mix; Make a constant finds the other places typed the same | Built |
| [34. Multi-select](increments/0034-multi-select.md) | Multi-select: several nodes and containers selected with Cmd+click or a box, then moved, nudged, coloured, deleted, duplicated, arranged and copied into another workflow together, each one undo step; a copy keeps the lines between its nodes and a paste declares the locals it needs | Built |
| [35. Finding by words](increments/0035-finding-by-words.md) | Finding by words: the Project window's search lists each constant whose path or text says the words, and each line typed into a Say, Show message or Log that does; opening one selects it in the Constants window or its workflow | Built |
| [36. Bugs from the review](increments/0036-review-bugs.md) | Bugs from the review: twenty-one fixed, each behind a test that failed first. The six that lose work: a folder renamed only in its capitals, a refused undo, the quit dialog with the inspector folded, a cleared folder name, the crash key, dragging in a field. Then fifteen more: an unsaved local value, a failed save and an unreadable file told to the author, limits in characters, a time switch's times, five in the Map, the folder shown, a pasted Return, a guide's name, a constant's mark and Watch's values | Built |
| [37. A desk and a stage](increments/0037-desk-and-stage.md) | A desk and a stage: four tones with one job each, white only where the author types or chooses, the game on a dark stage, one job for each colour, waiting for Save in ink, and every document tab in its tint | Built |
| [38. One row of tabs](increments/0038-one-row-of-tabs.md) | One row of tabs: every document a tab; keys to choose a tab by position, the one last used and its neighbours; Back and Forward through where the author has been; quick open from any tab | Built |
| [39. A history that can be read](increments/0039-readable-history.md) | A history that can be read: every change a sentence, Undo and Redo saying which they took back, refusals that stay until dismissed, History in each tab's status bar, and the Map naming the tool in hand | Built |
| [40. Lines do only what their author did](increments/0040-lines-do-what-was-done.md) | Lines do only what their author did: a line dropped on empty canvas offers a node instead of disconnecting, Tab and a double-click offer the same, a line about to be replaced shows red and dashed first, Unlink is named, and Add after says which output it joins from | Built |
| [41. Starting a preview opens nothing](increments/0041-preview-opens-nothing.md) | Starting a preview opens nothing: the clock's panel opens only from its readout, its hour steps and Stop sit on the window bar, and Watch's Follow switch is the author's to turn on, turned off by panning by hand | Built |
| [42. A Map that says what it shows](increments/0042-map-says-what-it-shows.md) | A Map that says what it shows: the stage plain beyond the floor unless it is being resized, the zone origin marked without orange lines across the view, and a tag saying which way the floor is when it is out of view | Built |
| [43. Panels fold, when told to](increments/0043-panels-fold.md) | Panels fold, when told to: the inspector and a Map's or a Workflow's left panel fold by button or key, Cmd+I and Cmd+Shift+I, for every view of the kind; a Map's panel folds to a rail of its tools, a Workflow's palette to a named strip | Built |
| [44. A Workflow names what is in hand](increments/0044-workflow-names-what-is-in-hand.md) | A Workflow names what is in hand: the workflow, what is selected and what the pointer is over in the status bar, or that Tab adds a node on empty canvas | Built |
| [45. A line in History selects what it names](increments/0045-history-lines-select.md) | A line in History selects what it names: a change says what it touched, a line says what a click selects, and a click selects that node, NPC placement or anchor | Built |
| [46. Watch keeps a trail](increments/0046-watch-keeps-a-trail.md) | Watch keeps a trail: each node the watched NPC passed, with what the node read or did that sent it on, and restarts with what they cleared; the Log, the Trail and the Values share one strip | Built |
| [47. Play from here](increments/0047-play-from-here.md) | Play from here: a cell's menu plays the game from that cell for one run, beside Preview from here, and the player start stays where it is | Built |
| [48. The inspector writes out a node's links](increments/0048-inspector-writes-links.md) | The inspector writes out a node's links: what joins to the selected node and where each output goes, with Unlink beside each line | Built |
| [49. The Pixel editor names what is in hand](increments/0049-pixel-editor-names-what-is-in-hand.md) | The Pixel editor names what is in hand: the sprite, the tool, its colour and the pixel under the pointer, in the status bar | Built |
| [50. A Say wraps its line on the node](increments/0050-say-wraps-on-its-node.md) | A Say wraps its line on the node: a Say or a Show message grows a line for each line its message takes, up to four, and its field has room in the inspector | Built |
| [51. Views say their changes and refusals](increments/0051-views-say-their-changes.md) | Views say their changes and refusals: a Map, a Workflow and the Pixel editor say each change in their own words, a stroke as one line, and the refusals they make stay until dismissed | Built |
| [52. A split has one status bar](increments/0052-split-has-one-status-bar.md) | A split has one status bar: the focused pane's, across the whole tab, under both panes | Built |
| [53. Dividers drag, and widths and folds are kept](increments/0053-dividers-drag-and-are-kept.md) | Dividers drag, and widths and folds are kept: the inspector's and the left panel's dividers drag for every view of a kind, a double-click puts one back, the canvas keeps its room, and the arrangement is kept per person for each size of screen | Built |
| [54. Arrangements by screen and by mode](increments/0054-arrangements-by-screen-and-mode.md) | Arrangements by screen and by mode: the screen chooses a kind's starting widths once and says so, a Map keeps an arrangement for editing and one for previewing, and the window stops narrowing rather than squeeze a panel | Built |
| [55. The window reopens as it was left](increments/0055-reopens-as-left.md) | The window reopens as it was left: its frame, its tabs in order, what sat beside what and the tab in front, saying what could not be found | Built |
| [56. Native resolution and a scale for text and controls](increments/0056-native-resolution-and-ui-scale.md) | Native resolution and a scale for text and controls: drawing at the screen's own resolution, with a scale per person that changes nothing else, built a view at a time | Planned |
| [57. The files are shared](increments/0057-files-are-shared.md) | The files are shared: a file changed outside the editor is taken in or asked about, never written over; a project that cannot open as it is says which file and why; what the editor does not know is said and backed up; notices wait their turn | Built |
| [58. Nothing used goes unseen](increments/0058-nothing-used-goes-unseen.md) | Nothing used goes unseen: a zone and its floors go together, another tab's unsaved changes are named, a shrinking floor lists all that goes and keeps the player start, and a local value in use is refused | Built |
| [59. Keys in a field do only what the field shows](increments/0059-keys-in-a-field.md) | Keys in a field do only what the field shows: Esc keeps what was typed, Cmd+Z stops at the field, Tab moves between fields, and the clock takes keys only once clicked | Planned |
| [60. Braces, a first colour, and a crashed tab](increments/0060-braces-colour-and-crashes.md) | Braces, a first colour, and a crashed tab: `{{` writes a brace, a sprite opens with the colour last used in it, and a crashed tab reopens when told | Planned |
