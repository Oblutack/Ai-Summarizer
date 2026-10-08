const WORDS_PER_MINUTE = 200;

// Words as people count them: runs of letters, digits and their punctuation, split by whitespace.
// Markdown marks (#, *, -, >) stand alone often enough that they must not count as words.
export function countWords(text: string): number {
  return text.split(/\s+/).filter((token) => /[\p{L}\p{N}]/u.test(token)).length;
}

export function readingMinutes(words: number): number {
  return Math.max(1, Math.round(words / WORDS_PER_MINUTE));
}

export function describeLength(text: string): string {
  const words = countWords(text);
  if (words === 0) return "";
  return `${words.toLocaleString("en-US")} ${words === 1 ? "word" : "words"} · ${readingMinutes(words)} min read`;
}
