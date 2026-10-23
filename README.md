# berad-engine

A Go/Ebitengine engine and shared editor for top-down tile RPGs.

**Experimental, early design stage. There is no implementation yet:** the module
folders exist, but they contain no code, examples or checks.

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

## Documentation

Design decisions, vocabulary and build progress are in [docs](docs/README.md).

## Contributing

Read [AGENTS.md](AGENTS.md) before working in this repository.

## License

MIT. See [LICENSE](LICENSE).
