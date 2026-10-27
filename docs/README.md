# Documentation

| Document | Holds |
| --- | --- |
| [Architecture](ARCHITECTURE.md) | The editor's numbered design decisions (D1-D17) |
| [Editor features](EDITOR_FEATURES.md) | What the editor lets an author do, with agreed and candidate directions |
| [Vocabulary](VOCABULARY.md) | One term per concept: spatial, art and editor terms |
| [First-pass review](FIRST_PASS_REVIEW.md) | The architecture checked against known problems, with sources |
| [Research notes](RESEARCH_NOTES.md) | Background research and unadopted proposals |

## Increments

Work on the engine and editor comes in increments. Each has its own record in
`increments/`, named by its number and topic and started from [the
template](templates/increment.md): what it proves, its decisions, its steps
and their results, and what changed after its author tried it.

An increment is **Planned** until its steps are built, **Built** once they
are and the checks pass, and **Judged** once its author has tried it and
accepted it.

| Increment | Holds | Status |
| --- | --- | --- |
| [1. Foundation](increments/0001-foundation.md) | The first implementation's steps and results | Built |
| [2. Walk the map](increments/0002-walk-the-map.md) | Walking the map: the preview's decisions, steps and results | Built |
| [3. NPC workflows](increments/0003-npc-workflows.md) | NPC workflows and the wireframe look: decisions, steps and results | Built |
| [4. Sounds in workflows](increments/0004-sounds-in-workflows.md) | Sounds in workflows: choosing and hearing cues, decisions, steps and results | Built |
| [5. The Audio window](increments/0005-audio-window.md) | The Audio window: adding sounds with their attribution and editing cues | Built |
| [6. Trimming and levels](increments/0006-trimming-and-levels.md) | Trimming and levels: a recording's waveform, region and measured gain | Built |
| [7. Ambient sounds and the soundscape](increments/0007-ambient-and-soundscape.md) | Ambient sounds and the soundscape: distance, profiles and families, monitor levels | Built |
| [8. Loudness](increments/0008-loudness.md) | Loudness: recordings evened out to a reference, how a cue is meant to sound | Judged |
| [9. Files and their pieces](increments/0009-files-and-pieces.md) | Files and their pieces: a file cut into recordings, its attribution, finding its sounds | Judged |
| [10. One window](increments/0010-one-window.md) | One window (spike): the views as tabs, a pinned clock and side-by-side panes | Built |
| [11. Drawing tiles](increments/0011-drawing-tiles.md) | The first pixel editor: the tile size, the shared palette's named ramps, drawing a tile and seeing its sprite on the map | Planned |
