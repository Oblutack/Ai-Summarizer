import type { Page } from "@playwright/test";

// Headless Chrome has no speech voices, so tests install a stand-in engine that records what would be
// spoken, and with which voice, and finishes each piece after a short, adjustable delay.
export async function installFakeSpeech(page: Page, options: { voices?: boolean; delay?: number } = {}) {
  await page.addInitScript(
    ({ withVoices, delay }) => {
      const w = window as unknown as Record<string, unknown>;
      const spoken: { text: string; voice: string | null; rate: number; pitch: number }[] = [];
      const timers = new Set<number>();
      let paused = 0;
      let resumed = 0;
      w.__spoken = spoken;
      w.__speech = { delay, get paused() { return paused; }, get resumed() { return resumed; } };

      class FakeUtterance {
        text: string;
        voice: { voiceURI: string } | null = null;
        rate = 1;
        pitch = 1;
        onend: (() => void) | null = null;
        onerror: (() => void) | null = null;
        constructor(text: string) {
          this.text = text;
        }
      }
      w.SpeechSynthesisUtterance = FakeUtterance;

      const voices = withVoices
        ? [
            { voiceURI: "fake-alex", name: "Fake Alex", lang: "en-US", default: true },
            { voiceURI: "fake-sam", name: "Fake Sam", lang: "en-GB", default: false },
          ]
        : [];
      Object.defineProperty(window, "speechSynthesis", {
        configurable: true,
        value: {
          getVoices: () => voices,
          addEventListener() {},
          removeEventListener() {},
          speak(u: FakeUtterance) {
            spoken.push({ text: u.text, voice: u.voice?.voiceURI ?? null, rate: u.rate, pitch: u.pitch });
            const timer = window.setTimeout(() => {
              timers.delete(timer);
              u.onend?.();
            }, (w.__speech as { delay: number }).delay);
            timers.add(timer);
          },
          cancel() {
            timers.forEach((t) => window.clearTimeout(t));
            timers.clear();
          },
          pause() {
            paused++;
          },
          resume() {
            resumed++;
          },
        },
      });
    },
    { withVoices: options.voices ?? true, delay: options.delay ?? 30 }
  );
}

export const spoken = (page: Page) =>
  page.evaluate(() => (window as unknown as { __spoken: { text: string; voice: string | null; rate: number; pitch: number }[] }).__spoken);

