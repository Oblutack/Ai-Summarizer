# 🤖 AI Document Summarizer

![Build Status](https://img.shields.io/badge/build-passing-brightgreen)
![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)

An intelligent web application designed to summarize documents and text-based content using AI. Built with a modern, scalable microservice architecture.

<!--
ADD A SCREENSHOT OF THE APPLICATION UI HERE ONCE IT'S READY.
![Project Screenshot](link/to/your/screenshot.png)
-->

---

## ✨ Key Features

-   **Instant Summarization:** Generate summaries from pasted text or uploaded PDF documents.
-   **User Accounts:** Register to save your summary history and access enhanced features.
-   **Document History:** All summaries for registered users are saved and easily accessible.
-   **Clean & Intuitive UI:** Designed for simplicity and efficiency.

---

## 🛠️ Tech Stack

This project utilizes a modern microservice architecture, with each component built using the best tool for the job:

| Component            | Technology                               | Purpose                                      |
| --------------------- | ----------------------------------------- | ------------------------------------------ |
| **Frontend**          | Next.js, React, Tailwind CSS              | User Interface (UI)                        |
| **API Gateway**       | Go (Golang), Gin                          | Main API, user management, and request routing |
| **AI Service**         | Python, FastAPI, LangChain                | PDF processing and summary generation      |
| **Database**     | PostgreSQL                                | Storage for user data and documents        |
| **Orchestration**      | Docker, Docker Compose                    | Local development and deployment           |

---

## 🚀 Getting Started

Follow these steps to get the complete application running on your local machine.

### Prerequisites

Before you begin, ensure you have the following tools installed:
-   [Git](https://git-scm.com/)
-   [Docker Desktop](https://www.docker.com/products/docker-desktop/)
-   [Go](https://golang.org/dl/) (v1.22+)
-   [Python](https://www.python.org/downloads/) (v3.11+)
-   [Node.js](https://nodejs.org/) (v20+ LTS)

### Installation

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-github-username/ai-summarizer.git
    cd ai-summarizer
    ```

2.  **Run all services with Docker Compose:**
    This command will build the Docker images and start the Go API, Python service, and PostgreSQL database.
    ```bash
    docker compose up --build
    ```

3.  **Access the services:**
    -   🚀 **Go API Gateway** is available at: `http://localhost:8080`
    -   🧠 **Python AI Service** is available at: `http://localhost:8001`
    -   🐘 **PostgreSQL Database** is available on port: `5432`

---

## 📂 Project Structure

The project is organized as a monorepo with a clear separation of concerns:

```
/
├── frontend/          # Next.js Frontend Application
├── go-api/            # Go (Gin) API Gateway
├── python-ai-service/ # Python (FastAPI) AI Microservice
├── .gitignore         # Files and folders ignored by Git
├── docker-compose.yml # Configuration for Docker orchestration
└── README.md          # This file
```

---

## 📄 License

This project is licensed under the **MIT License**. See the `LICENSE` file for more details.
