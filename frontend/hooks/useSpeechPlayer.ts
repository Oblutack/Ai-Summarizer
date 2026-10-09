"use client";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { pickVoices, splitForSpeech } from "../lib/podcast";
import {
  DEFAULT_PLAYBACK,
  clampRate,
  loadPlayback,
  savePlayback,
  secondsLeft,
  type PlaybackSettings,
  type VoiceRole,
} from "../lib/playback";

// A piece of speech that can be jumped to: a paragraph of a summary, or one line of a podcast.
export interface SpeechItem {
  text: string;
  // Which voice reads it: the single reader, or one of the two hosts.
  role: VoiceRole;
}

export type PlayStatus = "idle" | "playing" | "paused";

// Reads a list of items aloud with the browser's own voices (nothing is sent anywhere), and lets the listener
// pause, skip back and forward, jump to any item, change the speed or a voice while it plays. The speed and the
// voices are remembered on this device.
export function useSpeechPlayer(items: SpeechItem[], wantedLanguage: string) {
  const synth = typeof window !== "undefined" ? window.speechSynthesis : undefined;
  const supported = !!synth && typeof window !== "undefined" && "SpeechSynthesisUtterance" in window;

  const [voices, setVoices] = useState<SpeechSynthesisVoice[]>([]);
  const [settings, setSettings] = useState<PlaybackSettings>(DEFAULT_PLAYBACK);
  const [status, setStatus] = useState<PlayStatus>("idle");
  const [index, setIndex] = useState(-1);
  const [failed, setFailed] = useState(false);

  // Bumped to make the callbacks of an earlier playback do nothing once it has been replaced or stopped.
  const run = useRef(0);
  const position = useRef({ item: 0, chunk: 0 });
  const chunkLists = useMemo(() => items.map((item) => splitForSpeech(item.text)), [items]);
  const latest = useRef({ items, chunkLists, voices, settings });
  latest.current = { items, chunkLists, voices, settings };

  // What was chosen last time is read after the first render, so the page and the first client render agree.
  useEffect(() => setSettings(loadPlayback()), []);

  // The browser's voices arrive asynchronously in some browsers.
  useEffect(() => {
    if (!synth) return;
    const read = () => setVoices(synth.getVoices());
    read();
    synth.addEventListener?.("voiceschanged", read);
    return () => synth.removeEventListener?.("voiceschanged", read);
  }, [synth]);

  // The voice for each role: the one chosen (if this browser still has it), else the best for the language.
  const chosen = useMemo(() => {
    const [first, second] = pickVoices(voices, wantedLanguage || "en");
    const defaults: Record<VoiceRole, SpeechSynthesisVoice | undefined> = {
      read: first as SpeechSynthesisVoice | undefined,
      A: first as SpeechSynthesisVoice | undefined,
      B: second as SpeechSynthesisVoice | undefined,
    };
    const pick = (role: VoiceRole) => voices.find((v) => v.voiceURI === settings.voices[role]) ?? defaults[role];
    return { read: pick("read"), A: pick("A"), B: pick("B") };
  }, [voices, settings.voices, wantedLanguage]);
  const chosenRef = useRef(chosen);
  chosenRef.current = chosen;

  const speakChunk = useCallback(
    (id: number, item: number, chunk: number) => {
      if (!synth || run.current !== id) return;
      const { items: list, chunkLists: chunks, settings: current } = latest.current;
      if (item >= list.length) {
        setStatus("idle");
        setIndex(-1);
        return;
      }
      if (chunk >= chunks[item].length) {
        speakChunk(id, item + 1, 0); // an item with nothing to say, or the end of one
        return;
      }
      position.current = { item, chunk };
      setIndex(item);
      const role = list[item].role;
      const utterance = new SpeechSynthesisUtterance(chunks[item][chunk]);
      const voice = chosenRef.current[role];
      if (voice) utterance.voice = voice;
      utterance.rate = current.rate;
      // With a single voice installed, a lower pitch is what tells the second host apart.
      if (role === "B" && chosenRef.current.A && chosenRef.current.A === chosenRef.current.B) utterance.pitch = 0.8;
      utterance.onend = () => speakChunk(id, item, chunk + 1);
      utterance.onerror = (event: SpeechSynthesisErrorEvent) => {
        if (run.current !== id || event.error === "interrupted" || event.error === "canceled") return;
        run.current++;
        setStatus("idle");
        setIndex(-1);
        setFailed(true);
      };
      synth.speak(utterance);
    },
    [synth]
  );

  const play = useCallback(
    (from = 0) => {
      if (!synth || items.length === 0) return;
      synth.cancel();
      const id = ++run.current;
      setFailed(false);
      setStatus("playing");
      speakChunk(id, Math.min(Math.max(0, from), items.length - 1), 0);
    },
    [synth, items.length, speakChunk]
  );

  const stop = useCallback(() => {
    run.current++;
    synth?.cancel();
    setStatus("idle");
    setIndex(-1);
  }, [synth]);

  const pause = useCallback(() => {
    synth?.pause();
    setStatus("paused");
  }, [synth]);

  const resume = useCallback(() => {
    synth?.resume();
    setStatus("playing");
  }, [synth]);

  const next = useCallback(() => {
    const target = position.current.item + 1;
    if (target >= latest.current.items.length) stop();
    else play(target);
  }, [play, stop]);

  // Back goes to the start of the item being read, or to the one before it if that has only just begun.
  const previous = useCallback(() => {
    const { item, chunk } = position.current;
    play(chunk > 0 ? item : Math.max(0, item - 1));
  }, [play]);

  // Plays the current piece again, so a changed speed or voice is heard at once.
  const restartHere = useCallback(() => {
    if (!synth) return;
    const { item, chunk } = position.current;
    synth.cancel();
    const id = ++run.current;
    speakChunk(id, item, chunk);
  }, [synth, speakChunk]);

  const update = (change: (current: PlaybackSettings) => PlaybackSettings) => {
    setSettings((current) => {
      const updated = change(current);
      latest.current.settings = updated;
      savePlayback(updated);
      return updated;
    });
  };

  const setRate = (rate: number) => {
    update((current) => ({ ...current, rate: clampRate(rate) }));
    if (status === "playing") restartHere();
  };

  const setVoice = (role: VoiceRole, voiceURI: string) => {
    update((current) => ({ ...current, voices: { ...current.voices, [role]: voiceURI } }));
    // chosenRef is refreshed on the next render, so the restart waits for it.
    if (status === "playing") setTimeout(restartHere, 0);
  };

  // Stop talking when the player goes away.
  useEffect(() => () => stop(), [stop]);

  const remaining = secondsLeft(items, Math.max(index, 0), settings.rate);

  return {
    supported,
    canSpeak: supported && voices.length > 0,
    voices,
    voiceOf: (role: VoiceRole) => chosen[role]?.voiceURI ?? "",
    rate: settings.rate,
    status,
    // The item being spoken, or -1 when nothing is.
    index,
    failed,
    remaining,
    play,
    pause,
    resume,
    stop,
    next,
    previous,
    seek: play,
    setRate,
    setVoice,
  };
}
