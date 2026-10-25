# Editor module

`github.com/bradhannah/berad-engine/editor`

The shared editor program used by every game: a headless orchestrator that is
the only writer of project data, plus one Ebitengine process per editor window.
It uses the engine module; games never use this module.

| Package | Holds |
| --- | --- |
| `cmd/berad-editor` | The program: `open` runs the orchestrator, `window` runs one window |
| `orchestrator` | Project data, edits, undo, journal, saving, window processes |
| `protocol` | Messages, codec and framing |
| `client` | The window side of the protocol |
| `model` | Document and edit-kind seams, and the shared undo contract |
| `plugin` | The explicit plug-in registry |
| `plugins/project` | The project index and the create, rename and delete edits |
| `plugins/tilefloor` | Floors: paint either layer; resize on any side; add, change and remove anchors and NPC placements |
| `plugins/zone` | Zones and the edits that add, remove and reorder floors |
| `plugins/textdoc` | A placeholder text document for proving text editing and picks |
| `plugins/workflow` | Workflows: add, remove, change and move nodes; the layout kept in its own file |
| `plugins/npc` | NPCs and the edits that choose the behaviour and change the notes |
| `ui`, `ui/textedit` | Widgets in the wireframe look; the text editing core |
| `views/frame` | What every window shares: requests, status bar, inspector |
| `views/browser`, `views/mapview`, `views/textview`, `views/workflowview` | The four windows, testable without a screen; the Map window also runs previews |
| `views/picker` | Pick on map for any window that needs an anchor |
| `views/watchview` | The Watch window: where the watched NPC is in its workflow during a preview |
| `views/audioview` | The Audio window: adding sounds with their attribution, and editing cues and recordings |
| `sound` | Plays the project's cues in a window, from recordings the orchestrator sends |
| `window` | Ebitengine host: input and input method, requests, sound, the Open dialog, test scripts |
