import type { PodcastScript } from "../types";

// Browsers stop reading a long utterance part-way (Chrome cuts off after roughly fifteen seconds), so
// each turn is read in pieces that end at sentence boundaries wherever possible.
export const MAX_SPEECH_CHUNK = 180;

export function splitForSpeech(text: string, max = MAX_SPEECH_CHUNK): string[] {
  const clean = text.replace(/\s+/g, " ").trim();
  if (!clean) return [];
  const sentences = clean.match(/[^.!?…]+[.!?…]+["')\]]*\s*|[^.!?…]+$/g) ?? [clean];
  const chunks: string[] = [];
  let current = "";
  const flush = () => {
    if (current.trim()) chunks.push(current.trim());
    current = "";
  };
  for (const sentence of sentences) {
    if (sentence.length > max) {
      // A very long sentence is cut at the last space before the limit.
      flush();
      let rest = sentence.trim();
      while (rest.length > max) {
        const cut = rest.lastIndexOf(" ", max);
        const at = cut > 0 ? cut : max;
        chunks.push(rest.slice(0, at).trim());
        rest = rest.slice(at).trim();
      }
      current = rest;
      continue;
    }
    if (current && (current + sentence).length > max) flush();
    current += sentence;
  }
  flush();
  return chunks;
}

// The parts of a browser voice that matter here.
export interface VoiceLike {
  voiceURI: string;
  name: string;
  lang: string;
  default?: boolean;
}

// Two different voices for the two hosts. Voices in the wanted language come first (the document's
// language, or the browser's own); with only one voice available both hosts share it, and the caller
// tells them apart by pitch.
export function pickVoices(voices: VoiceLike[], wantedLang: string): [VoiceLike | undefined, VoiceLike | undefined] {
  const base = wantedLang.toLowerCase().split("-")[0];
  const rank = (v: VoiceLike) => {
    const lang = v.lang.toLowerCase();
    return (lang === wantedLang.toLowerCase() ? 0 : lang.startsWith(base) ? 1 : 2) - (v.default ? 0.5 : 0);
  };
  const sorted = [...voices].sort((a, b) => rank(a) - rank(b));
  const first = sorted[0];
  const second = sorted.find((v) => v.voiceURI !== first?.voiceURI && v.name !== first?.name) ?? first;
  return [first, second];
}

// The script as a Markdown document, for saving.
export function scriptToMarkdown(script: PodcastScript, hosts = { A: "Alex", B: "Sam" }): string {
  const lines = script.turns.map((t) => `**${hosts[t.speaker]}:** ${t.text}`);
  return `# ${script.title}\n\n${lines.join("\n\n")}\n`;
}

// The base language code of one of the languages we write summaries and scripts in, for choosing a
// matching voice. Anything unknown falls back to English.
const LANGUAGE_CODES: Record<string, string> = {
  English: "en",
  Spanish: "es",
  French: "fr",
  German: "de",
  Italian: "it",
  Portuguese: "pt",
  Dutch: "nl",
  Polish: "pl",
  Turkish: "tr",
  Russian: "ru",
  Serbian: "sr",
  Croatian: "hr",
  Bosnian: "bs",
  Chinese: "zh",
  Japanese: "ja",
};

export function languageCode(language: string): string {
  return LANGUAGE_CODES[language] ?? "en";
}
