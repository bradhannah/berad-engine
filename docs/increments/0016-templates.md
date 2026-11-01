# Increment 16: Templates

**Status:** Built

The sixteenth increment gives the Pixel editor **templates**: rules about
where a sprite may be painted, and guide lines to draw to, so art meets the
game's needs without its author being an artist. A template marks each pixel
**open**, **discouraged** or **not allowed**: a pickup might never use its
outermost pixel, and a character might keep three pixels clear at each side.
The pencil and the fill never paint where a template does not allow it, and
the Pixel editor shows and counts what is discouraged. Templates are
documents of their own, edited in a **Template editor** tab, and a sprite is
drawn to at most one. It builds on [increment 11](0011-drawing-tiles.md)'s
Pixel editor and on the template agreed in [the pixel editor's
design](../EDITOR_FEATURES.md#drawing), and is held to the same strict checks
as [increment 1](0001-foundation.md).

## What it proves

1. **A template is a few rules:** a rule marks the pixels within a width of
   the tile's border, its sides, its top or its bottom as discouraged or not
   allowed, so one template fits a sprite of any size. Where rules overlap,
   the strictest wins.
2. **Not allowed is absolute:** the pencil and the fill skip the pixels a
   sprite's template does not allow, and say how many they skipped; the
   eraser still clears them. To paint there, the author changes or removes
   the template.
3. **Discouraged is shown and counted:** discouraged pixels can be painted;
   the Pixel editor shades them, counts the ones painted, and marks them on
   request.
4. **Guides are lines to draw to:** a template's named guides, across or
   down, are drawn over the tile and are never part of it.
5. **Changing a template never erases art:** pixels it newly forbids are
   counted and marked, and Clear them clears them in one undo step.
6. **Templates are easy to apply:** the Pixel editor chooses a sprite's
   template from a menu, and New ▾ makes a sprite already drawn to one.
7. **Templates are documents:** each is listed in the project browser, edited
   in its own Template editor tab with a preview, saved as a versioned file of
   its own and undone like any other change.

## Decisions

Asked for by its author on 2026-11-01, paraphrased: to tell the person
drawing that some pixels are discouraged and others not allowed, such as an
item that never uses its outside border pixels or a humanoid NPC that never
comes within 3 pixels of its left and right sides, and to make that easy to
apply, since its author is not an artist.

Agreed by its author on 2026-11-01, before building, from the wireframes on
the canvas "Pixel templates, origin and families":

- **Not allowed is absolute:** tools never paint there. Breaking it means
  changing or removing the sprite's template; there is no per-sprite
  exception.
- **"Not allowed" and "discouraged"** are the editor's words, its author's
  own; "not allowed" replaces "kept clear" in the design.
- **A sprite's template is chosen in the Pixel editor,** and **New ▾ offers a
  sprite for each template**, such as "Sprite: Pickup". Folder defaults wait
  until folders can carry settings.
- **Templates are edited in a Template editor tab** of their own, like the
  Palette Editor, with a magnified preview.
- **One-way import of art is allowed in principle** but is not part of this
  increment: how art comes in is worked out once there is a real pack to
  bring in. A tile's origin and the Made with AI mark come with it.

Chosen while building, for its author to check:

- **A rule covers a width from its edge,** and where rules overlap the
  strictest wins, so "the next ring in is discouraged" is a border rule of
  width 2 beside one of width 1 that is not allowed. Rules measure from four
  edges: every side, the left and right sides, the top or the bottom.
- **A guide is measured from the top or the left, the bottom or the right,
  or the middle,** so a template fits a sprite of any size; it lies between
  pixels, and its name is drawn on a pale label kept inside the canvas.
- **A sprite names its template in its own file,** as sprite format version
  2, which reads version 1 unchanged; games ignore it. Templates themselves
  are editor documents, kept by the editor's art plug-in, since no game
  reads them.
- **Deleting a template is allowed;** its delete lists the sprites drawn to
  it, and they are then drawn to none. A template the project no longer
  lists counts as none.
- **A new template has no rules.** + Rule adds a border 1 pixel wide, not
  allowed; + Guide adds "Guide N" across the middle. The Template editor's
  buttons step through their choices (edge, mark, line, where from) rather
  than opening menus, each click one undo step; a guide's name is typed and
  waits for Save, Enter or Cmd+S.
- **The Pixel editor's Template toggle** hides the marks and guides; the
  tools still never paint where it is not allowed. Template sits above Shows
  in its panel, and its menu ends with Edit template….
- **The look:** not allowed is tinted dark and hatched, discouraged tinted
  amber and dotted, and guides are dashed blue, as in the first pixel editor
  wireframes. Show them outlines painted discouraged pixels in amber and
  not-allowed ones in red; the readout says how many in muted text, or as a
  problem with Clear them when any are not allowed.
- **The Template editor's preview** shows the template at the project's tile
  size, over any sprite chosen in Show on, which is never saved. Its tab is
  filed under Templates, after Sprites on the bar.
- **The sample** has a Pickup template (a border kept clear, the next ring
  discouraged, guides for the middle and where it rests) and a Character
  template (three pixels kept clear at each side, the fourth and the top two
  rows discouraged, guides for the middle, eyes and feet). The lantern is
  drawn to Pickup, its top and bottom rows touching the discouraged ring; the
  hero and the clerk to Character.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Template model | The template document: rules, guides and the mark each pixel gets; a sprite naming its template; the edits that set a template and a sprite's template | Unit tests of the marks and the format; both edit kinds pass the shared apply-then-undo test | Built |
| 2. Drawing to a template | The Pixel editor's Template menu, the overlay and guides, tools that skip what is not allowed, the counts, Show them and Clear them | View tests of each tool against a template, of the counts and of Clear them | Built |
| 3. The Template editor | A Template editor tab: rules and guides, and a preview over a chosen sprite | View tests of adding, changing and removing rules and guides | Built |
| 4. The project browser | A Templates section, New template, opening the Template editor, and New ▾ offering a sprite per template | View tests of the section and the menu | Built |
| 5. The sample | The sample project gets a Pickup and a Character template, used by its sprites; a real-window test draws to one | The sample's headless test; `make check` and the real-window suite pass | Built |

## Step results

- **Template model:** `editor/plugins/art` gains the template document,
  version 1: rules (edge, width, mark) and guides (name, line, where from,
  distance), the mark of each pixel, and the edits `art.template.set` and
  `art.sprite.template`, both passing the shared apply-then-undo test. The
  engine's sprite format, version 2, names the template a sprite is drawn
  to, and a sprite refers to it, so deleting a template lists its sprites.
- **Drawing to a template:** the Pixel editor's Template menu chooses the
  sprite's template; its marks and guides are drawn over the tile, by
  `editor/views/tileart`, which the Template editor shares. The pencil and
  the fill skip not-allowed pixels and the status bar says how many; the
  eraser clears them. The readout counts painted pixels that are
  discouraged or not allowed; Show them marks them and Clear them clears the
  ones not allowed in one undo step. Tested against an 8-pixel template.
- **The Template editor:** `editor/views/templateview`, a tab of its own,
  edits a template's rules and guides with buttons, each one undo step, and
  its guides' names with a save bar; its preview shows the template at the
  project's tile size over a chosen sprite. A template icon, a frame round a
  clear middle, joins the art types' icons.
- **The project browser:** a Templates section with a count and a [+],
  which makes a template and opens it; double-clicking one opens it; New ▾
  has Template and Template folder, and a "Sprite: <template>" for each
  template, which makes a sprite drawn to it.
- **The sample:** Pickup and Character, as above. The sample's headless
  test checks the lantern is painted only where Pickup allows and the
  figures are drawn to Character. A real-window test opens the lantern: a
  stroke from its edge skips the pixel Pickup keeps clear, and Edit
  template… opens Pickup in the Template editor, which adds a rule; both are
  saved. `make check` passes with no lint findings, and so does the
  real-window suite, once its project browser script waits for the Map's tab
  before quitting when alone: until then it could quit before the Map
  opened, which the sample's two new documents made happen.

## Not in this increment

- **Painting a template's marks by hand,** for shapes rules cannot make; rules
  cover the pickup and the character.
- **Folder defaults,** until folders can carry settings.
- **Families of pieces** (wall and path families made into a folder, each
  piece with a template from the family) and the **origin of art** with
  **import**, from the same canvas, come in later increments.
