# Engine module

`github.com/bradhannah/berad-engine/engine`

What a top-down tile RPG runs. This module must never import the editor module.

| Package | Holds |
| --- | --- |
| `ids` | ULIDs: permanent, sortable identifiers |
| `content` | The versioned envelope for saved documents and step-by-step upgrades |
| `tilefloor` | A tile floor: ground and top layers, anchors and bounds around the zone origin |
| `zone` | A zone: its floors from the bottom up |
| `tiles` | Built-in stand-in tiles, passability, stairs and the wall neighbour mask |
| `play` | A run of a zone: the player walking, one cell per step, and taking stairs |

The world model, characteristics, workflow runner, schedules and game loop
arrive as games need them.
