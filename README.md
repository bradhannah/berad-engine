# berad-engine

A Go/Ebitengine engine and shared editor for top-down tile RPGs.

**Experimental and early.** The editor has an orchestrator, a project browser,
Map and Text windows, and a preview that walks a player through a zone. The
engine has floors, zones, stand-in tiles and walking. There are no game
features beyond that, and no stable API yet.

## Layout

The repository holds two independently versioned Go modules:

| Module | Path | Holds |
| --- | --- | --- |
| Engine | [`engine/`](engine/README.md) | What a game runs: the world model, object characteristics, the workflow runner, NPC schedules, the game loop and mode switching, and drawing the world |
| Editor | [`editor/`](editor/README.md) | One shared editor program: a headless orchestrator plus one Ebitengine process per editor window |

Games depend on the engine module and on [berad-sdk](https://github.com/bradhannah/berad-sdk),
never on the editor module. The engine module never imports the editor module.
Pieces that any Ebitengine game could use unchanged belong in berad-sdk instead.

## Scope

berad-engine targets top-down tile RPGs, not every genre. Functionality is added
when a consuming game needs it, not in anticipation of hypothetical needs.
Each game keeps its own content, settings and unique game modes (such as intros)
in its own repository.

## Try it

```sh
make demo     # build the editor and open a sample project in .cache/demo-project
```

The project browser and a Map window open on a sample zone with two floors.

- **Project browser:** create zones, floors and text documents; rename in the
  inspector (Enter or the Rename button saves); delete with a where-used check;
  reorder floors; double-click or Enter opens.
- **Map window:** tools on the left (`G` ground, `W` walls, `O` doors, stairs
  and pickups, `E` erase, `A` anchors, `S` select walls, `R` resize floor), `1`-`9`
  pick a tile or wall family. The zone origin, cell 0,0, is outlined, with axes
  along its edges; every floor of the zone shares it. Resize floor drags an edge
  or corner of the floor, or adds and removes rows and columns from buttons.
  With Select walls, click a wall for its run, double-click inside a room for
  its walls, or drag an area, then pick a family to change them all.
  `PageUp` / `PageDown` change floor; the floor below shows faintly (`F`).
  Scroll pans, `Cmd`+scroll or `Cmd+=` / `Cmd+-` zoom, `Space`+drag pans.
- **Preview:** in a Map window press `P` (or the Preview button) and click a
  cell to start the player there. The arrow keys walk, one cell per tap, and
  two together walk diagonally; walking onto stairs changes floor. You can keep
  editing while you walk, other views of the zone show the player, and `P`
  stops. A preview never changes the project.
- **Text window:** Enter saves, `Cmd+Enter` or `Ctrl+J` starts a new line,
  Escape reverts; "Pick on map" chooses an anchor in a Map window.
- **Workflow window:** New workflow in the project browser, then Open. Click a
  palette entry to add a node after the selected one, or drag it onto the
  canvas; drag from an output's port to a node to connect them; drag nodes to
  move them; Backspace removes the selected node. The inspector edits the
  selected node: Enter saves a field, and "Pick on map" chooses a Go to's
  destination. Auto-arrange lays the workflow out.

`Cmd+Z` / `Cmd+Shift+Z` undo and redo (a whole stroke is one step); `Cmd+N`
opens another view of the same zone, and the views update live and share undo;
`Cmd+W` closes a window. The editor quits when its last window closes. Every
change is saved automatically; the files are in the project folder.

`Cmd+Shift+K` crashes the current window on purpose, to show the failure
handling: the other windows show a notice and a crash report appears in the
project's `.berad/logs`.

## How it fits together

- The **orchestrator** (`editor/orchestrator`) is the only writer of project
  data. One goroutine owns all state. Windows connect over a Unix socket.
- **Windows** (`editor/window`, behaviour in `editor/views`) are separate
  processes of the same program. They send commands and change only when the
  orchestrator's event arrives (strict updates).
- The **protocol** (`editor/protocol`) defines every message once as Go types,
  JSON-encoded behind a codec interface, with revision numbers so a window that
  misses an event reloads instead of drifting.
- **Edits** come from plug-ins registered in one explicit list
  (`editor/plugin`); each edit kind knows how to undo itself and must pass the
  shared apply-then-undo test (`editor/model`).
- Every saved file is a **versioned envelope** (`engine/content`), upgraded step
  by step on load after a backup. Edits are journaled before they are
  acknowledged and folded into the files, written then renamed.
- The **project index** (`editor/plugins/project`) lists every document.
  Creating, renaming and deleting documents are ordinary edits on it, so they
  are journaled and undoable like any other change.
- **Views** (`editor/views`) share a frame (`editor/views/frame`) and a small
  widget kit (`editor/ui`). Text fields keep a draft (`editor/ui/textedit`) and
  use berad-text for layout; the window runs the input method so accents and
  composed characters work.

## Checks

Both modules follow berad-sdk's strict Go policy: the same lint configuration
(`.golangci.yml`) and pinned tools (golangci-lint 2.15.0, goimports from
golang.org/x/tools v0.37.0), with zero findings and no baseline.

```sh
make tools     # install the pinned tools into .cache/tools (needs network)
make check     # formatting, lint, go vet and race tests for both modules
make format    # apply formatting
make gui-test  # tests that open real editor windows, driven by input scripts
make latency   # strict-mode command-to-event time over a real socket
```

Window tests need a desktop session, so they run only with `make gui-test`
(`BERAD_GUI_TESTS=1`). Windows driven by input scripts open without taking
focus. `BERAD_SCREENSHOT=path` makes a window save its pixels
once, for visual checks.

## Dependencies

The editor uses Ebitengine v2.10.5 and berad-sdk's berad-text. Its widgets are
its own (`editor/ui`): ebitenui was not used, because its text input is
single-line and has no input method support. The two modules are joined by
`go.work` for development.

## Documentation

Design decisions, vocabulary and build progress are in [docs](docs/README.md).

## Contributing

Read [AGENTS.md](AGENTS.md) before working in this repository.

## License

MIT. See [LICENSE](LICENSE).
