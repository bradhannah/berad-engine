# First build

The first implementation of [the editor architecture](ARCHITECTURE.md): small,
built properly (a foundation, not a prototype), and ordered so the riskiest parts
are proven first. It is held to the strict checks in the README from the start.

## What it proves

1. **Strict updates are fast enough** (D1): painting through the orchestrator
   stays within one frame.
2. **The multi-process core works** (D5, D6, D10): handshake, snapshots and
   subscriptions, commands and events, revision numbers, failure handling.
3. **Windows cooperate** (D3, D13): a second view of a map, shared undo, and
   pick on map across windows.
4. **Writing in a text field is pleasant**: multi-line text, copy and paste, and
   macOS accent input (press-and-hold uses the same composition system as other
   input methods).
5. **Data is safe** (D9, D11, D17): always saved, format versions, write then
   rename, nothing lost when the orchestrator is killed.

Content is placeholder only: a tile floor, named anchors and a text document. No
game features.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 0. Setup | Strict lint and formatting (berad-sdk's policy and pinned tools); chosen dependency versions | Checks run in both modules | Done |
| 1. Core | Protocol, codec, framing; orchestrator services; the paint edit through the plug-in list; versioned files | Scripted clients paint and undo; contract tests pass; round-trip time measured | Done (PR #3) |
| 2. First window | One program running as orchestrator or window; a Map window painting in strict mode; inspector; speed readout | Painting within one frame | Done (PR #3) |
| 3. Two views and failures | New view; live updates; shared undo; crash notice and report; clean shutdown | Killing either process loses nothing; reports captured | Done (PR #3) |
| 4. Working together | Anchors in the floor (format version 2); a Text window with a draft saved on Enter; pick on map | Text editing judged by its author | Next |
| 5. Stretch | VM-guest embedding; "create version" through git | Optional | Later |

## Results so far

- Every agreed behaviour in steps 1-3 has an automated test, including a real
  SIGKILL of an orchestrator process followed by recovery of all 50 acknowledged
  edits from the journal, upgrade with backup, refusal of newer files and of a
  second orchestrator, undo clashes, and revision-gap recovery in the client.
- Real-window tests driven by input scripts (`make gui-test`) prove painting and
  undoing a stroke through to the saved file, a second view undoing the first
  view's stroke, a crashed view leaving a report, and windows closing themselves
  when the orchestrator is killed.
- Command-to-event time over a real socket, including the journal write
  (`make latency`): median 26 µs, p99 58 µs over 2,000 paints. Inside a real
  window, with the display asleep, the readout showed about 1.2 ms median; still
  far inside the budget, and to be rechecked with the window focused.

## Step 4 decisions

- Text fields keep a local draft; Enter saves (D1).
- Pick on map uses four messages through the orchestrator (D13).
- Anchors live inside the floor document ([vocabulary](VOCABULARY.md)), which
  raises the floor format to version 2: the first real use of upgrade with
  backup.

## If something fails

- **Strict updates too slow for a gesture:** adapt only where measured, for
  example with a local preview while dragging and one command at the end.
- **ebitenui text editing not good enough:** try an editor built on berad-text,
  which already has caret geometry, or DebugUI.

## Known limitations

- Project locking is Unix-only.
- A second launch on an open project is refused with directions (D10).
- Saving runs on the orchestrator's loop every two seconds, briefly delaying
  commands while it writes.
- A document's saved revision changes on each save, adding one line to diffs.
