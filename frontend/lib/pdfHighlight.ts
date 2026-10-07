// Finds where a cited passage sits on a rendered PDF page, so the viewer can highlight it.
//
// The passage text comes from a different PDF reader than the one drawing the page (the server's),
// so the two disagree about spaces, line breaks and ligatures. Matching therefore ignores all
// whitespace and letter case, and works on the longest stretch of the passage that can be found.

// The parts of a PDF.js text item that matter here.
export interface TextPiece {
  str: string;
  // True when the piece ends a line; the reader of a line break sees a space, we need none.
  hasEOL?: boolean;
}

const MATCH_LENGTHS = [160, 100, 60, 40, 28, 20];
const WINDOW = 28;
const STEP = 14;

// Lowercase, no whitespace, no soft hyphens, common ligatures expanded.
export function compact(text: string): string {
  return text
    .replace(/­/g, "")
    .replace(/ﬀ/g, "ff")
    .replace(/ﬁ/g, "fi")
    .replace(/ﬂ/g, "fl")
    .replace(/ﬃ/g, "ffi")
    .replace(/ﬄ/g, "ffl")
    .toLowerCase()
    .replace(/\s+/g, "");
}

// Indexes of the text pieces that make up the passage, or [] when it cannot be located.
export function findHighlight(pieces: TextPiece[], passage: string): number[] {
  const wanted = compact(passage);
  if (wanted.length < 8) return [];

  let page = "";
  const owner: number[] = []; // for each character of `page`, the piece it came from
  pieces.forEach((piece, index) => {
    const text = compact(piece.str);
    page += text;
    for (let i = 0; i < text.length; i++) owner.push(index);
  });
  if (page.length === 0) return [];

  let start = -1;
  let skipped = 0; // how much of the start of the passage could not be found on the page
  // First choice: the start of the passage, as long a stretch as can be found.
  for (const length of MATCH_LENGTHS) {
    if (wanted.length < length) continue;
    const at = page.indexOf(wanted.slice(0, length));
    if (at !== -1) {
      start = at;
      break;
    }
  }
  // Otherwise any stretch from the middle (the start may differ, for example a heading). Only what
  // was actually found is highlighted: guessing where the missing start would be risks marking a
  // neighbouring line.
  if (start === -1) {
    for (let offset = STEP; offset + WINDOW <= wanted.length; offset += STEP) {
      const at = page.indexOf(wanted.slice(offset, offset + WINDOW));
      if (at !== -1) {
        start = at;
        skipped = offset;
        break;
      }
    }
  }
  // Short passages: try the whole of what there is.
  if (start === -1 && wanted.length < WINDOW) start = page.indexOf(wanted);
  if (start === -1) return [];

  const end = Math.min(page.length, start + wanted.length - skipped);
  const found = new Set<number>();
  for (let i = start; i < end; i++) found.add(owner[i]);
  return [...found].sort((a, b) => a - b);
}
