// What is shared by everything that plays: reading a summary aloud, the podcast, and recordings. The settings a person
// chooses once (how fast, which voices) are remembered on this device, so they are not asked for again each time.

// Speeds. 1 is normal; most people who listen to spoken text a lot like it a little faster.
export const PLAYBACK_RATES = [0.75, 1, 1.25, 1.5, 1.75, 2] as const;
export const DEFAULT_RATE = 1;

// The part of speech a voice is chosen for: reading a summary, and the two hosts of a podcast.
export type VoiceRole = "read" | "A" | "B";

export interface PlaybackSettings {
  rate: number;
  // The voice (its voiceURI) chosen for each role. A role with none takes the best one for the language.
  voices: Partial<Record<VoiceRole, string>>;
}

export const DEFAULT_PLAYBACK: PlaybackSettings = { rate: DEFAULT_RATE, voices: {} };

const KEY = "inkling.playback";
type Store = Pick<Storage, "getItem" | "setItem">;

function browserStore(): Store | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage;
  } catch {
    return null; // blocked storage (private windows, strict settings)
  }
}

// The nearest speed that is offered: anything stored may be old or edited by hand.
export function clampRate(value: unknown): number {
  if (typeof value !== "number" || !Number.isFinite(value)) return DEFAULT_RATE;
  return PLAYBACK_RATES.reduce((best, rate) => (Math.abs(rate - value) < Math.abs(best - value) ? rate : best), DEFAULT_RATE as number);
}

export function sanitizePlayback(raw: unknown): PlaybackSettings {
  const data = typeof raw === "object" && raw !== null ? (raw as Record<string, unknown>) : {};
  const voices: PlaybackSettings["voices"] = {};
  const stored = typeof data.voices === "object" && data.voices !== null ? (data.voices as Record<string, unknown>) : {};
  for (const role of ["read", "A", "B"] as const) {
    const voice = stored[role];
    if (typeof voice === "string" && voice.length > 0 && voice.length < 300) voices[role] = voice;
  }
  return { rate: clampRate(data.rate), voices };
}

export function loadPlayback(store: Store | null = browserStore()): PlaybackSettings {
  try {
    const stored = store?.getItem(KEY);
    return sanitizePlayback(stored ? JSON.parse(stored) : null);
  } catch {
    return DEFAULT_PLAYBACK;
  }
}

export function savePlayback(settings: PlaybackSettings, store: Store | null = browserStore()): void {
  try {
    store?.setItem(KEY, JSON.stringify(settings));
  } catch {
    // Remembering is a convenience: a full or blocked store must never get in the way.
  }
}

// About how long some text takes to read aloud at a speed: an average voice speaks around 150 words a minute.
const WORDS_PER_SECOND = 2.5;

export function secondsToSpeak(text: string, rate: number): number {
  const words = text.trim() ? text.trim().split(/\s+/).length : 0;
  return words / (WORDS_PER_SECOND * (rate > 0 ? rate : 1));
}

// The time left from an item on, for "about 3 min left".
export function secondsLeft(items: { text: string }[], from: number, rate: number): number {
  return items.slice(Math.max(0, from)).reduce((total, item) => total + secondsToSpeak(item.text, rate), 0);
}

// Whole minutes to show: "less than a minute" is 0, anything else rounds up.
export function minutesLeft(seconds: number): number {
  return seconds < 45 ? 0 : Math.ceil(seconds / 60);
}

// The time marks of a transcript ("[12:30] ...") as seconds, so a click on one can start the recording there.
export function secondsFromClock(clock: string): number | null {
  const match = clock.trim().match(/^(?:(\d+):)?(\d{1,2}):(\d{2})$/);
  if (!match) return null;
  const [, hours, minutes, seconds] = match;
  if (Number(seconds) > 59 || (hours !== undefined && Number(minutes) > 59)) return null;
  return Number(hours ?? 0) * 3600 + Number(minutes) * 60 + Number(seconds);
}

// A paragraph of a transcript split into its time mark and its words, or null if it has no time mark.
export function splitTimeMark(paragraph: string): { clock: string; seconds: number; rest: string } | null {
  const match = paragraph.match(/^\[((?:\d+:)?\d{1,2}:\d{2})\]\s*([\s\S]*)$/);
  if (!match) return null;
  const seconds = secondsFromClock(match[1]);
  return seconds === null ? null : { clock: match[1], seconds, rest: match[2] };
}

// The first time mark inside a longer text (a passage cited in a chat answer), in seconds.
export function firstTimeMark(text: string): { clock: string; seconds: number } | null {
  const match = text.match(/\[((?:\d+:)?\d{1,2}:\d{2})\]/);
  if (!match) return null;
  const seconds = secondsFromClock(match[1]);
  return seconds === null ? null : { clock: match[1], seconds };
}
