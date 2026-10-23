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
| `plugins/tilefloor` | The tile floor document and paint edit |
| `views/mapview` | Map window behaviour, testable without a screen |
| `window` | Ebitengine windows: input, drawing, test scripts |
