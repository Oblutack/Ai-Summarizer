import { describe, expect, it } from "vitest";
import {
  DEFAULT_PLAYBACK,
  PLAYBACK_RATES,
  clampRate,
  firstTimeMark,
  loadPlayback,
  minutesLeft,
  sanitizePlayback,
  savePlayback,
  secondsFromClock,
  secondsLeft,
  secondsToSpeak,
  splitTimeMark,
} from "./playback";

function memoryStore(initial: Record<string, string> = {}) {
  const data = { ...initial };
  return {
    data,
    getItem: (key: string) => data[key] ?? null,
    setItem: (key: string, value: string) => {
      data[key] = value;
    },
  };
}

describe("clampRate", () => {
  it("keeps the speeds that are offered", () => {
    for (const rate of PLAYBACK_RATES) expect(clampRate(rate)).toBe(rate);
  });

  it("moves anything else to the nearest one, and nonsense to normal speed", () => {
    expect(clampRate(1.3)).toBe(1.25);
    expect(clampRate(3)).toBe(2);
    expect(clampRate(0.1)).toBe(0.75);
    for (const bad of [NaN, Infinity, "fast", null, undefined, {}]) expect(clampRate(bad)).toBe(1);
  });
});

describe("saved playback settings", () => {
  it("come back as they were saved", () => {
    const store = memoryStore();
    savePlayback({ rate: 1.5, voices: { read: "voice-a", A: "voice-b", B: "voice-c" } }, store);
    expect(loadPlayback(store)).toEqual({ rate: 1.5, voices: { read: "voice-a", A: "voice-b", B: "voice-c" } });
  });

  it("start at normal speed with no voices chosen", () => {
    expect(loadPlayback(memoryStore())).toEqual(DEFAULT_PLAYBACK);
    expect(loadPlayback(null)).toEqual(DEFAULT_PLAYBACK);
  });

  it("survive anything odd in storage", () => {
    expect(loadPlayback(memoryStore({ "inkling.playback": "{not json" }))).toEqual(DEFAULT_PLAYBACK);
    expect(sanitizePlayback({ rate: "fast", voices: { read: 5, A: "", B: "x".repeat(400), C: "other" } })).toEqual(DEFAULT_PLAYBACK);
    expect(sanitizePlayback({ rate: 1.3, voices: "nope" })).toEqual({ rate: 1.25, voices: {} });
    expect(sanitizePlayback(null)).toEqual(DEFAULT_PLAYBACK);
  });

  it("never get in the way when the store is full or blocked", () => {
    const broken = {
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("full");
      },
    };
    expect(() => savePlayback(DEFAULT_PLAYBACK, broken)).not.toThrow();
    expect(loadPlayback(broken)).toEqual(DEFAULT_PLAYBACK);
  });
});

describe("how long reading takes", () => {
  it("is words at about 150 a minute, shorter when faster", () => {
    const text = "word ".repeat(150);
    expect(secondsToSpeak(text, 1)).toBeCloseTo(60, 0);
    expect(secondsToSpeak(text, 2)).toBeCloseTo(30, 0);
    expect(secondsToSpeak("", 1)).toBe(0);
    expect(secondsToSpeak("   ", 1)).toBe(0);
  });

  it("adds up what is left from an item on", () => {
    const items = [{ text: "word ".repeat(150) }, { text: "word ".repeat(75) }, { text: "word ".repeat(75) }];
    expect(secondsLeft(items, 0, 1)).toBeCloseTo(120, 0);
    expect(secondsLeft(items, 1, 1)).toBeCloseTo(60, 0);
    expect(secondsLeft(items, 3, 1)).toBe(0);
    expect(secondsLeft(items, -4, 1)).toBeCloseTo(120, 0);
  });

  it("is shown in whole minutes, with a little under one minute counted as none", () => {
    expect([0, 20, 44, 45, 60, 61, 125, 3600].map(minutesLeft)).toEqual([0, 0, 0, 1, 1, 2, 3, 60]);
  });
});

describe("time marks in a transcript", () => {
  it("are read as seconds", () => {
    expect(secondsFromClock("0:07")).toBe(7);
    expect(secondsFromClock("12:30")).toBe(750);
    expect(secondsFromClock("1:02:05")).toBe(3725);
    expect(secondsFromClock(" 5:00 ")).toBe(300);
  });

  it("are refused when they are not times", () => {
    for (const bad of ["", "12", "1:2", "12:60", "1:75:00", "ab:cd", "12:30:15:10", "-1:00"]) expect(secondsFromClock(bad), bad).toBeNull();
  });

  it("start a paragraph of a transcript", () => {
    expect(splitTimeMark("[12:30] Tom will book the venue.")).toEqual({ clock: "12:30", seconds: 750, rest: "Tom will book the venue." });
    expect(splitTimeMark("[1:02:05] Late in the day.")?.seconds).toBe(3725);
    expect(splitTimeMark("Transcript of sync.mp3 (length 0:53).")).toBeNull();
    expect(splitTimeMark("[99:99] nonsense")).toBeNull();
    expect(splitTimeMark("[a note] not a time")).toBeNull();
  });

  it("are found inside a passage that a chat answer cites", () => {
    expect(firstTimeMark("Transcript of x.mp3 (length 5:00).\n\n[2:15] Priya will send the budget.")).toEqual({ clock: "2:15", seconds: 135 });
    expect(firstTimeMark("No marks here, only a [bracket].")).toBeNull();
  });
});
