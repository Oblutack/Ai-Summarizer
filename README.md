 # AI Document Summarizer

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
| **Local AI**      | ![Ollama](https://img.shields.io/badge/Ollama-232323?style=flat-square&logo=ollama&logoColor=white)                                                                                                       | Powers local, offline model inference during development. |

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
-   Ollama
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
    -   In the `frontend/` directory, create a `.env.local` file and add your Google Client ID:
        ```
        NEXT_PUBLIC_GOOGLE_CLIENT_ID="YOUR_GOOGLE_CLIENT_ID"
        ```
    -   In the root `docker-compose.yml`, update the `environment` section for the `go-api` service with your JWT `SECRET` and `GOOGLE_CLIENT_ID`.

3.  **Run the AI Model**
    Open a new terminal and run the following command to download and serve the Llama 3 model. Leave this running.
    ```bash
    ollama run llama3
    ```

4.  **Launch the Application Stack**
    In two separate terminals at the project root:
    ```bash
    # Terminal 1: Start the backend services
    docker compose up --build

    # Terminal 2: Start the frontend development server
    cd frontend
    pnpm install
    pnpm dev
    ```

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
-   [ ] **Enhanced UX with Animations**: Implement subtle transitions and micro-interactions using Framer Motion.
-   [ ] **Deployment Configuration**: Prepare the application for live deployment on services like Vercel and Render.

---

## License

This project is licensed under the **MIT License**.
