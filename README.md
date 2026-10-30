# berad-engine

A Go/Ebitengine engine and shared editor for top-down tile RPGs.

**Experimental and early.** The editor is one window with a tab for each view
open: a project browser, Map, Text, Workflow, Audio, a Pixel editor and a
Palette Editor, and a preview that walks a player through a zone. The
engine has floors, zones, stand-in tiles and walking. There are no game
features beyond that, and no stable API yet.

## Layout

The repository holds two independently versioned Go modules:

| Module | Path | Holds |
| --- | --- | --- |
| Engine | [`engine/`](engine/README.md) | What a game runs: the world model, object characteristics, the workflow runner, NPC schedules, the game loop and mode switching, and drawing the world |
| Editor | [`editor/`](editor/README.md) | One shared editor program: an orchestrator and the editor window, one Ebitengine window, in one process |

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

The editor window opens with the project browser's tab; double-click the
sample zone, which has two floors, to open it in a Map tab. Every view opens
as a tab of the same window: the window bar has a tab for each kind of view,
with a second row for the documents of the kind in front when there are
several, the clock pinned at its right end, and Watch opens beside its Map
([increment 13](docs/increments/0013-single-window.md)). To start a project
of your own, open a folder that has none: `.cache/bin/berad-editor open
--project DIR` makes one there, holding only what every project holds.
To try an increment in a fresh project, as the sample is made only in an
empty one: `make demo DEMO_PROJECT=.cache/demo-<name>`.

- **Project browser:** create zones, floors and text documents; rename in the
  inspector (Enter or the Rename button saves); delete with a where-used check;
  reorder floors; double-click or Enter opens.
- **Map:** tools on the left (`G` ground, `W` walls, `O` doors, stairs
  and pickups, `E` erase, `A` anchors, `S` select walls, `R` resize floor), `1`-`9`
  pick a tile or wall family. The zone origin, cell 0,0, is outlined, with axes
  along its edges; every floor of the zone shares it. Resize floor drags an edge
  or corner of the floor, or adds and removes rows and columns from buttons.
  With Select walls, click a wall for its run, double-click inside a room for
  its walls, or drag an area, then pick a family to change them all.
  `PageUp` / `PageDown` change floor; the floor below shows faintly (`F`).
  Scroll pans, `Cmd`+scroll or `Cmd+=` / `Cmd+-` zoom, `Space`+drag pans.
- **Preview:** in a Map press `P` (or the Preview button) and click a
  cell to start the player there. The arrow keys walk, one cell per tap, and
  two together walk diagonally; walking onto stairs changes floor. You can keep
  editing while you walk, other views of the zone show the player, and `P`
  stops. A preview never changes the project.
- **Text:** Enter saves, `Cmd+Enter` or `Ctrl+J` starts a new line,
  Escape reverts; "Pick on map" chooses an anchor in a Map.
- **Workflow:** New workflow in the project browser, then Open. Click a
  palette entry to add a node after the selected one, or drag it onto the
  canvas; drag from an output's port to a node to connect them; drag nodes to
  move them; Backspace removes the selected node. The inspector edits the
  selected node: Enter saves a field, and "Pick on map" chooses a Go to's
  destination. Auto-arrange lays the workflow out.
- **Audio:** the project browser's Audio section lists the cues; open
  one, or click the section's [+] to add a sound. Adding a sound asks for a
  file, copies it into the project and asks who made it and whether it may be
  used in a commercial game. Audio lists cues by category, or
  recordings, with filters for problems and for sounds not cleared; it plays
  them, and edits a cue's name, category, gain and the recordings it plays in
  turn. A recording's waveform shows where its sound is: trim it to a region,
  make a new recording from a region, and even out its level with the gain
  berad-audio measures. An ambient cue is heard from a distance, drawn on the
  curve the game uses. The soundscape plays cues together at monitor levels,
  and is never saved.

`Cmd+Z` / `Cmd+Shift+Z` undo and redo (a whole stroke is one step); `Cmd+N`
opens another view of the same zone, and the views update live and share undo;
`Cmd+W` closes a tab. `Cmd+Q`, or closing the project browser's tab or the
window, quits the editor after asking. Every change is saved automatically;
the files are in the project folder.

`Cmd+Shift+K` crashes the current tab on purpose, to show the failure
handling: that tab closes, the others show a notice, and a crash report
appears in the project's `.berad/logs`.

## How it fits together

- The **orchestrator** (`editor/orchestrator`) is the only writer of project
  data. One goroutine owns all state. It runs in the editor's process, and
  each pane connects to it over an in-memory pipe of its own.
- The **editor window** (`editor/window`, behaviour in `editor/views`) shows
  each view in a pane of a tab. Panes send commands and change only when the
  orchestrator's event arrives (strict updates).
- The **protocol** (`editor/protocol`) defines every message once as Go types,
  JSON-encoded behind a codec interface, with revision numbers so a pane that
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
  use berad-text for layout; the editor window runs the input method so
  accents and composed characters work.

## Checks

Both modules follow berad-sdk's strict Go policy: the same lint configuration
(`.golangci.yml`) and pinned tools (golangci-lint 2.15.0, goimports from
golang.org/x/tools v0.37.0), with zero findings and no baseline.

```sh
make tools     # install the pinned tools into .cache/tools (needs network)
make check     # formatting, lint, go vet and race tests for both modules
make format    # apply formatting
make gui-test  # tests that open the real editor window, driven by input scripts
make latency   # strict-mode command-to-event time over a pane's connection
```

Real-window tests need a desktop session, so they run only with
`make gui-test` (`BERAD_GUI_TESTS=1`); `BERAD_SHOT_DIR` keeps their
screenshots. While input scripts drive it, the editor window opens without
taking focus. `BERAD_SCREENSHOT=path` makes each pane save a screenshot of
the window once, for visual checks.

## Dependencies

The editor uses Ebitengine v2.10.5 and berad-sdk's berad-text and berad-audio:
berad-audio's authoring package reads a project's audio files and its runtime
plays them. Its widgets are
its own (`editor/ui`): ebitenui was not used, because its text input is
single-line and has no input method support. The two modules are joined by
`go.work` for development.

## Documentation

Design decisions, vocabulary and each increment of work are in [docs](docs/README.md).

## Contributing

Read [AGENTS.md](AGENTS.md) before working in this repository.

## License

MIT. See [LICENSE](LICENSE).
