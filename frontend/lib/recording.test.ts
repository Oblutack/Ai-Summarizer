import { describe, expect, it } from "vitest";
import {
  RECORDING_BITS_PER_SECOND,
  RECORDING_MAX_BYTES,
  RECORDING_MAX_MINUTES,
  extensionFor,
  formatClock,
  pickRecordingType,
  recordingFileName,
} from "./recording";
import { MAX_AUDIO_MB } from "./links";

describe("pickRecordingType", () => {
  it("takes WebM with Opus where it can, which is what Chrome and Firefox record", () => {
    expect(pickRecordingType(() => true)).toBe("audio/webm;codecs=opus");
  });

  it("falls back to what the browser supports, MP4 on Safari", () => {
    expect(pickRecordingType((type) => type === "audio/mp4")).toBe("audio/mp4");
    expect(pickRecordingType((type) => type === "audio/webm")).toBe("audio/webm");
  });

  it("is null when nothing is supported", () => {
    expect(pickRecordingType(() => false)).toBeNull();
  });
});

describe("extensionFor", () => {
  it("gives the extension the server and the transcription service know", () => {
    expect(extensionFor("audio/webm;codecs=opus")).toBe(".webm");
    expect(extensionFor("audio/mp4")).toBe(".m4a");
    expect(extensionFor("audio/mp4;codecs=mp4a.40.2")).toBe(".m4a");
    expect(extensionFor("audio/ogg;codecs=opus")).toBe(".ogg");
    expect(extensionFor("AUDIO/WEBM")).toBe(".webm");
    expect(extensionFor("")).toBe(".webm");
  });
});

describe("recordingFileName", () => {
  it("is the date and time in local time, without characters a file name cannot have", () => {
    const name = recordingFileName("audio/webm", new Date(2026, 9, 9, 14, 5));
    expect(name).toBe("recording 2026-10-09 14.05.webm");
    expect(name).not.toMatch(/[:\\/*?"<>|]/);
  });

  it("pads single digits and follows the type", () => {
    expect(recordingFileName("audio/mp4", new Date(2026, 0, 3, 7, 8))).toBe("recording 2026-01-03 07.08.m4a");
  });
});

describe("formatClock", () => {
  it("reads like a player", () => {
    expect([0, 999, 7000, 65_000, 754_000, 3_599_000, 3_600_000, 3_725_900].map(formatClock)).toEqual([
      "0:00", "0:00", "0:07", "1:05", "12:34", "59:59", "1:00:00", "1:02:05",
    ]);
    expect(formatClock(-5000)).toBe("0:00");
  });
});

describe("the recording limits", () => {
  it("keep the longest recording under what the server takes", () => {
    const longestBytes = (RECORDING_BITS_PER_SECOND / 8) * RECORDING_MAX_MINUTES * 60;
    expect(longestBytes).toBeLessThan(RECORDING_MAX_BYTES);
    expect(RECORDING_MAX_BYTES).toBeLessThan(MAX_AUDIO_MB * 1024 * 1024);
  });
});
