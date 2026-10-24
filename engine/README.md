# Engine module

`github.com/bradhannah/berad-engine/engine`

What a top-down tile RPG runs. This module must never import the editor module.

| Package | Holds |
| --- | --- |
| `ids` | ULIDs: permanent, sortable identifiers |
| `content` | The versioned envelope for saved documents and step-by-step upgrades |
| `tilefloor` | A tile floor: ground and top layers, anchors, NPC placements and bounds around the zone origin |
| `zone` | A zone: its floors from the bottom up |
| `tiles` | Built-in stand-in tiles, passability, stairs and the wall neighbour mask |
| `play` | A run of a zone: the player walking, one cell per step, and taking stairs |
| `workflow` | Workflows: typed node kinds with declared parameters and outputs, and the layout kept apart |
| `npc` | An NPC: its behaviour workflow and its author's private notes |
| `clock` | The game clock: a time of day, a rate, stop and jump |
| `npcrun` | NPCs running their workflows, a tick at a time; Go to finds its way with berad-path |

The engine depends on one berad-sdk module, `modules/berad-path`, pinned at a
reviewed revision. The world model, characteristics, schedules and game loop
arrive as games need them.
