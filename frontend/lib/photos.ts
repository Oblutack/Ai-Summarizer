// Photos of pages. A phone photo is often 4000 pixels wide and several megabytes, far more than reading the text needs
// (the server shrinks it anyway) and enough to be slow to send or to run into the size limit. So a big photo is made
// smaller in the browser, just before it is sent. The original file is never changed, and if anything goes wrong the
// original is sent as it is.

// 3000 pixels on the long side is about 250 dots per inch on an A4 page: sharp enough for small print.
export const PHOTO_MAX_SIDE = 3000;
export const PHOTO_SHRINK_ABOVE_BYTES = 3 * 1024 * 1024;
const JPEG_QUALITY = 0.88;

export function fitWithin(width: number, height: number, maxSide: number): { width: number; height: number } {
  const longest = Math.max(width, height);
  if (longest <= maxSide) return { width, height };
  const scale = maxSide / longest;
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) };
}

export function needsShrinking(width: number, height: number, bytes: number): boolean {
  return Math.max(width, height) > PHOTO_MAX_SIDE || bytes > PHOTO_SHRINK_ABOVE_BYTES;
}

// "IMG_0042.HEIC.png" -> "IMG_0042.HEIC.jpg"
export function jpegName(name: string): string {
  return /\.[^./]+$/.test(name) ? name.replace(/\.[^./]+$/, ".jpg") : `${name}.jpg`;
}

// The photo, made smaller when it is big. Anything that is not a picture the browser can open comes back unchanged.
export async function shrinkPhoto(file: File): Promise<File> {
  try {
    if (typeof createImageBitmap !== "function") return file;
    const bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
    try {
      if (!needsShrinking(bitmap.width, bitmap.height, file.size)) return file;
      const { width, height } = fitWithin(bitmap.width, bitmap.height, PHOTO_MAX_SIDE);
      const canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      const context = canvas.getContext("2d");
      if (!context) return file;
      // A see-through background (a PNG) would turn black as a JPEG: it is paper, so it is white.
      context.fillStyle = "#ffffff";
      context.fillRect(0, 0, width, height);
      context.drawImage(bitmap, 0, 0, width, height);
      const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/jpeg", JPEG_QUALITY));
      if (!blob || blob.size >= file.size) return file;
      return new File([blob], jpegName(file.name), { type: "image/jpeg", lastModified: file.lastModified });
    } finally {
      bitmap.close();
    }
  } catch {
    return file;
  }
}
