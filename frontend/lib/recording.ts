// Recording a meeting in the browser. The helpers here are plain functions so they can be tested without a microphone.

// Speech is clear at 32 kbit/s, and that keeps a long meeting small: 90 minutes is about 22 MB, under the 25 MB
// a recording may be (see AUDIO limits in lib/links.ts and go-api).
export const RECORDING_BITS_PER_SECOND = 32_000;
export const RECORDING_MAX_MINUTES = 90;
// The recorder is stopped before the file gets near the largest size the server takes.
export const RECORDING_MAX_BYTES = 24 * 1024 * 1024;

// In order of preference. Chrome and Firefox record WebM, Safari records MP4 (audio only); all of them are formats
// the transcription service reads.
const PREFERRED_TYPES = ["audio/webm;codecs=opus", "audio/webm", "audio/mp4", "audio/ogg;codecs=opus"];

// The first type this browser can record, or null (then the browser's own default is used, or recording is not offered).
export function pickRecordingType(isSupported: (type: string) => boolean): string | null {
  return PREFERRED_TYPES.find((type) => isSupported(type)) ?? null;
}

export function extensionFor(mimeType: string): string {
  const type = mimeType.toLowerCase();
  if (type.includes("webm")) return ".webm";
  if (type.includes("mp4") || type.includes("aac") || type.includes("m4a")) return ".m4a";
  if (type.includes("ogg")) return ".ogg";
  if (type.includes("wav")) return ".wav";
  return ".webm";
}

const two = (n: number) => String(n).padStart(2, "0");

// "recording 2026-10-09 14.05.webm": no colons (not allowed in a file name on Windows), and sorted by date.
export function recordingFileName(mimeType: string, now: Date = new Date()): string {
  const day = `${now.getFullYear()}-${two(now.getMonth() + 1)}-${two(now.getDate())}`;
  return `recording ${day} ${two(now.getHours())}.${two(now.getMinutes())}${extensionFor(mimeType)}`;
}

// 0:07, 12:30, 1:02:05
export function formatClock(milliseconds: number): string {
  const total = Math.max(0, Math.floor(milliseconds / 1000));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  return hours > 0 ? `${hours}:${two(minutes)}:${two(seconds)}` : `${minutes}:${two(seconds)}`;
}

// Whether this browser can record at all: it needs a microphone API and MediaRecorder.
export function canRecordHere(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.MediaRecorder !== "undefined" &&
    typeof navigator !== "undefined" &&
    Boolean(navigator.mediaDevices?.getUserMedia)
  );
}
