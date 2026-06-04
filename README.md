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

---

## Codeforces VJudge Bot (cf-submit)

The cf-submit service uses CloakBrowser to log into Codeforces and submit solutions on behalf of a bot account. Because Cloudflare's managed challenge blocks datacenter IPs (including Docker/VPS), **the service must run on a machine with a residential IP** — either your local machine or a VPS behind a residential proxy.

### macOS (local development)

Run the service directly on your Mac so CloakBrowser uses your residential IP:

```bash
pip install cloakbrowser fastapi uvicorn requests

# Start on port 8003 (Docker backend reaches it via host.docker.internal:8003)
PORT=8003 python3 deploy/cf-submit/server.py
```

To start automatically on login, create a LaunchAgent:

```bash
mkdir -p ~/Library/LaunchAgents
cat > ~/Library/LaunchAgents/com.aioj.cf-submit.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.aioj.cf-submit</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/bin/python3</string>
        <string>/path/to/AIOJ/deploy/cf-submit/server.py</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PORT</key>
        <string>8003</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/cf-submit.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/cf-submit.log</string>
</dict>
</plist>
EOF

# Replace /path/to/AIOJ with the actual path, then load:
launchctl load ~/Library/LaunchAgents/com.aioj.cf-submit.plist
```

Make sure `cmd/aioj/main.go` points to `http://host.docker.internal:8003`.

### Linux VPS (production)

On a VPS, Cloudflare blocks the datacenter IP. Set `CF_PROXY` to a residential proxy:

```bash
pip install cloakbrowser fastapi uvicorn requests

# Install Chromium runtime deps and fonts
sudo apt-get install -y xvfb \
    libglib2.0-0 libgobject-2.0-0 libnspr4 libnss3 \
    libatk1.0-0 libatk-bridge2.0-0 libcups2 libxkbcommon0 \
    libatspi2.0-0 libxcomposite1 libxdamage1 libxfixes3 \
    libcairo2 libpango-1.0-0 libasound2 \
    fonts-noto-color-emoji fonts-freefont-ttf fonts-unifont

python -m cloakbrowser install
```

Create a systemd service at `/etc/systemd/system/cf-submit.service`:

```ini
[Unit]
Description=AIOJ cf-submit service
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/path/to/AIOJ/deploy/cf-submit
ExecStartPre=/bin/bash -c 'rm -f /tmp/.X99-lock && Xvfb :99 -screen 0 1920x1080x24 -ac &'
ExecStart=/usr/bin/python3 server.py
Environment=PORT=8003
Environment=DISPLAY=:99
Environment=CF_PROXY=http://user:pass@residential-proxy-host:port
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable cf-submit
sudo systemctl start cf-submit
sudo systemctl status cf-submit
```

Point the backend at `http://localhost:8003` (same machine) or update `CF_SUBMIT_URL` if running separately.

### Proxy format

`CF_PROXY` accepts HTTP and SOCKS5 proxies:

```
CF_PROXY=http://user:pass@host:port
CF_PROXY=socks5://user:pass@host:port
```

When `CF_PROXY` is set, `geoip=True` is automatically enabled — CloakBrowser will match the browser's timezone and locale to the proxy's exit IP, making the session look more natural.

Verify the proxy is active:
```bash
curl http://localhost:8003/health
# {"status":"ok","proxy":true}
```
