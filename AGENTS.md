# berad-engine contributor instructions

Read [README.md](README.md) first. This repository is public: everything in it
must be generic and self-contained.

## Boundaries

- Two Go modules: `engine` (what games run) and `editor` (the shared editor).
  The engine module must never import the editor module. Games use the engine
  module and berad-sdk, never the editor module.
- Sort new code with one test. Would any Ebitengine game use it unchanged? It
  belongs in [berad-sdk](https://github.com/bradhannah/berad-sdk). Is it about
  top-down tile-RPG worlds, workflows or authoring? It belongs here. Is it one
  game's content, settings or unique modes? It belongs in that game's repository.
- Add functionality only when a consuming game needs it now. Do not build
  features, abstractions or extension points for hypothetical future games.
- Keep OS-specific features behind abstractions.

## Design rules

- Organise the framework by layer and features as plug-ins. A plug-in registers
  its edits, workflow node types, object characteristics and editor sections with
  the framework. Each program registers its plug-ins explicitly in one list;
  duplicate names must fail at startup. Do not rely on import side effects.
- Keep a plug-in's runtime part (engine module) separate from its editor part
  (editor module), each behind small interfaces.
- Every kind of edit knows what it changes, which documents it touches, and how
  to undo itself. Every edit kind must pass the shared test that applying and
  then undoing it restores exactly the same data. Every message codec must
  round-trip every message.
- Every saved file (project documents, journal entries, game saves) carries a
  format version from the first implementation. Upgrade old files step by step
  on load, write to a temporary file and rename it into place, and back up a file
  before upgrading it.
- Store workflow layout (node positions and similar) separately from workflow
  logic, so layout changes do not obscure logic changes in diffs.
- In the editor, the orchestrator is the only writer of project data and is built
  from small services behind small interfaces, not one large object. The
  editor window's panes never read or write project files directly; they reach
  project data only through the protocol, though they share its process.
- Apply SOLID principles at these seams; do not add interfaces where no seam is
  needed.

## Working practice

- Do not imply that an implementation, test suite or tool exists before it does.
  Report what was actually verified.
- Public content only: no private repository links, credentials, personal
  machine paths, private evidence or game content.
- Commit, push, create pull requests or change remote state only with
  authorization. Use a dedicated branch; do not commit to `main`. Humans merge
  pull requests.
- Record each increment of work in `docs/increments/`, as
  `NNNN-topic.md`: the next number, four digits, and a short topic in lower
  case with hyphens. Start from [the template](docs/templates/increment.md),
  keep its status line current (Planned, Built, Judged), and list it in
  [docs/README.md](docs/README.md). Only its author marks an increment Judged.
- Project-authored source and documentation are MIT-licensed. Preserve the
  notices of any transferred third-party code.
