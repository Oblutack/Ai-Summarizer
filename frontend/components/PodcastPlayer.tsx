"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import axios from "axios";
import { API_URL, apiError } from "../lib/api";
import { languageCode, pickVoices, scriptToMarkdown, splitForSpeech } from "../lib/podcast";
import { LANGUAGES } from "../lib/summaryOptions";
import type { PodcastScript } from "../types";

const HOSTS = { A: "Alex", B: "Sam" } as const;
const RATES = [0.8, 1, 1.15, 1.3, 1.5];

type Status = "idle" | "playing" | "paused";

interface PodcastPlayerProps {
  documentId: number;
}

// A short two-host conversation about a document. The server writes the script (once, then it is kept);
// the browser reads it aloud with two of its own voices, so nothing is sent anywhere to be spoken.
export default function PodcastPlayer({ documentId }: PodcastPlayerProps) {
  const [script, setScript] = useState<PodcastScript | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [language, setLanguage] = useState("");

  const [voices, setVoices] = useState<SpeechSynthesisVoice[]>([]);
  const [voiceA, setVoiceA] = useState("");
  const [voiceB, setVoiceB] = useState("");
  const [rate, setRate] = useState(1);
  const [status, setStatus] = useState<Status>("idle");
  const [current, setCurrent] = useState(-1);

  const synth = typeof window !== "undefined" ? window.speechSynthesis : undefined;
  // Bumped to make the callbacks of an earlier playback do nothing once it has been replaced or stopped.
  const run = useRef(0);
  const settings = useRef({ rate, voiceA, voiceB, voices });
  settings.current = { rate, voiceA, voiceB, voices };

  // Ask for the script: the stored one if there is one, else a newly written one.
  const load = useCallback(
    async (opts: { language: string; regenerate: boolean }) => {
      setLoading(true);
      setError("");
      try {
        const response = await axios.post<PodcastScript>(`${API_URL}/documents/${documentId}/podcast`, opts);
        setScript(response.data);
      } catch (err) {
        setError(apiError(err, "Could not make the podcast."));
      } finally {
        setLoading(false);
      }
    },
    [documentId]
  );

  useEffect(() => {
    void load({ language: "", regenerate: false });
  }, [load]);

  // The browser's voices arrive asynchronously in some browsers.
  useEffect(() => {
    if (!synth) return;
    const read = () => setVoices(synth.getVoices());
    read();
    synth.addEventListener?.("voiceschanged", read);
    return () => synth.removeEventListener?.("voiceschanged", read);
  }, [synth]);

  // Choose two different voices for the hosts once there are voices and a script.
  useEffect(() => {
    if (voices.length === 0) return;
    const wanted = script?.language ? languageCode(script.language) : (navigator.language ?? "en");
    const [a, b] = pickVoices(voices, wanted);
    setVoiceA((cur) => cur || a?.voiceURI || "");
    setVoiceB((cur) => cur || b?.voiceURI || "");
  }, [voices, script?.language]);

  const stop = useCallback(() => {
    run.current++;
    synth?.cancel();
    setStatus("idle");
    setCurrent(-1);
  }, [synth]);

  // Stop talking when the player goes away.
  useEffect(() => () => stop(), [stop]);

  const speakFrom = useCallback(
    (start: number) => {
      if (!synth || !script) return;
      synth.cancel();
      const id = ++run.current;
      setStatus("playing");
      setError("");

      const speakTurn = (i: number) => {
        if (run.current !== id) return;
        if (i >= script.turns.length) {
          setStatus("idle");
          setCurrent(-1);
          return;
        }
        setCurrent(i);
        const turn = script.turns[i];
        const chunks = splitForSpeech(turn.text);

        const speakChunk = (k: number) => {
          if (run.current !== id) return;
          if (k >= chunks.length) {
            speakTurn(i + 1);
            return;
          }
          const { rate: r, voiceA: a, voiceB: b, voices: all } = settings.current;
          const utterance = new SpeechSynthesisUtterance(chunks[k]);
          const wanted = turn.speaker === "A" ? a : b;
          const voice = all.find((v) => v.voiceURI === wanted);
          if (voice) utterance.voice = voice;
          utterance.rate = r;
          // With a single voice installed, a lower pitch is what tells the second host apart.
          if (a === b && turn.speaker === "B") utterance.pitch = 0.8;
          utterance.onend = () => speakChunk(k + 1);
          utterance.onerror = (event) => {
            if (run.current !== id || event.error === "interrupted" || event.error === "canceled") return;
            run.current++;
            setStatus("idle");
            setCurrent(-1);
            setError("Your browser could not read this aloud. The script is still here to read.");
          };
          synth.speak(utterance);
        };
        speakChunk(0);
      };
      speakTurn(start);
    },
    [synth, script]
  );

  const pause = () => {
    synth?.pause();
    setStatus("paused");
  };
  const resume = () => {
    synth?.resume();
    setStatus("playing");
  };

  const download = () => {
    if (!script) return;
    const blob = new Blob([scriptToMarkdown(script, HOSTS)], { type: "text/markdown" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `${script.title.replace(/[^\p{L}\p{N}]+/gu, "-").replace(/^-|-$/g, "") || "podcast"}.md`;
    link.click();
    URL.revokeObjectURL(url);
  };

  const canSpeak = !!synth && voices.length > 0;

  return (
    <div className="mt-4 border-2 border-ink rounded-md p-3" data-testid="podcast">
      {loading && (
        <p className="text-xl tracking-wider uppercase text-ink/70" role="status">
          Writing the script... this takes a few seconds.
        </p>
      )}

      {error && (
        <p className="text-red-500 text-lg" role="alert">
          {error}
        </p>
      )}

      {script && (
        <>
          <h4 className="text-2xl font-bold tracking-wide" data-testid="podcast-title">
            {script.title}
          </h4>
          <p className="text-base text-ink/70">
            A conversation between {HOSTS.A} and {HOSTS.B}, written from this document. Voices are your browser&apos;s
            own.
          </p>

          {!canSpeak && (
            <p className="mt-2 text-lg" role="status">
              {synth
                ? "Your browser has no speech voices installed, so it cannot read this aloud. You can still read the script."
                : "Your browser cannot read text aloud. You can still read the script."}
            </p>
          )}

          <div className="mt-3 flex flex-wrap items-center gap-3 text-xl">
            {status === "idle" && (
              <button
                type="button"
                onClick={() => speakFrom(0)}
                disabled={!canSpeak}
                className="rounded-md border-2 border-ink bg-ink px-5 py-1 uppercase font-bold text-canvas hover:opacity-90 disabled:opacity-40"
              >
                Play
              </button>
            )}
            {status === "playing" && (
              <button
                type="button"
                onClick={pause}
                className="rounded-md border-2 border-ink px-5 py-1 uppercase font-bold hover:bg-ink hover:text-canvas"
              >
                Pause
              </button>
            )}
            {status === "paused" && (
              <button
                type="button"
                onClick={resume}
                className="rounded-md border-2 border-ink bg-ink px-5 py-1 uppercase font-bold text-canvas hover:opacity-90"
              >
                Resume
              </button>
            )}
            {status !== "idle" && (
              <button
                type="button"
                onClick={stop}
                className="rounded-md border-2 border-ink px-5 py-1 uppercase font-bold hover:bg-ink hover:text-canvas"
              >
                Stop
              </button>
            )}
            <label className="flex items-center gap-2">
              <span className="uppercase tracking-wider text-base">Speed</span>
              <select
                value={rate}
                onChange={(e) => setRate(Number(e.target.value))}
                className="rounded border-2 border-ink bg-canvas px-1"
              >
                {RATES.map((r) => (
                  <option key={r} value={r}>
                    {r}x
                  </option>
                ))}
              </select>
            </label>
            <button
              type="button"
              onClick={download}
              className="rounded-md border-2 border-ink px-4 py-1 text-base uppercase hover:bg-ink hover:text-canvas"
            >
              Save script
            </button>
          </div>

          {canSpeak && (
            <div className="mt-2 flex flex-wrap gap-4 text-base">
              {(
                [
                  ["A", voiceA, setVoiceA],
                  ["B", voiceB, setVoiceB],
                ] as const
              ).map(([host, value, set]) => (
                <label key={host} className="flex items-center gap-2">
                  <span className="uppercase tracking-wider">{HOSTS[host]}&apos;s voice</span>
                  <select
                    value={value}
                    onChange={(e) => set(e.target.value)}
                    className="max-w-[16rem] rounded border-2 border-ink bg-canvas px-1"
                  >
                    {voices.map((v) => (
                      <option key={v.voiceURI} value={v.voiceURI}>
                        {v.name} ({v.lang})
                      </option>
                    ))}
                  </select>
                </label>
              ))}
            </div>
          )}

          <ol className="mt-4 max-h-80 space-y-2 overflow-y-auto pr-1 text-xl" aria-label="Script">
            {script.turns.map((turn, i) => (
              <li
                key={i}
                data-testid={`podcast-turn-${i}`}
                data-active={current === i}
                className={`border-l-4 pl-3 ${current === i ? "border-ink font-bold" : "border-ink/20"}`}
              >
                <button
                  type="button"
                  onClick={() => canSpeak && speakFrom(i)}
                  disabled={!canSpeak}
                  className="block w-full text-left disabled:cursor-default"
                  aria-label={canSpeak ? `Play from ${HOSTS[turn.speaker]}: ${turn.text}` : undefined}
                >
                  <span className="mr-2 text-sm uppercase tracking-widest text-ink/50">{HOSTS[turn.speaker]}</span>
                  {turn.text}
                </button>
              </li>
            ))}
          </ol>

          <div className="mt-4 flex flex-wrap items-center gap-3 border-t border-dashed border-ink/40 pt-3 text-base">
            <label className="flex items-center gap-2">
              <span className="uppercase tracking-wider">Language</span>
              <select
                value={language}
                onChange={(e) => setLanguage(e.target.value)}
                className="rounded border-2 border-ink bg-canvas px-1"
              >
                <option value="">Same as the document</option>
                {LANGUAGES.map((l) => (
                  <option key={l} value={l}>
                    {l}
                  </option>
                ))}
              </select>
            </label>
            <button
              type="button"
              onClick={() => {
                stop();
                void load({ language, regenerate: true });
              }}
              disabled={loading}
              className="rounded-md border-2 border-ink px-4 py-1 uppercase hover:bg-ink hover:text-canvas disabled:opacity-40"
            >
              Write a new one
            </button>
          </div>
        </>
      )}
    </div>
  );
}
