import { deflateSync } from "node:zlib";

// Real pictures for tests, made without any library: an 8-bit grey PNG that the browser can open and draw.
// With `noise` the pixels are not all the same, so the picture does not squeeze down to nothing (like a real photo).
const CRC_TABLE = Array.from({ length: 256 }, (_, n) => {
  let c = n;
  for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  return c >>> 0;
});

function crc32(bytes: Buffer): number {
  let c = 0xffffffff;
  for (const byte of bytes) c = CRC_TABLE[(c ^ byte) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

function chunk(type: string, data: Buffer): Buffer {
  const head = Buffer.alloc(8);
  head.writeUInt32BE(data.length, 0);
  head.write(type, 4, "ascii");
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(Buffer.concat([head.subarray(4), data])), 0);
  return Buffer.concat([head, data, crc]);
}

export function pngFile(name: string, width: number, height: number, noise = 0) {
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header[8] = 8; // bits a sample
  header[9] = 0; // grey
  const rows = Buffer.alloc((width + 1) * height); // each row starts with its filter type: 0, none
  let seed = 12345;
  for (let y = 0; y < height; y++) {
    const start = y * (width + 1) + 1;
    for (let x = 0; x < width; x++) {
      seed = (seed * 1103515245 + 12345) & 0x7fffffff;
      rows[start + x] = (Math.round((x / width) * 200) + 20 + (noise ? seed % noise : 0)) & 0xff;
    }
  }
  const png = Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk("IHDR", header),
    chunk("IDAT", deflateSync(rows, { level: 1 })),
    chunk("IEND", Buffer.alloc(0)),
  ]);
  return { name, mimeType: "image/png", buffer: png };
}

// A file with the name and type of a JPEG photo. The stub AI service never opens it, and the browser only reshapes
// photos that are big, so these small stand-ins reach the service as they are.
export function jpegFile(name: string, bytes = 2048) {
  return { name, mimeType: "image/jpeg", buffer: Buffer.alloc(bytes, 7) };
}
