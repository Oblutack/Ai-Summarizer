 # AI Document Summarizer
<p align="center">
  <a href="https://ai-summarizer-ten-tan.vercel.app" target="_blank">
    <img src="https://img.shields.io/badge/Live_Demo-Visit_Now-brightgreen?style=for-the-badge" alt="Live Demo" />
  </a>
</p>

    
<p align="center">
  <img src="https://github.com/user-attachments/assets/90c61360-bc65-4457-bbb4-0d645fb76b33" alt="Application Demo" width="800"/>
</p>

  
<p align="center">
  An intelligent web application for generating concise summaries from PDF documents and text-based content. Built with a modern, scalable microservice architecture and a unique E-Ink inspired design.
</p>


---

## Table of Contents

- [Key Features](#key-features)
- [Tech Stack](#tech-stack)
- [System Architecture](#system-architecture)
- [Getting Started](#getting-started)
- [Future Roadmap](#future-roadmap)
- [License](#license)

---

## Key Features

-   **Dual Input Modes**: Summarize content by either uploading a PDF document or directly pasting text.
-   **Multi-Document Summaries**: Attach up to 5 PDFs and get one combined summary that notes overlaps and differences between them.
-   **Summary Styles**: Standard, bullet points, executive brief, "explain simply", or takeaways with action items.
-   **Output Language**: Get the summary in any of 15 languages, whatever language the source is in.
-   **Chat With Your Documents**: Registered users can ask questions about any saved summary from the dashboard. Long documents are searched with BM25 so only the relevant passages reach the model.
-   **Advanced Summarization Control**: 
    -   **Word Count Slider**: For short summaries, precisely control the desired length.
    -   **Page Limit Input**: For long documents, request a detailed summary of a specific page length.
-   **Dynamic Summarization Strategy**: Automatically switches between a simple summarization method for short texts and a powerful **MapReduce** strategy for long documents.
-   **Secure User Authentication**: Full registration and login system with both email/password (hashed with bcrypt) and **Google OAuth 2.0**.
-   **Persistent History**: Registered users can save, view, and re-download their summarization history.
-   **Professional PDF Export**: Generate beautifully formatted PDF documents from summaries, featuring custom fonts and proper pagination.
-   **Unique E-Ink UI**: A custom-designed, minimalist interface inspired by e-ink displays for enhanced readability and focus.

---

## Tech Stack

This project is built with a decoupled architecture, ensuring each component is optimized for its specific task.

| Component         | Technology                                                                                                                                                                                            | Description                                        |
| ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------- |
| **Frontend**      | ![Next.js](https://img.shields.io/badge/Next.js-000000?style=flat-square&logo=next.js&logoColor=white) ![TypeScript](https://img.shields.io/badge/TypeScript-3178C6?style=flat-square&logo=typescript&logoColor=white) ![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-38B2AC?style=flat-square&logo=tailwind-css&logoColor=white) | A modern, server-rendered React application.       |
| **API Gateway**   | ![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white) ![Gin](https://img.shields.io/badge/Gin-0078D6?style=flat-square&logo=gin&logoColor=white)                                                                          | The main API responsible for user management and request routing. |
| **AI Service**    | ![Python](https://img.shields.io/badge/Python-3776AB?style=flat-square&logo=python&logoColor=white) ![FastAPI](https://img.shields.io/badge/FastAPI-009688?style=flat-square&logo=fastapi&logoColor=white) ![LangChain](https://img.shields.io/badge/LangChain-FFFFFF?style=flat-square&logo=langchain&logoColor=black) | A dedicated microservice for AI logic and summarization. |
| **Database**      | ![PostgreSQL](https://img.shields.io/badge/PostgreSQL-316192?style=flat-square&logo=postgresql&logoColor=white)                                                                                         | Stores all user and document data.                 |
| **Orchestration** | ![Docker](https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white)                                                                                                     | Manages the local development environment.         |
| **LLM**           | ![Groq](https://img.shields.io/badge/Groq-F55036?style=flat-square)                                                                                                                                     | Llama 3.1 8B served through Groq's OpenAI-compatible API. |

---

## System Architecture

-   **Frontend**: A **Next.js** application handles the user interface. It communicates with the API Gateway for all operations.
-   **API Gateway**: A **Go (Gin)** service acts as the central entry point. It manages user authentication (JWT), handles registration/login, and routes summarization requests to the appropriate service.
-   **AI Service**: A **Python (FastAPI)** microservice contains all the AI logic. It uses **LangChain** to interface with a language model, process PDF files, and perform summarization tasks.
-   **Database**: A **PostgreSQL** instance stores all user data and saved document summaries.
-   **Containerization**: The entire backend stack is containerized and managed by **Docker Compose**, allowing for a one-command setup.

---

## Getting Started

Follow these steps to get the complete application running on your local machine.

### Prerequisites

-   Git
-   Docker Desktop
-   A [Groq](https://console.groq.com) API key
-   Go (v1.24+)
-   Python (v3.12+)
-   Node.js (v20+ LTS) with `pnpm` (`npm install -g pnpm`)

### Installation & Setup

1.  **Clone the Repository**
    ```bash
    git clone https://github.com/your-github-username/ai-summarizer.git
    cd ai-summarizer
    ```

2.  **Configure Environment Variables**
    -   Copy `.env.example` to `.env` in the project root and fill in the database password, a JWT `SECRET` (e.g. `openssl rand -base64 48`), your `GOOGLE_CLIENT_ID` and your `GROQ_API_KEY`. `docker compose` refuses to start if any are missing, and `.env` is gitignored.
    -   In `frontend/`, create `.env.local`:
        ```
        NEXT_PUBLIC_API_URL="http://localhost:8080"
        NEXT_PUBLIC_GOOGLE_CLIENT_ID="YOUR_GOOGLE_CLIENT_ID"
        ```

3.  **Launch the Application Stack**
    In two separate terminals at the project root:
    ```bash
    # Terminal 1: Start the backend services
    docker compose up --build

    # Terminal 2: Start the frontend development server
    cd frontend
    pnpm install
    pnpm dev
    ```

### Running the tests

```bash
cd go-api && go test ./...
cd python-ai-service && pip install -r requirements.txt -r requirements-dev.txt && pytest
cd frontend && pnpm lint
```

### Limits

Summarization and chat endpoints are rate limited per IP. PDFs are capped at 10 MB each (5 files and 25 MB per multi-document request) and pasted text at 200,000 characters. Chat questions are capped at 1,000 characters. Passwords must be 8-72 characters.

Documents saved before the chat feature existed have no stored source text, so chat is only offered for documents summarized after it was added.

### Operations

- **Health:** both services expose `/healthz` (liveness) and `/readyz` (readiness: database and AI service reachable, a language model available). Docker Compose waits on these.
- **Migrations:** the schema is managed with SQL migrations in `go-api/migrations/`, applied automatically on startup. Add a new numbered `.up.sql`/`.down.sql` pair for any schema change.
- **Logging:** JSON lines on stdout with a request id (`X-Request-ID`) that is passed from the Go API to the AI service, so one id traces a request through both. Set `LOG_FORMAT=text` for readable local logs and `LOG_LEVEL=debug|info|warn|error`.
- **Resilience:** repeated provider failures open a circuit breaker so requests fail fast with a 503 instead of hanging (`LLM_BREAKER_THRESHOLD`, `LLM_BREAKER_COOLDOWN_SECONDS`). If a configured model is retired the AI service falls back to the next one in `LLM_FALLBACK_MODELS`.
- **Caching:** identical summarize requests (same text, options and model) are answered from an in-memory cache (`SUMMARY_CACHE_SIZE`, `SUMMARY_CACHE_TTL_SECONDS`; set the size to `0` to disable).
- **Pagination:** `GET /documents?limit=20&before=<id>` returns newest first; the next cursor is in the `X-Next-Cursor` response header.
- **Integration tests:** set `TEST_DSN` to a Postgres database whose name contains `test` to run the migration and pagination tests (they wipe that database's schema).

---

## Usage

-   **Web Application**: Access the frontend at `http://localhost:3000`.
-   **API Gateway**: The Go API is available at `http://localhost:8080`.

---

## Future Roadmap

-   [x] E-Ink Inspired UI/UX
-   [x] Google OAuth Integration
-   [x] Customizable Summaries (Word Count & Page Limit)
-   [x] PDF Export
-   [x] **Enhanced UX with Animations**: Implement subtle transitions and micro-interactions using Framer Motion.
-   [x] **Deployment Configuration**: Prepare the application for live deployment on services like Vercel and Render.

---

## License

This project is licensed under the **MIT License**.
