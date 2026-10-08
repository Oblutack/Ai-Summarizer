<div align="center">

# Inkling

**Read your documents with a second pair of eyes: summaries, answers that cite the exact page, and a check on whether to trust them.**

Inkling turns PDFs, Word and PowerPoint files, web pages and text into streaming summaries, answers questions about one document or your whole library with page-exact citations, checks its own summaries against the original, and can even turn a document into a podcast. A full-stack, three-service application with an e-ink inspired interface: a Next.js frontend, a Go API gateway, and a Python AI service, backed by PostgreSQL.

[![CI](https://github.com/Oblutack/Inkling/actions/workflows/ci.yml/badge.svg)](https://github.com/Oblutack/Inkling/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)
![Next.js](https://img.shields.io/badge/Next.js-15-000000?logo=next.js&logoColor=white)
![Python](https://img.shields.io/badge/Python-3.11-3776AB?logo=python&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-4169E1?logo=postgresql&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)

[**Live demo**](https://ai-summarizer-ten-tan.vercel.app) &nbsp;·&nbsp; [Features](#features) &nbsp;·&nbsp; [Quick start](#quick-start) &nbsp;·&nbsp; [Architecture](#architecture) &nbsp;·&nbsp; [API](#api-reference) &nbsp;·&nbsp; [Configuration](#configuration) &nbsp;·&nbsp; [Testing](#testing)

<br>

<img src="docs/demo.gif" alt="Demo of Inkling: the landing page, a summary streaming in Spanish, sign-up and login, a PDF summary with live progress, the proof check, the cited page opened in the original PDF, chat with suggested questions, a question across all documents, a podcast, the account page, and the dark theme and language switch" width="900">

<sub>A tour of Inkling: the landing page, a streamed summary in another language, sign-up, a PDF summary with live progress, the check that marks every sentence against the original, the cited page opened with the passage highlighted, chat with citations, one question across all documents, a podcast, the account page, and the dark theme in another language.</sub>

</div>

---

## Features

### Summarization
- **PDFs, Word, PowerPoint, web links or pasted text.** Drop in up to **5 files** (PDF, `.docx` or `.pptx`, 10 MB each, mixed freely) and get one combined summary that calls out overlaps and differences between them, or paste a web address and Inkling reads the page (articles, documentation, even a link straight to a PDF). Slides count as pages, so answers can cite a slide.
- **Live streaming.** Words appear as the model writes them, with real progress ("Summarized 3 of 8 sections") for long documents, and a Cancel button that actually stops the work.
- **Five styles and 15 languages.** Standard, bullet points, executive brief, explain-it-simply, or takeaways with action items, in any of 15 output languages regardless of the source language.
- **Length control.** A word-count slider for short summaries, or a page limit for long documents.
- **Handles long documents.** Anything too big for one prompt goes through a map-reduce pipeline: sections are summarized in parallel (with bounded concurrency), then combined.
- **Instant repeats.** Identical requests (same text, options, model and prompt version) are answered from an in-memory cache.

### Chat with your documents
- Ask questions about any saved summary. The original text is kept server-side, and for long documents a built-in BM25 search picks only the relevant passages for the model, so answers stay grounded and cheap.
- **Listen as a podcast.** Any saved document can become a short conversation between two hosts, a curious one and one who explains, written from the document's own content (the prompt forbids adding facts, and every figure is kept exact). The script is written once, stored, and replayed for free; the browser reads it aloud with two of its own voices, highlighting the current line, with pause, speed, voice choice, jump-to-any-line and a Markdown download. Voices are the browser's built-in ones (free, but they vary in quality); the script is kept server-side, so a real text-to-speech service could replace them without changing anything else.
- **Ask all your documents at once.** One question is searched across everything you have saved, and the answer cites the document and page each statement came from (opening the original when it was kept). Every document is cut into page-sized passages and indexed when it is saved (older ones the first time you ask); the best passages are found by **meaning** and by keyword together, and the model answers only from them, so nothing is sent that did not match. Meaning comes from a small embedding model that runs inside the AI service (no extra paid service, nothing leaves your machine), so "Can I keep a pet?" finds a passage that only says "cats and dogs"; keyword search (Postgres full-text, with stemming) still rescues exact terms and takes over by itself if the embedding model is off. On a 58-question test set (`python-ai-service/evals/search`, which you can rerun) the right passage was in the top three for 98% of questions with page-sized passages, against 83% for keyword search alone, and 90% against 62% with paragraph-sized passages. Users only ever search their own documents.
- **Collections.** Tags group your documents, and a group can be asked as a whole: pick a tag in the *Search in* box (or filter the list by it) and the question is searched, indexed and answered from just those documents, so the course notes are not mixed up with the lease. Tags are the collections: there is no second kind of folder to manage. For a collection of two or more documents, **Write an overview** produces one briefing on the whole group: what it is about, where the documents agree, where they differ and what only one of them mentions, with the documents named behind every point. It is written from the saved summaries (so it is quick and cheap), streams in like any summary, can be copied, and is not stored; it can only be as accurate as the summaries it starts from.
- **Check a summary against the original.** One click marks every sentence of a saved summary as found, partly found or not found in the document, and opening a sentence shows the passage (and page) that backs it. Numbers are checked on their own, since a wrong figure is the classic summarizer mistake: a number the document never mentions is called out, and so is one that exists elsewhere but not beside the matching wording. It is plain text matching, so it costs no model call, is instant and gives the same answer every time; it compares wording, not meaning, so "partly found" means look closer, not wrong.
- **Open the original.** When the PDF was kept, a citation can open it at the cited page in a built-in viewer with the passage highlighted. Originals are stored per user (capped, `STORED_FILES_MB_PER_USER`), only ever served to their owner, and deleted with the document or the account.
- **Answers cite their sources.** Each statement carries a small numbered marker; click it to see the exact passage and the page it came from (and the file, when several PDFs were combined). Markers are checked on the server, so one can never point at a passage that does not exist, and answers that cannot be found in the document say so instead of citing.
- Conversation history is supported, and the model is told to say so when the answer is not in the document.
- **Suggested questions and highlight-to-ask.** Opening the chat offers questions worth asking about that document (written once, kept), and selecting a few words of a summary offers to ask what they mean.

### Working with your summaries
- **Search, rename and tag.** Find saved summaries by title or text, rename them, and put up to eight tags on each to filter by.
- **Write it again.** Re-summarize a saved document in another style, length (TL;DR, short, one page, detailed) or language, from the text kept with it.
- **Study it.** Flashcards and a multiple-choice quiz made from the document (written once and kept, so opening them again is free and works even after the daily allowance is used up).
- **Share, send and export.** A public read-only link to the summary (never the source text), revocable at any time; email a summary to yourself; save it as PDF, Markdown or a Word document; copy it; or have your browser read it aloud.
- **Standing instructions.** Tell Inkling once, on the account page, how you like your summaries ("focus on costs and deadlines") and every summary follows.

### The interface
- **Light and dark themes** (following your system until you choose), and the interface in **English, Spanish, German, French and Bosnian** (chosen from your browser's language, or from the switcher; summaries can be written in 15 languages whatever the interface language).
- **Installable.** On a phone or computer, "Install" puts Inkling on your home screen or desktop; a small service worker shows a friendly page when you are offline and never stores your documents.

### Accounts and privacy
- Email/password (bcrypt) and **Google sign-in**, with email confirmation and password reset.
- **Revocable server-side sessions** in `httpOnly` cookies; a device list with remote sign-out; "sign out everywhere".
- **Your data is yours:** one-click JSON export and permanent account deletion (hard delete, re-authenticated).
- Saved history with cursor pagination, PDF export, and daily usage meters.

### Built for production
- Per-user daily quotas (with refunds when work fails), per-IP and per-user rate limits, a per-account login throttle, and optional **Cloudflare Turnstile** bot protection.
- A circuit breaker and automatic model fallback so a provider outage or a retired model degrades gracefully instead of hanging.
- Structured JSON logs with a request ID that follows each request from the browser through the Go API into the AI service.
- Health and readiness endpoints, Docker health checks, non-root images, SQL migrations, and CI that runs every test suite against a real Postgres.
- **Observability:** Prometheus metrics from both backends (traffic, latency, model outcomes, token usage, cache and circuit-breaker state) with a ready-made Grafana dashboard, and opt-in Sentry error reporting that never sends documents or request data. See [Observability](#observability).

---

## Quick start

**Prerequisites:** [Docker](https://www.docker.com/), [Node.js 20+](https://nodejs.org/) with [pnpm](https://pnpm.io/), and a free [Groq API key](https://console.groq.com/keys).

```bash
# 1. Clone
git clone https://github.com/Oblutack/Inkling.git
cd Inkling

# 2. Configure the backend (Postgres password, Groq key, Google client ID)
cp .env.example .env
#    edit .env: set POSTGRES_PASSWORD, GROQ_API_KEY and GOOGLE_CLIENT_ID

# 3. Start Postgres, the AI service and the Go API
docker compose up --build

# 4. Start the frontend (in a second terminal)
cd frontend
printf 'NEXT_PUBLIC_API_URL=http://localhost:8080\nNEXT_PUBLIC_GOOGLE_CLIENT_ID=<your-client-id>\n' > .env.local
pnpm install
pnpm dev
```

Open **http://localhost:3000**. Compose waits for each service's health check, so the first start takes a minute while images build.

<details>
<summary><b>Notes for first-time setup</b></summary>

- **Google client ID.** Compose requires `GOOGLE_CLIENT_ID` to be set. To try the app without Google sign-in, any placeholder value works (the Google button will simply not complete); to enable it, create an OAuth client at [Google Cloud Console](https://console.cloud.google.com/apis/credentials) with `http://localhost:3000` as an authorized JavaScript origin.
- **Email links.** With the default `MAIL_PROVIDER=log`, confirmation and reset emails are not sent: their links are printed to the API log (`docker logs go-api`). See [Email](#email) to send real mail.
- **Existing database volume.** If you previously ran an older version and `go-api` cannot authenticate, your Postgres volume still has the old password. Either set `POSTGRES_PASSWORD` to the old value or run `docker compose down -v` to start fresh (this deletes local data).

</details>

---

## Architecture

```mermaid
flowchart LR
    B([Browser])
    subgraph FE [Frontend: Next.js 15]
        N[App Router<br/>nonce CSP middleware]
    end
    subgraph API [API gateway: Go + Gin]
        G[Auth, sessions, quotas<br/>rate limits, CSRF, Turnstile]
    end
    subgraph AI [AI service: FastAPI]
        P[Text extraction (PDF, Word, PowerPoint, web pages)<br/>map-reduce, BM25 retrieval<br/>cache, circuit breaker, fallback]
    end
    DB[(PostgreSQL 15)]
    L{{Groq LLM}}

    B -->|pages| N
    B -->|"JSON and SSE, httpOnly cookie"| G
    G -->|SQL, migrations| DB
    G -->|"summarize, chat, request id"| P
    P -->|"OpenAI-compatible API"| L
```

The browser only ever talks to the Go gateway. It owns identity and persistence and treats the AI service as an internal, stateless dependency, so the model provider can change without touching accounts or data.

<details>
<summary><b>How a streamed summary flows</b></summary>

```mermaid
sequenceDiagram
    actor U as User
    participant G as Go API
    participant A as AI service
    participant L as Groq
    participant D as Postgres

    U->>G: POST /summarize?stream=true (cookie, PDFs)
    G->>G: session, rate limit, quota claim, validate
    G->>A: forward request + X-Request-ID
    A->>A: extract text, cache lookup
    loop each section (bounded concurrency)
        A->>L: summarize section
        A-->>G: status: summarized n of N
        G-->>U: status event
    end
    A->>L: write final summary (stream)
    L-->>A: tokens
    A-->>G: delta events
    G-->>U: delta events
    A-->>G: done (+ source text)
    G->>D: save summary and source text
    G-->>U: done (source text never sent)
```

Failures before the first byte are ordinary JSON errors; once streaming has started they arrive as a final `error` event. The quota is refunded whenever the user did not receive a complete summary.

</details>

### Services

| Service | Stack | Responsibility |
| --- | --- | --- |
| [`frontend/`](frontend) | Next.js 15, TypeScript, Tailwind CSS, Framer Motion | UI, streaming reader, account pages, per-request CSP |
| [`go-api/`](go-api) | Go 1.27, Gin, GORM, pgx, golang-migrate | Auth and sessions, quotas, rate limits, proxying and streaming, persistence |
| [`python-ai-service/`](python-ai-service) | FastAPI, LangChain (OpenAI client and text splitter), pypdf, httpx | PDF, Word, PowerPoint and web page text extraction, summarization pipeline, chat retrieval, LLM client |
| PostgreSQL | Postgres 15 | Users, sessions, documents, usage, email tokens |
| LLM | Groq (OpenAI-compatible), default `openai/gpt-oss-20b` | Language model, configurable with automatic fallback |

### Design decisions
- **A gateway in front of the AI service.** Authentication, quotas and persistence live in one place; the AI service stays stateless and horizontally scalable.
- **Server-side sessions instead of JWTs.** Logout, password change and account deletion take effect immediately because the server can revoke a session. Only a SHA-256 hash of each token is stored.
- **Server-sent events over WebSockets.** Streaming a one-way result needs nothing more, passes through proxies cleanly, and keeps the HTTP error semantics intact until the first byte.
- **BM25 instead of an embedding service for chat.** No extra infrastructure or cost, and strong enough for single-document question answering. A vector index is the natural next step for "ask across all my documents".
- **Fail closed where it matters.** If the bot check cannot reach Cloudflare, or the quota cannot be checked, the request is refused rather than waved through.

---

## API reference

All endpoints are served by the Go gateway on port `8080`. Authenticated routes use the session cookie (or `Authorization: Bearer <session token>` for scripts). State-changing browser requests must come from an allowed origin.

<details open>
<summary><b>Summaries</b></summary>

| Method | Path | Auth | Description |
| --- | --- | :-: | --- |
| `POST` | `/public/summarize` | none | Summarize one PDF (multipart `file`). Nothing is saved. |
| `POST` | `/public/summarize-multiple` | none | Summarize up to 5 PDFs (multipart `files`). |
| `POST` | `/public/summarize-text` | none | Summarize pasted text (`{"text": "..."}`). |
| `POST` | `/public/summarize-url` | none | Summarize the web page (or PDF) at an address (`{"url": "https://..."}`). |
| `POST` | `/summarize`, `/summarize-multiple`, `/summarize-text`, `/summarize-url` | session | Same, but the result is saved to the user's history. Counts against the daily quota. |
| `POST` | `/documents/:id/chat` | session | Ask a question about a saved document (`{"question", "history"}`). Replies with `{"answer", "sources"}`: the answer has `[1]`-style markers, and `sources` lists the cited passages (`id`, `text`, `page`, `pageEnd`, `document`). |
| `GET` | `/documents?limit=20&before=<id>` | session | List saved documents, newest first. The next cursor is in the `X-Next-Cursor` header. |
| `DELETE` | `/documents/:id` | session | Delete a saved document (and its stored original PDFs). |
| `PUT` | `/documents/:id` | session | Rename a document and/or replace its tags (`{"filename"?, "tags"?}`; up to 8 tags of 30 characters). `GET /documents` also takes `?q=` (title or summary) and `?tag=`, and `GET /documents/tags` lists your tags with counts. |
| `POST` | `/documents/:id/rewrite` | session | Write the summary again (`{"style"?, "wordCount"?, "language"?}`) from the stored text. Replaces the summary, the podcast and the study material. Counts as a summary. |
| `POST` | `/documents/:id/suggestions` | session | Questions worth asking about a document. Written once and kept; does not use the allowance. |
| `POST` | `/documents/:id/study` | session | Flashcards or a quiz (`{"kind": "flashcards" or "quiz", "regenerate"?}`). Stored material is replayed free; new material counts as a chat question. |
| `POST` / `DELETE` | `/documents/:id/share` | session | Create (or fetch) / remove the public link of a summary. |
| `POST` | `/documents/:id/email` | session | Email the summary to your own address (never to anyone else). |
| `GET` | `/shared/:token` | public | A shared summary: its title and text only. Never cached or indexed. |
| `PUT` | `/account/instructions` | session | Standing instructions added to every summary prompt (`{"customInstructions"}`, up to 500 characters). |
| `POST` | `/documents/:id/podcast` | session | A two-host conversation about a saved document (`{"language"?, "regenerate"?}`). Replies with `{"title", "language", "turns": [{"speaker": "A"|"B", "text"}], "cached"}`. The script is stored: replays are free and work even after the daily allowance is used up; writing a new one counts as a chat question. |
| `POST` | `/library/overview` | session | One briefing on the documents with a tag (`{"tag"}`, at least two documents), written from their saved summaries; takes `wordCount`, `style`, `language` and `?stream=true` like a summary, is not saved, and counts as a summary. |
| `POST` | `/library/ask` | session | Ask a question across all of your saved documents, or only those with one tag (`{"question", "history", "tag"?}`). Replies with `{"answer", "sources"}`; each source names its document (`documentId`, `documentTitle`, `savedAt`), page, and the stored original (`fileId`) when there is one. Counts as a chat question. |
| `POST` | `/documents/:id/proof` | session | Check each sentence of a saved summary against its document. Replies with the sentences, a verdict (`strong`, `weak`, `none`) for each, the matching passages with their pages, and counts. Uses no model call, so it does not count against the daily quota. |
| `GET` | `/documents/:id/files/:fileId` | session | Download one stored original PDF of your own document. Document list entries carry a `files` array (`id`, `name`, `size`). |

Common parameters: `wordCount` (50-1000), `pageLimit` (0-20), `style` (`default`, `bullets`, `brief`, `simple`, `takeaways`), `language`. Add **`?stream=true`** to any summarize route to receive server-sent events:

```text
data: {"type":"status","stage":"summarizing","done":3,"total":8}
data: {"type":"delta","text":"## Overview\n"}
data: {"type":"done","filename":"report.pdf"}
```

</details>

<details>
<summary><b>Accounts and security</b></summary>

| Method | Path | Auth | Description |
| --- | --- | :-: | --- |
| `POST` | `/signup`, `/login`, `/auth/google` | none | Create an account / start a session. |
| `POST` | `/auth/forgot-password`, `/auth/reset-password` | none | Emailed, single-use, one-hour reset link. |
| `POST` | `/auth/verify-email` | none | Redeem an emailed confirmation token. |
| `GET` | `/auth/me` | session | The signed-in user. |
| `POST` | `/auth/logout`, `/auth/logout-all` | - / session | End this session / every session. |
| `GET`, `DELETE` | `/auth/sessions`, `/auth/sessions/:id` | session | List signed-in devices, sign one out. |
| `POST` | `/auth/change-password`, `/auth/resend-verification` | session | Change password (signs out other devices); resend confirmation. |
| `GET` | `/usage` | session | Today's usage against the daily quotas. |
| `GET` | `/account/export` | session | Download all of your data as JSON. |
| `DELETE` | `/account` | session | Permanently delete the account and its data. |

</details>

<details>
<summary><b>Operations</b></summary>

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/healthz` | Liveness: the process is up. |
| `GET` | `/readyz` | Readiness: database and AI service reachable (the AI service also checks that a model is available). |
| `GET` | `/options` | The supported summary styles and languages. |

</details>

---

## Configuration

Backend settings live in the root `.env` (see [`.env.example`](.env.example)); frontend settings in `frontend/.env.local`.

| Variable | Default | Purpose |
| --- | --- | --- |
| `POSTGRES_PASSWORD` | *required* | Database password. |
| `GROQ_API_KEY` | *required* | Language model access. |
| `GOOGLE_CLIENT_ID` | *required* | Google sign-in (any placeholder to skip). |
| `LLM_MODEL`, `LLM_FALLBACK_MODELS` | `openai/gpt-oss-20b`, `openai/gpt-oss-120b` | Primary model and ordered fallbacks; a retired model is skipped automatically. |
| `LLM_TEMPERATURE`, `LLM_REASONING_EFFORT` | `0.3`, `low` | How freely the model writes (`default` asks for the provider's own value) and how long it thinks first. A low temperature keeps summaries closer to the text; the summary-quality check below is how these were chosen. |
| `MAIL_PROVIDER` | `log` | `log`, `brevo` or `resend`. See [Email](#email). |
| `MAIL_FROM`, `BREVO_API_KEY`, `RESEND_API_KEY` | - | Sender and provider credentials. |
| `REQUIRE_EMAIL_VERIFICATION` | `false` | Require a confirmed email before summarizing. |
| `QUOTA_SUMMARIES_PER_DAY`, `QUOTA_CHAT_PER_DAY` | `50`, `200` | Daily limits per signed-in user (UTC day, `0` = unlimited). |
| `TURNSTILE_SECRET` | empty (off) | Enables the Cloudflare Turnstile check. Pair with `NEXT_PUBLIC_TURNSTILE_SITE_KEY` in the frontend. |
| `COOKIE_SECURE`, `COOKIE_SAMESITE`, `COOKIE_DOMAIN` | `false` (compose), `lax` | Session cookie attributes. See [Deployment](#deployment). |
| `CORS_ALLOWED_ORIGINS`, `FRONTEND_URL` | `http://localhost:3000` | Allowed browser origins and the base URL of emailed links. |
| `TRUSTED_PROXIES` | unset | CIDRs of your reverse proxy, so client IPs (and rate limits) cannot be spoofed. |
| `RATE_LIMIT_MULTIPLIER` | `1` | Multiplies every request rate limit, for deployments with many users behind one address. |
| `STORED_FILES_MB_PER_USER` | `100` | Original PDF data kept per user for the document viewer. `0` keeps none; summaries and chat still work. |
| `EMBEDDINGS`, `EMBEDDING_MODEL`, `EMBEDDING_MIN_SCORE` | `on`, `BAAI/bge-small-en-v1.5`, per model | Search by meaning (AI service). `EMBEDDINGS=off` switches it off and search is by keyword only, which saves about 200 MB of memory. A larger model such as `thenlper/gte-base` (a 440 MB download) finds more but needs correspondingly more memory. The score is the lowest similarity that still counts as related; the default is deliberately low. Changing the model re-embeds your passages gradually as questions are asked. |
| `AI_SERVICE_TOKEN` | empty | Shared secret between the Go API and the AI service (same value on both). Required when the AI service is reachable from the internet. |
| `METRICS_TOKEN` | empty (off) | Switches on `GET /metrics` on both backends, for callers sending `Authorization: Bearer <token>`. |
| `SENTRY_DSN`, `SENTRY_ENVIRONMENT`, `SENTRY_RELEASE` | empty (off) | Opt-in error reporting. Only errors and stack traces are sent: no request bodies, cookies or documents. |
| `LOG_FORMAT`, `LOG_LEVEL` | `json`, `info` | Structured logging. `LOG_FORMAT=text` is easier to read locally. |
| `SUMMARY_CACHE_SIZE`, `SUMMARY_CACHE_TTL_SECONDS` | `256`, `3600` | In-memory summary cache (`0` disables). |
| `LLM_BREAKER_THRESHOLD`, `LLM_BREAKER_COOLDOWN_SECONDS` | `5`, `30` | Circuit breaker around the LLM provider. |

Frontend: `NEXT_PUBLIC_API_URL` (the gateway, e.g. `http://localhost:8080`), `NEXT_PUBLIC_GOOGLE_CLIENT_ID`, and optionally `NEXT_PUBLIC_TURNSTILE_SITE_KEY`.

### Email

Verification and password-reset links go through a small provider interface. `log` (the default) prints links to the API log so the whole flow works locally with no account.

| Provider | Free tier | Notes |
| --- | --- | --- |
| [Brevo](https://www.brevo.com/) | 300 emails/day | Can start from a single verified sender address; a domain improves deliverability (Gmail senders are discouraged). |
| [Resend](https://resend.com/) | 3,000/month, 100/day | Simple API; needs a verified domain. |

Check each provider's current terms when you sign up.

---

## Observability

Both backends expose Prometheus metrics at `/metrics`. The endpoint is **off unless `METRICS_TOKEN` is set**, and then answers only to callers presenting it as a bearer token.

| Metric family | Answers |
| --- | --- |
| `http_requests_total`, `http_request_duration_seconds` | Traffic, error rate and latency per route (route patterns only, so label cardinality stays bounded) |
| `ai_service_requests_total`, `ai_service_response_seconds` | How the Go API sees the AI service |
| `llm_requests_total`, `llm_request_duration_seconds` | Model calls by outcome (`ok`, `provider_error`, `model_missing`, `circuit_open`, `cancelled`) and latency |
| `llm_tokens_total` | Prompt and completion tokens, for cost tracking |
| `summary_cache_lookups_total`, `llm_circuit_open` | Cache hit ratio and circuit-breaker state |
| `account_events_total`, `quota_rejections_total` | Logins, failed logins, sign-ups, users hitting their daily limits |

For a ready-made dashboard, start Prometheus and Grafana next to the normal stack:

`````````bash
docker compose -f docker-compose.yml -f docker-compose.observability.yml up -d
# Grafana:    http://localhost:3001   (the "Inkling" dashboard opens by default)
# Prometheus: http://localhost:9090
`````````

The dashboard covers overview, traffic, latency, the language model (including tokens per call and an estimated spend using prices you set), account events and resources. Both tools are bound to localhost, and the token in that file is a fixed development value.

**Error reporting** is opt-in: set `SENTRY_DSN` and unhandled errors and panics are sent to Sentry with their stack trace and request ID. Request bodies, headers, cookies, local variables and breadcrumbs are never attached, because here they would be people's documents.

---

## Testing

The project has **800+ automated tests** across the stack, and every one runs in CI on each push and pull request, together with linting, type checking and vulnerability scanning.

| Layer | What runs | Where |
| --- | --- | --- |
| Go API | Unit and Postgres integration tests with the race detector, `golangci-lint`, `govulncheck` | `go-api/` |
| AI service | `pytest` with coverage, `ruff` (lint and format), `mypy`, `pip-audit` | `python-ai-service/` |
| Frontend | `eslint`, `tsc`, Vitest unit tests with coverage, production build, `pnpm audit` | `frontend/` |
| End to end | Playwright drives the whole stack in a real browser (sign-up, login, reset, summaries, chat, account, CSP) | `frontend/e2e/` |

`````````bash
# Go: unit tests (integration tests are skipped without a database)
cd go-api && go test ./...

# Go: add the Postgres integration tests (migrations, sessions, quotas, CSRF, account deletion)
createdb summarizer_test    # the database name must contain "test": its schema is wiped
TEST_DSN="host=localhost port=5433 user=user password=... dbname=summarizer_test sslmode=disable" go test -race ./...

# Python
cd python-ai-service
pip install -r requirements.txt -r requirements-dev.txt
pytest --cov && ruff check . && ruff format --check . && mypy .

# Frontend
cd frontend
pnpm lint && pnpm typecheck && pnpm test

# End to end (starts its own stack on ports 13000, 18080 and 18081; needs Go, Docker for Postgres,
# and Chrome. Reuses the compose database with a separate "summarizer_e2e" database.)
pnpm exec playwright install chromium   # once, if you do not have Chrome
pnpm test:e2e
`````````

The end-to-end suite replaces the LLM with a small stub that speaks the AI service's HTTP contract, so it is fast, free and deterministic; the real model is covered by the Python tests and by manual runs.

**Summary quality.** Passing tests say the code works; they cannot say whether a summary is *right*. `python-ai-service/evals/summary` summarizes twelve made-up documents whose facts are known (a lease, a quarterly report, a slide deck, a C++ explainer, meeting notes, a warranty, a table of figures, a Spanish article, a 40,000-character report that needs several passes, three documents together, and the overview of a collection), several times each, with the real model, and scores every summary without another model: did it keep the facts, did it make the mistakes that kind of document invites (a currency the text never names, "year on year" for "on the previous quarter", a figure moved to the wrong month, a warranty exclusion turned into cover), did it invent a number or a date, does it have the format, language and length it was asked for, does it mention its own word count. The share of *clean* runs is the headline number. It found real problems in the first run (57% clean: a currency invented on a slide deck every time, an impossible "31 February", a Spanish article "summarized in English" that came back in Spanish, "(about 150 words)" printed into the summary), which prompt rules against inventing figures and always naming the output language fixed (95% clean over 126 summaries), and it picked the model's temperature (92% clean at the provider's default, 97% at 0.2, 98% at 0.4). The scoring and the documents are unit-tested offline in CI (every hand-written reference summary must score clean, and known-bad summaries must be caught); the model runs are manual, since every call counts against your provider key (the runner says how many it is about to make and refuses more than 150 unless you add `--yes`):

`````````bash
cd python-ai-service
python evals/summary/run_eval.py                       # every case, 3 samples each: about 100 model calls
python evals/summary/run_eval.py --samples 6 --yes --label my-change --baseline evals/summary/baseline.json   # about 200 calls
python evals/summary/run_eval.py --compare evals/summary/results/a.json evals/summary/results/b.json
`````````

What is covered, beyond the happy paths: concurrency (a quota of 3 admits exactly 3 of 12 simultaneous requests), single-use tokens under races, session revocation, CSRF and CORS, streaming edge cases (failures mid-stream, large events, cancellation), migrations applied to a legacy-shaped database, circuit-breaker state transitions, and log hygiene (no query strings, bodies or recipient addresses in logs).

---

## Security

| Area | Measure |
| --- | --- |
| Sessions | Random 256-bit tokens in `httpOnly` cookies, only a hash stored; 30-day sliding expiry with a 90-day hard limit; revoked on logout, password change, reset and account deletion. |
| Passwords | bcrypt, 8-72 characters, common-password blocklist, equal-time comparison for unknown users, per-account login throttle. |
| CSRF | `Origin` check on every state-changing request, strict credentialed CORS allowlist, `SameSite` cookies. |
| Injection and XSS | Parameterized SQL only; nonce-based Content-Security-Policy with `strict-dynamic`; the API serves a `default-src 'none'` CSP. |
| Abuse | Per-IP and per-user rate limits, atomic daily quotas, optional Turnstile, request and upload size caps. |
| Fetching web pages | The AI service only opens public addresses: every address, and every redirect, must resolve to the public internet (no loopback, private ranges, cloud metadata or Docker-internal names), the address the connection really reached is checked before any of the reply is read (so DNS tricks fail), and downloads are capped in size and time, with no proxies, cookies or credentials. |
| Uploaded Office files | `.docx` and `.pptx` are unpacked with size limits against zip bombs, and XML with entity declarations is refused. |
| Privacy | Source text never returned to clients; logs contain route patterns and IDs, not content; one-click export and hard deletion. |
| Service to service | The AI service can require a shared secret (`AI_SERVICE_TOKEN`) so it is safe on a public URL; `/metrics` needs its own bearer token and is off by default; compose binds the database and AI service to localhost only. |
| Containers | Non-root users; the Go API ships as a static binary in a `scratch` image with nothing else in it; health checks on every service. |
| Dependencies | Versions pinned in `go.sum`, `pnpm-lock.yaml` and `requirements.txt`. CI fails on known vulnerabilities (`govulncheck`, `pip-audit`, `pnpm audit`), and Dependabot opens weekly update PRs. |

To report a vulnerability, please open a private security advisory on GitHub instead of a public issue.

---

## Deployment

The backend is a standard 12-factor stack: build the `go-api` and `python-ai-service` images, provide a Postgres database, and set the environment variables above. The frontend is a standard Next.js app (for example on Vercel).

A [`render.yaml`](render.yaml) Blueprint describes the backend for [Render](https://render.com) (both services and a Postgres database), and **[docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)** is a step-by-step guide covering Vercel, free-tier limits, monitoring, a staging copy and rollback. When the AI service is publicly reachable, set the same `AI_SERVICE_TOKEN` on both backends; without it the AI service would accept requests from anyone.

**Frontend and API on different sites** (for example `vercel.app` and `onrender.com`): set `COOKIE_SECURE=true`, `COOKIE_SAMESITE=none`, `CORS_ALLOWED_ORIGINS` and `FRONTEND_URL` to the frontend origin, and `TRUSTED_PROXIES` to your proxy's CIDRs. Browsers that block third-party cookies (Safari, Firefox) will not keep a login across two unrelated domains, so for production put both under one parent domain (`app.example.com` and `api.example.com`) and use `COOKIE_SAMESITE=lax` with `COOKIE_DOMAIN=.example.com`.

Schema changes are SQL migrations in [`go-api/migrations/`](go-api/migrations), applied automatically on startup under an advisory lock, so several instances can start at once.

---

## Project structure

```text
.
├── frontend/                 Next.js app (App Router)
│   ├── app/                  Pages: dashboard, account, login, signup, reset, verify
│   ├── components/           UI, including summarizer/ (form, input, output panels)
│   ├── hooks/useSummarizer   Streaming client state machine
│   ├── lib/                  API helper, SSE reader, markdown and PDF export helpers
│   └── middleware.ts         Per-request nonce Content-Security-Policy
├── go-api/
│   ├── server/               Router: every route and the middleware in front of it
│   ├── controllers/          Handlers: auth, account, summaries, chat, streaming proxy
│   ├── middleware/           Sessions, CSRF, quotas, rate limits, Turnstile, logging
│   ├── metrics/              Prometheus metrics
│   ├── auth/                 Tokens, sessions, cookies, single-use email tokens
│   ├── mailer/               log / Brevo / Resend providers and email templates
│   ├── migrations/           Embedded SQL migrations
│   └── testutil/             Per-package Postgres schemas for integration tests
├── python-ai-service/
│   ├── main.py               Endpoints and the summarization pipeline
│   ├── llm.py                Model verification, fallback, circuit breaker
│   ├── prompts.py            Styles, languages, prompt construction
│   ├── retrieval.py          BM25 retrieval; numbered passages with page and file for citations
│   ├── embeddings.py         Local embedding model: vectors for searching documents by meaning
│   ├── study.py              Suggested questions, flashcards and quizzes: prompts and tolerant parsers
│   ├── evals/search/         Test library and questions for measuring search quality (run before changing the model)
│   ├── citations.py          Cleans the model's [n] markers so each points at a real passage
│   ├── attribution.py        Checks each summary sentence against the document (the proof check)
│   ├── podcast.py            Two-host script prompt, and reading the model's transcript back
│   ├── cache.py              Summary cache
│   ├── logging_setup.py      JSON logs and request IDs
│   ├── metrics.py            Prometheus metrics
│   ├── internal_auth.py      Shared-secret check for the AI service
│   └── error_reporting.py    Opt-in Sentry
├── observability/            Prometheus config and the Grafana dashboard
├── docs/                     DEPLOYMENT.md and the demo GIF
├── render.yaml               Render Blueprint
├── docker-compose.observability.yml   Prometheus + Grafana add-on
└── docker-compose.yml
```

---

## Contributing

Issues and pull requests are welcome. Before opening a PR, please make sure the suites for the parts you touched pass (see [Testing](#testing)); CI runs all of them. Commits follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).

## License

Released under the [MIT License](LICENSE).
