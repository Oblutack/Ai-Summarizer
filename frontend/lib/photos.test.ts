import { describe, expect, it } from "vitest";
import { fitWithin, jpegName, needsShrinking, PHOTO_MAX_SIDE, PHOTO_SHRINK_ABOVE_BYTES, shrinkPhoto } from "./photos";

describe("fitWithin", () => {
  it("never makes a picture bigger", () => {
    expect(fitWithin(800, 600, 3000)).toEqual({ width: 800, height: 600 });
    expect(fitWithin(3000, 2000, 3000)).toEqual({ width: 3000, height: 2000 });
  });

  it("brings the long side down to the limit and keeps the shape", () => {
    expect(fitWithin(4032, 3024, 3000)).toEqual({ width: 3000, height: 2250 });
    expect(fitWithin(3024, 4032, 3000)).toEqual({ width: 2250, height: 3000 });
    expect(fitWithin(12000, 100, 3000)).toEqual({ width: 3000, height: 25 });
  });

  it("never rounds a side down to nothing", () => {
    expect(fitWithin(30000, 1, 3000)).toEqual({ width: 3000, height: 1 });
  });
});

describe("needsShrinking", () => {
  it("is for pictures that are big in pixels or in bytes", () => {
    expect(needsShrinking(PHOTO_MAX_SIDE + 1, 100, 1000)).toBe(true);
    expect(needsShrinking(100, PHOTO_MAX_SIDE + 1, 1000)).toBe(true);
    expect(needsShrinking(1000, 1000, PHOTO_SHRINK_ABOVE_BYTES + 1)).toBe(true);
  });

  it("leaves a picture that is already small enough alone", () => {
    expect(needsShrinking(PHOTO_MAX_SIDE, PHOTO_MAX_SIDE, PHOTO_SHRINK_ABOVE_BYTES)).toBe(false);
    expect(needsShrinking(1200, 1600, 400_000)).toBe(false);
  });
});

describe("jpegName", () => {
  it("swaps the extension", () => {
    expect(jpegName("IMG_0042.PNG")).toBe("IMG_0042.jpg");
    expect(jpegName("my.page.final.webp")).toBe("my.page.final.jpg");
    expect(jpegName("photo")).toBe("photo.jpg");
    expect(jpegName("folder.v2/photo")).toBe("folder.v2/photo.jpg");
  });
});

describe("shrinkPhoto", () => {
  it("sends the original when the file is not a picture the browser can open", async () => {
    const notAPicture = new File(["not a picture"], "page.jpg", { type: "image/jpeg" });
    expect(await shrinkPhoto(notAPicture)).toBe(notAPicture);
  });
});
