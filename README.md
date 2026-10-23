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

## Checks

Both modules follow berad-sdk's strict Go policy: the same lint configuration
(`.golangci.yml`) and pinned tools (golangci-lint 2.12.2, goimports from
golang.org/x/tools v0.37.0), with zero findings and no baseline.

```sh
make tools   # install the pinned tools into .cache/tools (needs network)
make check   # formatting, lint, go vet and race tests for both modules
make format  # apply formatting
```

## Dependencies

Ebitengine v2.10.5 and ebitenui v0.7.3 are the chosen versions. They are added
to a module when its first code uses them.

## Contributing

Read [AGENTS.md](AGENTS.md) before working in this repository.

## License

MIT. See [LICENSE](LICENSE).
