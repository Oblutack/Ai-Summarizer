import { describe, expect, it } from "vitest";
import type { PodcastScript } from "../types";
import { MAX_SPEECH_CHUNK, pickVoices, scriptToMarkdown, splitForSpeech, type VoiceLike } from "./podcast";

describe("splitForSpeech", () => {
  it("keeps a short turn whole", () => {
    expect(splitForSpeech("Seven years on the unit.")).toEqual(["Seven years on the unit."]);
  });

  it("splits at sentence ends and keeps sentences together up to the limit", () => {
    const chunks = splitForSpeech("First sentence here. Second sentence here. Third sentence here.", 45);
    expect(chunks).toEqual(["First sentence here. Second sentence here.", "Third sentence here."]);
  });

  it("never exceeds the limit, even for one very long sentence", () => {
    const long = "word ".repeat(200).trim() + ".";
    const chunks = splitForSpeech(long, 60);
    expect(chunks.length).toBeGreaterThan(5);
    expect(chunks.every((c) => c.length <= 60)).toBe(true);
    expect(chunks.join(" ").replace(/\s+/g, " ")).toBe(long);
  });

  it("loses no words", () => {
    const text = "Alpha beta. Gamma delta epsilon! Zeta? Eta theta iota kappa lambda mu nu xi omicron pi rho sigma.";
    for (const max of [20, 40, 80, MAX_SPEECH_CHUNK]) {
      expect(splitForSpeech(text, max).join(" ")).toBe(text);
    }
  });

  it("handles awkward input", () => {
    expect(splitForSpeech("")).toEqual([]);
    expect(splitForSpeech("   \n  ")).toEqual([]);
    expect(splitForSpeech("No final punctuation")).toEqual(["No final punctuation"]);
    expect(splitForSpeech("Wait... what?!  Really.")).toEqual(["Wait... what?! Really."]);
  });

  it("treats a word longer than the limit as unsplittable but still returns it", () => {
    expect(splitForSpeech("x".repeat(50), 20).join("")).toBe("x".repeat(50));
  });
});

describe("pickVoices", () => {
  const voice = (name: string, lang: string, extra: Partial<VoiceLike> = {}): VoiceLike => ({
    voiceURI: name,
    name,
    lang,
    ...extra,
  });

  it("gives the hosts two different voices in the wanted language", () => {
    const voices = [voice("Marie", "fr-FR"), voice("Zira", "en-US"), voice("David", "en-US"), voice("Hazel", "en-GB")];
    const [a, b] = pickVoices(voices, "en-US");
    expect(a?.lang).toBe("en-US");
    expect(b?.lang).toBe("en-US");
    expect(a?.name).not.toBe(b?.name);
  });

  it("prefers the browser's default among equals", () => {
    const [a] = pickVoices([voice("One", "en-US"), voice("Two", "en-US", { default: true })], "en-US");
    expect(a?.name).toBe("Two");
  });

  it("matches the language without the region", () => {
    const [a] = pickVoices([voice("Marie", "fr-FR"), voice("Zira", "en-GB")], "en-US");
    expect(a?.name).toBe("Zira");
  });

  it("falls back to any voice, and to a single shared one", () => {
    const [a, b] = pickVoices([voice("Marie", "fr-FR")], "en-US");
    expect(a?.name).toBe("Marie");
    expect(b?.name).toBe("Marie");
  });

  it("returns nothing when there are no voices", () => {
    expect(pickVoices([], "en-US")).toEqual([undefined, undefined]);
  });
});

describe("scriptToMarkdown", () => {
  it("writes the title and each turn under its host's name", () => {
    const script: PodcastScript = {
      title: "Atlas X200",
      language: "",
      turns: [
        { speaker: "A", text: "What is it?" },
        { speaker: "B", text: "A compressor." },
      ],
    };
    expect(scriptToMarkdown(script)).toBe("# Atlas X200\n\n**Alex:** What is it?\n\n**Sam:** A compressor.\n");
  });
});

describe("languageCode", () => {
  it("knows every language the app writes in", async () => {
    const { LANGUAGES } = await import("./summaryOptions");
    const { languageCode } = await import("./podcast");
    for (const language of LANGUAGES) {
      expect(languageCode(language), language).toMatch(/^[a-z]{2}$/);
    }
    expect(languageCode("Spanish")).toBe("es");
    expect(languageCode("Klingon")).toBe("en");
  });
});
