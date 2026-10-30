# Editor module

`github.com/bradhannah/berad-engine/editor`

The shared editor program used by every game: an orchestrator that is the only
writer of project data, and the editor window, one Ebitengine window with a tab
for each view open, in one process.
It uses the engine module; games never use this module.

| Package | Holds |
| --- | --- |
| `cmd/berad-editor` | The program: `open` runs the orchestrator and the editor window |
| `orchestrator` | Project data, edits, undo, journal, saving, panes' connections and crash reports |
| `protocol` | Messages, codec and framing |
| `client` | A pane's side of the protocol |
| `model` | Document and edit-kind seams, and the shared undo contract |
| `plugin` | The explicit plug-in registry |
| `plugins/project` | The project index and the create, rename and delete edits |
| `plugins/tilefloor` | Floors: paint either layer; resize on any side; add, change and remove anchors and NPC placements |
| `plugins/zone` | Zones and the edits that add, remove and reorder floors |
| `plugins/textdoc` | A placeholder text document for proving text editing and picks |
| `plugins/workflow` | Workflows: add, remove, change and move nodes; the layout kept in its own file |
| `plugins/npc` | NPCs and the edits that choose the behaviour and change the notes |
| `ui`, `ui/textedit` | Widgets in the wireframe look; the text editing core |
| `views/frame` | What every view shares: requests, status bar, inspector |
| `views/browser`, `views/mapview`, `views/textview`, `views/workflowview` | The project browser, Map, Text and Workflow, testable without a screen; the Map also runs previews |
| `views/picker` | Pick on map for any view that needs an anchor |
| `views/watchview` | Watch: where the watched NPC is in its workflow during a preview |
| `views/audioview` | Audio: adding sounds with their attribution; editing cues and recordings, their regions, levels, distances, profiles and families; the soundscape |
| `sound` | Plays the project's cues in a pane, from recordings the orchestrator sends, and Audio's soundscape at its monitor levels |
| `window` | The editor window: the window bar, tabs, splits and the pinned clock, and each pane's host: input and input method, requests, sound, the Open dialog, test scripts |
