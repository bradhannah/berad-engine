# Engine module

`github.com/bradhannah/berad-engine/engine`

What a top-down tile RPG runs. This module must never import the editor module.

| Package | Holds |
| --- | --- |
| `ids` | ULIDs: permanent, sortable identifiers |
| `content` | The versioned envelope for saved documents and step-by-step upgrades |
| `tilefloor` | A tile floor: ground and top layers, anchors, NPC placements, ambient sounds, portals and bounds around the zone origin |
| `zone` | A zone: its floors from the bottom up |
| `tiles` | Built-in stand-in tiles, passability, stairs and the wall neighbour mask |
| `play` | A run of a zone: the player walking, one cell per step, and taking stairs |
| `workflow` | Workflows: typed node kinds with declared parameters and outputs, and the layout kept apart |
| `npc` | An NPC: its behaviour workflow and its author's private notes |
| `clock` | The game clock: a time of day, a rate, stop and jump |
| `npcrun` | NPCs running their workflows, a tick at a time; Go to finds its way with berad-path |
| `game` | The game runtime: a project being played, a tick at a time, with nothing drawn; it enters another zone when told, and holds the player on a portal until it is |
| `gameload` | What a game plays, gathered from a project's documents: from an editor window's, or from a project folder; and where a portal leads |
| `player` | A game shown and played: draws the game runtime's screen, writes what it shows, plays its sounds, and runs it in a window of its own |
| `cmd/berad-play` | The player as a program: `berad-play --project DIR` |

The engine depends on two berad-sdk modules, `modules/berad-path` and
`modules/berad-audio`, each pinned at a reviewed revision. Only package
`player` and the programs under `cmd` use Ebitengine and berad-audio's
Ebitengine backend; everything else is headless, so a run repeats exactly,
and `boundary_test.go` fails when that changes. The world model,
characteristics, schedules and a game's own modes arrive as games need them.
