import { describe, expect, it } from "vitest";
import { isDocumentFile, webLinkIn } from "./links";

describe("webLinkIn", () => {
  it("finds a web link, however much blank space surrounds it", () => {
    expect(webLinkIn("https://example.com/post")).toBe("https://example.com/post");
    expect(webLinkIn("  http://news.example/a?b=1#c \n")).toBe("http://news.example/a?b=1#c");
    expect(webLinkIn("HTTPS://Example.com")).toBe("HTTPS://Example.com");
  });

  it("takes an address that starts with www", () => {
    expect(webLinkIn("www.example.com/page")).toBe("https://www.example.com/page");
  });

  it("is not fooled by text, file names or half a link", () => {
    for (const text of [
      "",
      "   ",
      "Read https://example.com/post for details",
      "https://example.com and https://example.org",
      "report.pdf",
      "example.com",
      "notes.txt",
      "ftp://example.com/file",
      "javascript:alert(1)",
      "https://",
      "https://localhost",
      "www.",
      "A sentence. With dots. Everywhere.",
    ]) {
      expect(webLinkIn(text), text).toBeNull();
    }
  });

  it("refuses an address that is absurdly long", () => {
    expect(webLinkIn("https://example.com/" + "a".repeat(2100))).toBeNull();
  });
});

describe("isDocumentFile", () => {
  it("accepts PDF, Word and PowerPoint files in any letter case", () => {
    for (const name of ["a.pdf", "B.PDF", "lease.docx", "Deck.PPTX", "my.file.name.docx"]) {
      expect(isDocumentFile(name), name).toBe(true);
    }
  });

  it("refuses everything else", () => {
    for (const name of ["a.txt", "old.doc", "old.ppt", "sheet.xlsx", "pdf", "docx", "a.pdf.exe", ""]) {
      expect(isDocumentFile(name), name).toBe(false);
    }
  });
});

describe("recordings", () => {
  it("are told apart from documents by their extension, in any letter case", async () => {
    const { isAudioFile } = await import("./links");
    for (const name of ["sync.mp3", "Call.M4A", "voice.wav", "talk.ogg", "memo.flac", "clip.webm", "screen.mp4", "a.mpeg", "b.mpga"]) {
      expect(isAudioFile(name), name).toBe(true);
      expect(isDocumentFile(name), name).toBe(true);
    }
    for (const name of ["report.pdf", "notes.docx", "deck.pptx", "mp3", "song.mp3.exe", "voice.aac", ""]) {
      expect(isAudioFile(name), name).toBe(false);
    }
  });
});
