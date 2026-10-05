import { describe, expect, it } from "vitest";
import { parseSseBlock, readSummaryEvents, type SummaryEvent } from "./sse";

// A fetch Response whose body arrives in the given pieces, like a real network stream.
function responseOf(...chunks: string[]): Response {
  const encoder = new TextEncoder();
  return new Response(
    new ReadableStream({
      start(controller) {
        for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
        controller.close();
      },
    })
  );
}

async function collect(response: Response): Promise<SummaryEvent[]> {
  const events: SummaryEvent[] = [];
  await readSummaryEvents(response, (e) => events.push(e));
  return events;
}

describe("parseSseBlock", () => {
  it("parses a data line", () => {
    expect(parseSseBlock('data: {"type":"delta","text":"hi"}')).toEqual({ type: "delta", text: "hi" });
  });

  it("ignores comments and keep-alives", () => {
    expect(parseSseBlock(": keep-alive")).toBeNull();
    expect(parseSseBlock("")).toBeNull();
  });

  it("ignores malformed JSON and objects without a type", () => {
    expect(parseSseBlock("data: {not json")).toBeNull();
    expect(parseSseBlock('data: {"text":"no type"}')).toBeNull();
    expect(parseSseBlock("data: null")).toBeNull();
  });

  it("finds the data line among other fields", () => {
    expect(parseSseBlock('event: message\ndata: {"type":"done"}')).toEqual({ type: "done" });
  });
});

describe("readSummaryEvents", () => {
  it("delivers events in order", async () => {
    const events = await collect(
      responseOf(
        'data: {"type":"status","stage":"preparing"}\n\n',
        'data: {"type":"delta","text":"Hello"}\n\n',
        'data: {"type":"done","filename":"a.pdf"}\n\n'
      )
    );
    expect(events).toEqual([
      { type: "status", stage: "preparing" },
      { type: "delta", text: "Hello" },
      { type: "done", filename: "a.pdf" },
    ]);
  });

  it("reassembles an event split across chunks", async () => {
    const events = await collect(responseOf('data: {"type":"del', 'ta","text":"split"}\n', "\n"));
    expect(events).toEqual([{ type: "delta", text: "split" }]);
  });

  it("handles several events in one chunk", async () => {
    const events = await collect(
      responseOf('data: {"type":"delta","text":"a"}\n\ndata: {"type":"delta","text":"b"}\n\n')
    );
    expect(events.map((e) => (e.type === "delta" ? e.text : ""))).toEqual(["a", "b"]);
  });

  it("does not split multi-byte characters that straddle chunks", async () => {
    const bytes = new TextEncoder().encode('data: {"type":"delta","text":"héllo ✓"}\n\n');
    const cut = bytes.indexOf(0xc3) + 1; // between the two bytes of "é"
    const response = new Response(
      new ReadableStream({
        start(controller) {
          controller.enqueue(bytes.slice(0, cut));
          controller.enqueue(bytes.slice(cut));
          controller.close();
        },
      })
    );
    expect(await collect(response)).toEqual([{ type: "delta", text: "héllo ✓" }]);
  });

  it("delivers a final event that has no trailing blank line", async () => {
    expect(await collect(responseOf('data: {"type":"done"}'))).toEqual([{ type: "done" }]);
  });

  it("skips garbage between events", async () => {
    const events = await collect(responseOf(': ping\n\ndata: {"type":"done"}\n\n'));
    expect(events).toEqual([{ type: "done" }]);
  });

  it("fails clearly when there is no body", async () => {
    await expect(readSummaryEvents(new Response(null), () => {})).rejects.toThrow(/empty response/);
  });
});
