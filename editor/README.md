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
| `plugins/tilefloor` | Floors: paint either layer; add, change and remove anchors |
| `plugins/zone` | Zones and the edits that add, remove and reorder floors |
| `plugins/textdoc` | A placeholder text document for proving text editing and picks |
| `ui`, `ui/textedit` | Widgets in the plain white look; the text editing core |
| `views/frame` | What every window shares: requests, status bar, inspector |
| `views/browser`, `views/mapview`, `views/textview` | The three windows, testable without a screen |
| `window` | Ebitengine host: input and input method, requests, test scripts |
