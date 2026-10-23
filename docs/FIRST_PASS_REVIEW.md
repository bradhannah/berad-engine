# First-pass design review

A review of the editor architecture against common, documented problems with
this kind of design. The agreed outcomes are decision D17 in
[the architecture](ARCHITECTURE.md); the rest is analysis.

## Problems checked

| Known problem | Where the design stands |
| --- | --- |
| Building an engine instead of a game | Need-driven scope (D17): nothing goes in unless a consuming game needs it now. |
| No format versions in saved data | Every saved file and journal entry is versioned (D17). |
| Multi-process state drifting, locks, deadlocks, bugs that will not reproduce | One writer, strict updates, no shared locks, revision numbers, recorded traffic. The remaining risk is speed, measured against a one-frame budget. |
| Undo with several editors is confusing | Per-window undo, the recognised "undo your own changes" approach; undos that would overwrite another window's later change are refused. A useful test: undo a lot, copy, redo back, and nothing should change. |
| Snapshot-based undo is slow; changes depend on each other | Each edit stores what changed and how to undo it (D14). |
| Node graphs diff and merge badly as text; spaghetti | Notes, containers, collapsing and a minimap; layout stored apart from logic (D17). |
| Editor code mutating a running game | Separate engine and editor modules; preview changes only through the orchestrator. The engine needs a way to accept updated content while running, for preview. |
| Plug-in registration through import side effects | An explicit plug-in list (D17). |

## SOLID

The design supports SOLID. The two risks, an orchestrator growing into one large
object and plug-in contracts going unenforced, are addressed by D17. Apply it at
the seams, not everywhere.

## berad-sdk, berad-engine, or the game?

- Would any Ebitengine game use it unchanged? **berad-sdk.**
- Is it about top-down tile-RPG worlds, workflows or authoring? **berad-engine.**
- Is it one game's content, settings or unique modes? **That game's repository.**

berad-sdk's own rules agree: no game semantics or world rules, and no tile
abstraction without a concrete need.

**Likely berad-sdk additions:** pathfinding (A*, with caller-supplied neighbours
and costs, extracted from Ultima V Redux); mouse support in berad-input; palette
and indexed-colour operations, pixel-art shape drawing and flood fill in
berad-graphics; multi-line text editing in berad-text if ebitenui's text box is
not good enough.

**Stays in berad-engine:** the world model, wall-family auto-connection, object
characteristics, the workflow runner and node types, schedules, conversations,
the game loop and mode switching, live content reload, and the editor. The
editor's message transport stays in the editor until something else needs it.

## Later gaps

Workflow timing and determinism (seeded berad-rng streams so replay and tests
repeat exactly), game save format, importing art and palettes, commands and
keyboard shortcuts as data, and macOS packaging.

## Sources

- [Game Developer: writing a game engine from scratch](https://www.gamedeveloper.com/programming/writing-a-game-engine-from-scratch-part-1-messaging)
- [Figma: multiplayer editing](https://www.figma.com/blog/multiplayer-editing-in-figma/)
- [Liveblocks: multiplayer undo and redo](https://development.liveblocks.io/blog/how-to-build-undo-redo-in-a-multiplayer-environment.md)
- [Epic Games: diffing Blueprints](https://www.unrealengine.com/en-US/blog/diffing-blueprints)
- [NodeGit: diffing and merging node graphs](https://history.siggraph.org/?p=214587)
- [Godot: running code in the editor](https://docs.godotengine.org/en/4.4/tutorials/plugins/running_code_in_the_editor.html)
- [Bugnet: save corruption across versions](https://bugnet.io/blog/debugging-save-corruption-across-versions)
- [Arawn: save data versioning](https://arawn-software-publishing.gitbook.io/arawn/basics/save-data-versioning)
- [staticplug](https://beta.pkg.go.dev/github.com/hansmi/staticplug)
- [containerd plugin registration](https://pkg.go.dev/github.com/containerd/plugin)
