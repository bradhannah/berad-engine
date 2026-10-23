# Engine module

`github.com/bradhannah/berad-engine/engine`

What a top-down tile RPG runs. This module must never import the editor module.

| Package | Holds |
| --- | --- |
| `ids` | ULIDs: permanent, sortable identifiers |
| `content` | The versioned envelope for saved documents and step-by-step upgrades |
| `tilefloor` | A minimal tile floor, the first document type |

The world model, characteristics, workflow runner, schedules and game loop
arrive as games need them.
