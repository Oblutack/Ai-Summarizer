// Events sent by the summarize endpoints when called with ?stream=true.
export type SummaryEvent =
  | { type: "status"; stage: "preparing" | "summarizing" | "writing"; done?: number; total?: number }
  | { type: "delta"; text: string }
  | { type: "done"; filename?: string }
  | { type: "error"; status?: number; message: string };

// Parses one server-sent event block ("data: {...}"). Returns null for blocks we don't understand,
// such as comments or keep-alives.
export function parseSseBlock(block: string): SummaryEvent | null {
  const line = block.split("\n").find((l) => l.startsWith("data: "));
  if (!line) return null;
  try {
    const event = JSON.parse(line.slice("data: ".length));
    return event && typeof event.type === "string" ? (event as SummaryEvent) : null;
  } catch {
    return null;
  }
}

// Reads a fetch response body as server-sent events, calling onEvent for each one as it arrives.
export async function readSummaryEvents(
  response: Response,
  onEvent: (event: SummaryEvent) => void
): Promise<void> {
  if (!response.body) throw new Error("The server sent an empty response.");

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    // Events are separated by a blank line; a chunk can hold several or only part of one.
    let boundary: number;
    while ((boundary = buffer.indexOf("\n\n")) !== -1) {
      const event = parseSseBlock(buffer.slice(0, boundary));
      buffer = buffer.slice(boundary + 2);
      if (event) onEvent(event);
    }
  }

  const rest = parseSseBlock(buffer);
  if (rest) onEvent(rest);
}
