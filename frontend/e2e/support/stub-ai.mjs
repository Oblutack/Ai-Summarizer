// A stand-in for the Python AI service, so the end-to-end tests are fast, free and deterministic.
// It speaks the same HTTP contract the Go gateway expects (see python-ai-service/main.py):
//   GET  /healthz
//   POST /summarize-text[?stream=true]   JSON {text}
//   POST /summarize | /summarize-multiple[?stream=true]   multipart file(s)
//   POST /chat                           JSON {text, question, history}
// Magic inputs: text containing FAIL_ME makes the "model" fail; SLOW_ME makes it write slowly.
import http from "node:http";

const PORT = Number(process.env.STUB_AI_PORT ?? 18081);
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => resolve(Buffer.concat(chunks)));
    req.on("error", reject);
  });
}

// Filenames of the uploaded parts; the stub does not read the PDFs themselves.
function uploadedNames(body) {
  return [...body.toString("latin1").matchAll(/filename="([^"]+)"/g)].map((m) => m[1]);
}

const words = (text) => text.split(/\s+/).filter(Boolean);
const summaryOf = (subject, source) =>
  `## Stub summary\n\n- Subject: ${subject}\n- Opening words: ${words(source).slice(0, 8).join(" ")}\n- Words in source: ${words(source).length}`;

function json(res, status, payload) {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(JSON.stringify(payload));
}

async function streamSummary(res, summary, doneExtra, { slow, fail }) {
  res.writeHead(200, { "Content-Type": "text/event-stream; charset=utf-8", "Cache-Control": "no-cache" });
  const send = (event) => res.write(`data: ${JSON.stringify(event)}\n\n`);
  send({ type: "status", stage: "preparing" });
  send({ type: "status", stage: "summarizing", done: 1, total: 2 });
  await sleep(40);
  send({ type: "status", stage: "summarizing", done: 2, total: 2 });
  if (fail) {
    send({ type: "error", status: 502, message: "The language model is unavailable. Please try again." });
    return res.end();
  }
  send({ type: "status", stage: "writing" });
  for (const piece of summary.match(/\S+\s*/g) ?? []) {
    await sleep(slow ? 500 : 15);
    if (res.destroyed || res.writableEnded) return;
    send({ type: "delta", text: piece });
  }
  send({ type: "done", ...doneExtra });
  res.end();
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, "http://stub");
  const stream = url.searchParams.get("stream") === "true";

  if (req.method === "GET" && url.pathname === "/healthz") return json(res, 200, { status: "ok" });
  if (req.method !== "POST") return json(res, 404, { detail: "not found" });

  const body = await readBody(req);

  if (url.pathname === "/summarize-text") {
    const { text = "" } = JSON.parse(body.toString("utf8") || "{}");
    const flags = { slow: text.includes("SLOW_ME"), fail: text.includes("FAIL_ME") };
    if (stream) return streamSummary(res, summaryOf("pasted text", text), { text }, flags);
    if (flags.fail) return json(res, 502, { detail: "The language model is unavailable. Please try again." });
    return json(res, 200, { summary: summaryOf("pasted text", text), text });
  }

  if (url.pathname === "/summarize" || url.pathname === "/summarize-multiple") {
    const names = uploadedNames(body);
    const filename = names.join(", ");
    const text = `(stub) extracted text of ${filename}`;
    const summary = summaryOf(filename, text);
    if (stream) return streamSummary(res, summary, { filename, text }, {});
    return json(res, 200, { filename, summary, text });
  }

  if (url.pathname === "/chat") {
    const { question = "", text = "" } = JSON.parse(body.toString("utf8") || "{}");
    const answer = `Stub answer to "${question}" (the document has ${words(text).length} words)`;
    // Magic questions: NOCITE gets an answer with no citations; INVENTED also uses a marker that
    // matches no source (the real service strips those; the UI must still never link them).
    if (question.includes("NOCITE")) return json(res, 200, { answer: `${answer}.`, sources: [] });
    const invented = question.includes("INVENTED") ? " and [9]" : "";
    return json(res, 200, {
      answer: `${answer} [1]${invented}.`,
      sources: [
        { id: 1, text: "Stub passage that the answer cites.", page: 2, pageEnd: 2, document: null },
      ],
    });
  }

  return json(res, 404, { detail: "not found" });
});

server.listen(PORT, "127.0.0.1", () => console.log(`stub AI service on :${PORT}`));
process.on("SIGTERM", () => server.close(() => process.exit(0)));
