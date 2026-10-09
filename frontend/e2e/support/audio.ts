// A real, playable sound for tests: a quiet tone as a 16-bit mono WAV file the browser can decode and play.
export function wavFile(name: string, seconds: number, sampleRate = 8000) {
  const samples = Math.round(seconds * sampleRate);
  const data = Buffer.alloc(samples * 2);
  for (let i = 0; i < samples; i++) data.writeInt16LE(Math.round(Math.sin((2 * Math.PI * 440 * i) / sampleRate) * 3000), i * 2);
  const header = Buffer.alloc(44);
  header.write("RIFF", 0);
  header.writeUInt32LE(36 + data.length, 4);
  header.write("WAVE", 8);
  header.write("fmt ", 12);
  header.writeUInt32LE(16, 16); // size of this part
  header.writeUInt16LE(1, 20); // plain PCM
  header.writeUInt16LE(1, 22); // one channel
  header.writeUInt32LE(sampleRate, 24);
  header.writeUInt32LE(sampleRate * 2, 28); // bytes a second
  header.writeUInt16LE(2, 32); // bytes a sample
  header.writeUInt16LE(16, 34); // bits a sample
  header.write("data", 36);
  header.writeUInt32LE(data.length, 40);
  return { name, mimeType: "audio/wav", buffer: Buffer.concat([header, data]) };
}
