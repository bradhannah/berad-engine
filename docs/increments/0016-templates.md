# Increment 16: Templates

**Status:** Planned

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

- (none yet)

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Template model | The template document: rules, guides and the mark each pixel gets; a sprite naming its template; the edits that set a template and a sprite's template | Unit tests of the marks and the format; both edit kinds pass the shared apply-then-undo test | Planned |
| 2. Drawing to a template | The Pixel editor's Template menu, the overlay and guides, tools that skip what is not allowed, the counts, Show them and Clear them | View tests of each tool against a template, of the counts and of Clear them | Planned |
| 3. The Template editor | A Template editor tab: rules and guides, a preview over a chosen sprite, and where it is used | View tests of adding, changing and removing rules and guides | Planned |
| 4. The project browser | A Templates section, New template, opening the Template editor, and New ▾ offering a sprite per template | View tests of the section and the menu | Planned |
| 5. The sample | The sample project gets a Pickup and a Character template, used by its sprites; a real-window test draws to one | The sample's headless test; `make check` and the real-window suite pass | Planned |

## Step results

(none yet)

## Not in this increment

- **Painting a template's marks by hand,** for shapes rules cannot make; rules
  cover the pickup and the character.
- **Folder defaults,** until folders can carry settings.
- **Families of pieces** (wall and path families made into a folder, each
  piece with a template from the family) and the **origin of art** with
  **import**, from the same canvas, come in later increments.
