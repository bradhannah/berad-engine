# Increment 57: The files are shared

**Status:** Planned

The editor never writes over a file it has not seen, and says what it
cannot read rather than refusing to open, as agreed on 2026-11-13 in "the
files are shared" in [the editor features](../EDITOR_FEATURES.md#principles).
It settles the review's findings 2, 4 and 14, and fixes two losses found
while tracing them, 53 and 54. It builds on
[increment 55](0055-reopens-as-left.md), and is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it is to prove

1. **A file changed outside is never written over (2):**
   - before writing a document, the editor checks the file is as it read
     it, by size, modified time and content;
   - a document with nothing waiting in the editor takes in the file's
     version, and History says "Took in Clerk's day as changed on disk";
   - a document changed in both places asks Keep mine, Take the file's or
     Later. The version not kept goes to the project's backups, and Later
     writes nothing, keeping the changes in the journal under a lasting
     notice.
2. **A project that cannot open as it is says why (4):** a window names
   each file that cannot be read and why, and a journal that no longer fits
   the files with how many changes it holds, and offers Open without them
   or Quit. A file left out is never written over; a journal set aside is
   moved whole to the backups.
3. **What the editor does not know is said (14):** on opening, a lasting
   notice names each file and field the next save would drop, and each such
   file is copied to the backups before that save.
4. **Journal changes to a left-out document are kept (53),** not lost when
   the next save empties the journal.
5. **A required document that cannot be read is not written over (54).**
6. **Nothing else behaves differently:** the real-window walks pass.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Seen files | What each document's file was when read or written, checked before each write | Unit tests | Planned |
| 2. Taking in and asking | A file changed outside taken in, or asked about | Unit tests; a real-window walk | Planned |
| 3. Opening as it is | Bad files and a journal that does not fit named in a window, Open without them or Quit | Unit tests | Planned |
| 4. Unknown fields | Said at opening, backed up before the save that drops them | Unit tests | Planned |
| 5. Checks | `make check` and the real-window walks | All pass | Planned |
