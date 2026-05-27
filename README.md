# AIOJ - Lightweight Online Judge

A fully functional, scalable, and lightweight Online Judge for competitive programming.

## Features
- **Monolith Go Backend**: High performance, easy to deploy (Chi Router, PostgreSQL).
- **Dockerized Sandbox Execution**: Uses `criyle/go-judge` for secure, cgroups-isolated code execution.
- **Language Support**: 12+ pre-configured languages (C, C++, Python, Java, Rust, Node.js, etc.) via simple YAML configurations.
- **VJudge Bot Support**: Submit problems seamlessly to remote platforms (like Codeforces) via integrated background bots.
- **Advanced Contest Management**: Scoreboards, freeze times, and ACM/OI scoring rules.
- **Collaboration Permissions**: Codeforces-style permission system (Owner, Co-author, Tester) for problems and contests.
- **Modern React SPA**: Vite + React 19 + TypeScript + Tailwind CSS with a CodeMirror 6 code editor.

## Getting Started

### Prerequisites
- Docker and Docker Compose

### 1. Setup Environment
```bash
cp .env.example .env
```

### 2. Start the System
```bash
docker compose up -d
```

Wait a few seconds for the database and judge executor containers to initialize.

### 3. Run Database Migrations
Run the migrations to create the tables and seed the default admin account:
```bash
make migrate-up
```

### 4. Access the Application
- Open your browser and navigate to: [http://localhost](http://localhost)

### Default Admin Credentials
You can immediately log in and access the Admin Dashboard and Problem Setter workspace using:
- **Username:** `admin`
- **Password:** `admin_secret`

---

## Architecture Overview
- **Backend:** `:8080` (Go)
- **Frontend:** `:80` (Nginx + React SPA)
- **Database:** `:5432` (PostgreSQL 18)
- **Sandbox:** `:5050` (go-judge)
