# AI Document Summarizer

![Build Status](https://img.shields.io/badge/build-passing-brightgreen?style=for-the-badge)
![License](https://img.shields.io/badge/license-MIT-blue?style=for-the-badge)

An intelligent web application for generating concise summaries from PDF documents and text-based content. Built with a modern, scalable microservice architecture for robust performance and maintainability.

---

## Table of Contents

- [Key Features](#key-features)
- [Tech Stack](#tech-stack)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
- [Project Structure](#project-structure)
- [Future Roadmap](#future-roadmap)
- [License](#license)

---

## Key Features

-   **AI-Powered Summarization**: Leverages local, open-source language models to generate accurate summaries from PDF documents.
-   **Secure User Authentication**: Full registration and login system using JWT for secure, session-based access.
-   **Persistent History**: Registered users can save and access their summarization history at any time.
-   **Guest Mode**: Allows anonymous users to test the core functionality without creating an account.
-   **Clean & Intuitive Interface**: A minimalist UI designed for clarity and ease of use.

---

## Tech Stack

This project is built with a decoupled architecture, ensuring each component is optimized for its specific task.

| Component         | Technology                                                                                                                                                                                            | Description                                        |
| ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------- |
| **Frontend**      | ![Next.js](https://img.shields.io/badge/Next.js-000000?style=for-the-badge&logo=next.js&logoColor=white) ![React](https://img.shields.io/badge/React-20232A?style=for-the-badge&logo=react&logoColor=61DAFB) ![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-38B2AC?style=for-the-badge&logo=tailwind-css&logoColor=white) | A modern, server-rendered React application.       |
| **API Gateway**   | ![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)                                                                                                                  | The main API responsible for user management and request routing. |
| **AI Service**    | ![Python](https://img.shields.io/badge/Python-3776AB?style=for-the-badge&logo=python&logoColor=white) ![FastAPI](https://img.shields.io/badge/FastAPI-009688?style=for-the-badge&logo=fastapi&logoColor=white) | A dedicated microservice for PDF processing and AI summarization. |
| **Database**      | ![PostgreSQL](https://img.shields.io/badge/PostgreSQL-316192?style=for-the-badge&logo=postgresql&logoColor=white)                                                                                         | Stores all user and document data.                 |
| **Orchestration** | ![Docker](https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white)                                                                                                     | Manages the local development environment.         |
| **Local AI**      | ![Ollama](https://img.shields.io/badge/Ollama-232323?style=for-the-badge&logo=ollama&logoColor=white)                                                                                                       | Powers local, offline model inference during development. |

---

## Getting Started

Follow these steps to get the complete application running on your local machine.

### Prerequisites

Ensure you have the following tools installed and running:
-   [Git](https://git-scm.com/)
-   [Docker Desktop](https://www.docker.com/products/docker-desktop/)
-   [Ollama](https://ollama.com/)
-   [Go](https://golang.org/dl/) (v1.24+)
-   [Python](https://www.python.org/downloads/) (v3.12+)
-   [Node.js](https://nodejs.org/) (v20+ LTS) with `pnpm` (`npm install -g pnpm`)

### Installation

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-github-username/ai-summarizer.git
    cd ai-summarizer
    ```

2.  **Download & Run the AI Model:**
    Open a new terminal and run the following command. The first time, this will download the Llama 3 model (approx. 4.7 GB). Leave this process running in the background.
    ```bash
    ollama run llama3
    ```

3.  **Run Backend Services:**
    In a separate terminal at the project root, start the backend infrastructure.
    ```bash
    docker compose up --build
    ```

4.  **Run the Frontend:**
    In a third terminal, navigate to the `frontend` directory and start the development server.
    ```bash
    cd frontend
    pnpm install
    pnpm dev
    ```

5.  **Access the Application:**
    -   The web application is now available at `http://localhost:3000`.
    -   The Go API Gateway is accessible at `http://localhost:8080`.

---

## Project Structure

The project is organized as a monorepo with a clear separation of concerns.

```/
├── frontend/          # Next.js Frontend Application
├── go-api/            # Go (Gin) API Gateway
├── python-ai-service/ # Python (FastAPI) AI Microservice
├── .gitignore         # Files and folders ignored by Git
├── docker-compose.yml # Configuration for Docker orchestration
└── README.md          # This file```
```
---

## Future Roadmap

The following features are planned for future development:
-   [ ] **E-Ink Inspired UI/UX**: A complete redesign focused on readability and minimalism, prototyped in Figma.
-   [ ] **Google OAuth Integration**: Allow users to sign up and log in with their Google account.
-   [ ] **Customizable Summaries**: Options to select summary length (short, medium, long) and format (plain text, markdown).
-   [ ] **PDF Export**: Functionality to download generated summaries as PDF files.
-   [ ] **Enhanced UX**: Subtle animations and transitions to improve the overall user experience.

---

## License

This project is licensed under the **MIT License**.
