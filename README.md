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

## Deploying to Production (VPS + Domain + HTTPS)

This guide walks you through deploying AIOJ on a real server with your own domain name and HTTPS (the green lock in the browser). Even if you've never deployed anything before — just follow each step.

---

### What You Will Need

| Item | Why | Estimated Cost |
|------|-----|---------------|
| **A VPS (Virtual Private Server)** | Runs AIOJ 24/7 on the internet | $5–20/month |
| **A domain name** | `your-cool-site.com` instead of an IP address | $5–15/year |
| **30 minutes** | Time to set everything up | Free |

**Recommended VPS providers for beginners:** Hetzner, DigitalOcean, Vultr, Linode.  
**Recommended domain registrars:** Namecheap, Cloudflare Registrar, Porkbun.

---

### Step 1: Get a VPS Server

1. Sign up at any VPS provider (we'll use **DigitalOcean** as an example, but the steps are similar everywhere).
2. Create a new **Droplet** (server):
   - **OS:** Ubuntu 24.04 LTS
   - **Plan:** Basic, $6/month (1 vCPU, 1 GB RAM) — enough for a small OJ
   - **Region:** Pick one closest to your users
3. Choose **SSH key** authentication (more secure than password).
4. Click **Create Droplet**. You'll get an **IP address** like `167.99.123.45`. **Save this IP** — you'll need it everywhere.

---

### Step 2: Get a Domain Name

1. Go to any domain registrar and search for an available domain.
2. Buy it. Example: `myoj.com`

---

### Step 3: Point Your Domain to Your Server (DNS)

This tells the internet: "When someone types `myoj.com`, send them to my server at `167.99.123.45`."

1. Log into your domain registrar's dashboard.
2. Find **DNS Settings** or **DNS Management** or **Manage DNS**.
3. Add an **A Record**:
   - **Name/Host:** `@` (means the root domain — `myoj.com`)
   - **Type:** `A`
   - **Value/Target:** Your VPS IP address (e.g., `167.99.123.45`)
   - **TTL:** `3600` (or leave default)
4. *(Optional)* Add a second A record for `www`:
   - **Name/Host:** `www`
   - **Type:** `A`
   - **Value/Target:** Same IP
5. Save. DNS changes can take **a few minutes to 48 hours** to propagate, but usually it's quick (5–30 minutes).

> **Verify:** Run `nslookup myoj.com` (replace with your domain). It should return your server IP. If not, wait and try again.

---

### Step 4: Set Up Your Server

SSH into your VPS:

```bash
ssh root@YOUR_SERVER_IP
```

#### 4a. Update the system

```bash
apt update && apt upgrade -y
```

#### 4b. Install Docker & Docker Compose

```bash
# Install Docker
curl -fsSL https://get.docker.com | sh

# Add your user to the docker group (so you don't need sudo)
usermod -aG docker $USER

# Log out and back in for the group change to take effect
exit
ssh root@YOUR_SERVER_IP
```

#### 4c. Configure Firewall (CRITICAL)

The firewall controls which ports are open to the internet. You need ports **22** (SSH), **80** (HTTP), and **443** (HTTPS). Everything else should be blocked.

```bash
# Install ufw (Uncomplicated Firewall)
apt install ufw -y

# Allow SSH (port 22) — IMPORTANT: do this first or you'll lock yourself out!
ufw allow 22/tcp

# Allow HTTP (port 80) — needed for Let's Encrypt verification and redirects
ufw allow 80/tcp

# Allow HTTPS (port 443) — your secure site traffic
ufw allow 443/tcp

# Enable the firewall
ufw enable

# Verify
ufw status
```

You should see:
```
Status: active

To                         Action      From
--                         ------      ----
22/tcp                     ALLOW       Anywhere
80/tcp                     ALLOW       Anywhere
443/tcp                    ALLOW       Anywhere
```

**DO NOT expose ports 5432, 6379, 5050, 8080, 9090, 3000 to the internet.** These are internal services. Only Caddy (or Nginx) on ports 80/443 should be public.

If you are using the Codeforces VJudge bot, also allow Docker containers to reach the host's cf-submit service:

```bash
# Allow Docker bridge networks to reach cf-submit on port 8003
sudo ufw allow from 172.16.0.0/12 to any port 8003 proto tcp
```

#### 4d. Clone AIOJ

```bash
git clone https://github.com/TahsinArafat/AIOJ.git /opt/aioj
cd /opt/aioj
```

---

### Step 5: Configure AIOJ for Production

#### 5a. Secure your environment variables

```bash
cp .env.example .env
nano .env
```

Change the values to **strong, random secrets**:

```env
DB_PASSWORD=use-a-very-long-random-password-here-64-chars
JWT_SECRET=another-very-long-random-string-different-from-above
JUDGE_CONCURRENCY=4
```

> **How to generate random secrets:**
> ```bash
> openssl rand -hex 32   # Generates a 64-character random hex string
> ```

#### 5b. Update config.yaml

Edit `config.yaml` and change the JWT secret:

```yaml
auth:
  jwt_secret: "paste-the-same-JWT_SECRET-from-.env-here"
```

---

### Step 6: Enable HTTPS with Caddy (RECOMMENDED)

Caddy is a web server that **automatically** gets and renews SSL certificates from Let's Encrypt. You write 5 lines of config, and it handles everything — no cron jobs, no certbot commands, no manual renewal.

#### 6a. Edit the Caddyfile

```bash
nano deploy/Caddyfile
```

Replace `yourdomain.com` with your actual domain:

```caddyfile
myoj.com {
    reverse_proxy frontend:80
    encode gzip zstd
}
```

That's it. Caddy will automatically:
- Get a free SSL certificate from Let's Encrypt
- Redirect HTTP → HTTPS for all visitors
- Renew the certificate 30 days before it expires (forever)

#### 6b. Fix port conflict (important!)

By default, the `frontend` service in `docker-compose.yml` binds to port 80. Caddy also needs port 80 and 443. Remove the frontend's port binding:

```bash
nano docker-compose.yml
```

Find the `frontend` service and **comment out** the ports line:

```yaml
  frontend:
    build:
      context: ./web
      dockerfile: Dockerfile
    # ports:                    # <-- COMMENT OUT or REMOVE this line
    #   - "80:80"              # <-- COMMENT OUT or REMOVE this line
    depends_on:
      - backend
    restart: unless-stopped
```

The frontend container still runs on port 80 **inside** Docker's internal network — Caddy can reach it there. You're just removing the public exposure so Caddy takes over.

#### 6c. Start everything

```bash
# First, start ONLY the core services (without Caddy) to initialize the database
docker compose up -d postgres judge redis backend judge-worker frontend

# Wait ~10 seconds for everything to be ready

# Run database migrations
make migrate-up

# Now start Caddy (which needs everything else running)
docker compose --profile production up -d
```

> `--profile production` tells Docker to also start the Caddy service. Without this flag, Caddy stays off — so your local development setup remains unchanged.

#### 6d. Verify HTTPS

Open your browser and go to `https://myoj.com`. You should see:
- The AIOJ login page
- A **padlock icon** (🔒) in the address bar — means HTTPS is working

**If you see a "Secure Connection Failed" error or `ERR_SSL_PROTOCOL_ERROR`:**

- Wait 1–2 minutes, then refresh. Let's Encrypt takes a moment to issue the certificate on first run.
- Check Caddy's logs for the exact error: `docker compose logs caddy --tail 20`
- **Let's Encrypt rate limit?** If you see `too many certificates (50) already issued for "example.tld"`, the domain suffix has hit its weekly limit. Common with cheap/free subdomain providers. Either wait (up to 7 days) or use a different domain. To switch domains: edit `deploy/Caddyfile` → run `docker compose stop caddy && docker compose rm -f caddy` → `docker volume rm aioj_caddy_data aioj_caddy_config` → restart.
- If you just want to run on plain HTTP temporarily, skip Caddy entirely: `docker compose stop caddy && docker compose rm -f caddy`, then uncomment `80:80` under the `frontend` service and run `docker compose up -d frontend`.

---

### Alternative: Nginx + Certbot (More Control)

If you prefer Nginx over Caddy, a ready-to-use config is at `deploy/nginx/aioj.conf`.

#### Install Nginx and Certbot

```bash
apt install nginx certbot python3-certbot-nginx -y
```

#### Configure Nginx

```bash
# Copy the config
cp deploy/nginx/aioj.conf /etc/nginx/sites-available/aioj

# Edit it — replace "yourdomain.com" with your actual domain
nano /etc/nginx/sites-available/aioj
```

Change these lines in the file:
```
server_name yourdomain.com www.yourdomain.com;
ssl_certificate     /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;
```

Replace `yourdomain.com` with your actual domain everywhere.

#### Change the frontend port

In `docker-compose.yml`, change the frontend port from `80` to `8080`:

```yaml
  frontend:
    # ...
    ports:
      - "8080:80"    # <-- Change from "80:80" to "8080:80"
```

Then start Docker:

```bash
docker compose up -d
make migrate-up
```

#### Enable the site and get SSL

```bash
# Enable the Nginx site
ln -s /etc/nginx/sites-available/aioj /etc/nginx/sites-enabled/
rm /etc/nginx/sites-enabled/default   # Remove default site

# Test Nginx config
nginx -t

# Reload Nginx
systemctl reload nginx

# Get SSL certificate (Let's Encrypt)
certbot --nginx -d myoj.com -d www.myoj.com

# Follow the prompts — choose redirect (option 2) to force HTTPS

# Test auto-renewal (Certbot adds a systemd timer automatically)
certbot renew --dry-run
```

---

### Visual Management Panels (Easiest GUI Option — No Config Files)

If you prefer clicking buttons over editing config files, use this combo:

| Panel | What You Do With It | Access At |
|-------|-------------------|-----------|
| **Nginx Proxy Manager** | Add domains, get SSL certs, manage redirects — all from a web UI | `http://your-ip:81` |
| **Dockge** | Start/stop containers, view logs, edit docker-compose, update images — all visually | `http://your-ip:5001` |

Both are free, open-source, and run as Docker containers. **You never touch a config file for domains or SSL again.**

#### Step-by-Step Setup

**1. Prepare docker-compose.yml**

Before starting, open `docker-compose.yml` and **comment out** these lines (NPM will handle ports 80/443 instead):

```yaml
  frontend:
    # ports:           # ← COMMENT THIS OUT
    #   - "80:80"      # ← COMMENT THIS OUT
```

If you previously added Caddy, you can leave it or remove it — NPM replaces both Caddy and the frontend's public port. Just don't use `--profile production`.

**2. Start AIOJ + the management panels**

```bash
# Start core services first
docker compose up -d postgres judge redis backend judge-worker frontend

# Wait 10 seconds, then run migrations
make migrate-up

# Now add NPM + Dockge using the override file
docker compose -f docker-compose.yml -f deploy/docker-compose.npm.yml up -d
```

**3. Open firewall for the admin UIs (temporary)**

The admin panels run on ports 81 and 5001. Open them temporarily while you configure things:

```bash
ufw allow 81/tcp
ufw allow 5001/tcp
```

> **After setup is complete, close these ports:** `ufw delete allow 81/tcp && ufw delete allow 5001/tcp`. Your admin panels should NOT be publicly accessible long-term.

**4. Configure Nginx Proxy Manager**

Open your browser and go to `http://YOUR_SERVER_IP:81`

- **Default login:** `admin@example.com` / `changeme`
- It will immediately ask you to change email and password — **do this first.**

Now add your domain with SSL:

1. Click **Hosts** → **Proxy Hosts** → **Add Proxy Host**
2. Fill in:
   - **Domain Names:** `myoj.com` (and optionally `www.myoj.com`)
   - **Scheme:** `http`
   - **Forward Hostname / IP:** `frontend` (the Docker service name)
   - **Forward Port:** `80`
   - **Block Common Exploits:** ✅ Check this
3. Click the **SSL** tab:
   - **SSL Certificate:** Select "Request a new SSL Certificate"
   - **Force SSL:** ✅ Check this
   - **HTTP/2 Support:** ✅ Check this
   - **Email Address:** Enter your email (Let's Encrypt needs this)
   - **Agree to Terms:** ✅ Check this
4. Click **Save**

NPM will obtain a free Let's Encrypt certificate within ~30 seconds. Visit `https://myoj.com` — you'll see the green lock.

**Renewals are automatic.** NPM renews certificates 30 days before expiry. Zero maintenance.

**5. (Optional) Explore Dockge**

Go to `http://YOUR_SERVER_IP:5001`. Dockge shows:

- **All your containers** with status, CPU, memory usage
- **One-click start/stop/restart** for any service
- **Live log viewer** — see what's happening in real time
- **docker-compose editor** — modify your compose file from the browser
- **Image update checker** — see when new versions are available

To view AIOJ in Dockge, click the "Scan" button at the bottom of the sidebar — it auto-discovers your existing compose file.

**6. Lock down admin panels**

Once everything is working, close the admin ports:

```bash
ufw delete allow 81/tcp
ufw delete allow 5001/tcp
```

To access the panels later, use an **SSH tunnel** (secure — no open ports needed):

```bash
# Run this on YOUR local machine (not the VPS):
ssh -L 81:localhost:81 -L 5001:localhost:5001 root@YOUR_SERVER_IP

# Then open in your browser:
# http://localhost:81      → Nginx Proxy Manager
# http://localhost:5001    → Dockge
```

This tunnels traffic through SSH — the admin panels are only accessible from your computer, not the internet.

---

### Step 7: Log In and Secure Admin

1. Go to `https://myoj.com`
2. Log in with the default admin credentials:
   - **Username:** `admin`
   - **Password:** `admin_secret`
3. **IMMEDIATELY change the admin password:** Go to your profile → Settings → Change Password.

---

### Port Reference (Production)

When deployed behind Caddy, Nginx, or NPM, only these ports should be open to the internet:

| Port | Service | Public? | Purpose |
|------|---------|---------|---------|
| 22 | SSH | ✅ Yes | Server management |
| 80 | Caddy/Nginx/NPM | ✅ Yes | HTTP → HTTPS redirect |
| 443 | Caddy/Nginx/NPM | ✅ Yes | HTTPS (your site) |
| 81 | NPM Admin (optional) | ⚠️ Setup only | Nginx Proxy Manager UI — close after setup, use SSH tunnel instead |
| 5001 | Dockge (optional) | ⚠️ Setup only | Docker management UI — close after setup, use SSH tunnel instead |
| 8080 | Backend (Go) | ❌ No | Internal API |
| 8003 | cf-submit (host) | ❌ No | CloakBrowser Codeforces automation — only accessible from Docker containers |
| 5432 | PostgreSQL | ❌ No | Database |
| 6379 | Redis | ❌ No | Job queue |
| 5050 | go-judge | ❌ No | Code sandbox |

---

### Troubleshooting

| Problem | Likely Cause | Fix |
|---------|-------------|-----|
| "Site can't be reached" | DNS hasn't propagated yet | Wait 5–30 min. Run `nslookup yourdomain.com` to check. |
| "Connection refused" | Firewall blocking port 80/443 | Run `ufw status`. Make sure 80/tcp and 443/tcp are ALLOW. |
| "Secure Connection Failed" on first Caddy run | SSL cert still being issued | Wait 1–2 min, refresh. Check `docker compose logs caddy`. |
| Caddy fails to start | Port 80 already in use by frontend | Did you comment out `80:80` in docker-compose.yml? (Step 6b) |
| NPM shows "502 Bad Gateway" | NPM can't reach the frontend container | In NPM, set Forward Hostname to `frontend` (Docker service name), not `localhost`. |
| NPM SSL fails ("Internal error") | Port 80 or 443 blocked by another service | Make sure you commented out `80:80` in frontend AND are not running Caddy alongside NPM. |
| NPM admin UI not loading | Firewall blocking port 81 | `ufw allow 81/tcp`. Remember to close it after setup. |
| "SSL certificate error" after domain change | Old cert cached | Delete the cert in NPM (SSL Certificates tab), request new one. Or for Caddy: `docker compose stop caddy && docker compose rm caddy`. |
| Migrations fail | Database not ready | Wait longer after `docker compose up`. Check: `docker compose logs postgres`. |
| 502 Bad Gateway | Backend container not running | `docker compose ps` — all services should be "Up". |
| Codeforces submission shows SE (System Error) | cf-submit not reachable, bot locked, or Cloudflare blocked | See [Codeforces VJudge Bot Debugging](#debugging-cf-submit) section below. Common causes: missing proxy, wrong CF_SUBMIT_URL, bot account password empty, firewall blocking Docker→host traffic. |
| `dial tcp 172.x.x.x:8003: i/o timeout` | UFW blocking Docker→host traffic | `sudo ufw allow from 172.16.0.0/12 to any port 8003 proto tcp && sudo ufw reload` |
| `lookup host.docker.internal: no such host` | `extra_hosts` not configured | Ensure `extra_hosts: ["host.docker.internal:host-gateway"]` is present in `docker-compose.yml` under `backend` and `judge-worker`. |
| `login form not found. CF blocked: False` | Proxy not working or JS rendering failed | Verify proxy: `curl -s localhost:8003/health` should show `"proxy":true`. Clear stale browser profile: `rm -rf /tmp/cb-profile`. Check screenshots in `/tmp/cf-screenshots/`. |
| `login form not found. CF blocked: True` | Cloudflare challenge page returned | Your proxy is not working. Codeforces sees your real (datacenter) IP. Get a residential proxy. |
| `cf-submit error: crash: ... ImportError: geoip2 is required` | `cloakbrowser[geoip]` not installed | Reinstall: `pip install "cloakbrowser[geoip]" --break-system-packages` |

---

## Codeforces VJudge Bot (cf-submit)

AIOJ can submit solutions to Codeforces on behalf of bot accounts. The `cf-submit` service runs a headful Chromium browser via CloakBrowser (Playwright-based) to bypass Cloudflare's bot detection and interact with Codeforces' login and submission pages.

**Cloudflare blocks datacenter IPs.** On a VPS (AWS, DigitalOcean, Hetzner, etc.) you MUST route through a residential proxy. Without a proxy, Codeforces will return a "Just a moment..." Cloudflare challenge page instead of the login form.

### How It Works

```
User submits to CF problem
       │
       ▼
AIOJ Backend → POST /submit → cf-submit service
       │
       ▼
CloakBrowser (headful Chromium) → Codeforces.com
       │
       ▼
Returns submission ID → Backend polls CF API for verdict
```

### Architecture Options

You can run `cf-submit` either:

| Option | Setup | Default Port | Use When |
|--------|-------|------|----------|
| **systemd service** on host | `systemctl` | `8003` | Best for simplicity — runs Python directly on the host, no Docker overhead |
| **Docker container** | `docker compose` | `8002/8003` | Runs inside Docker network, no systemd needed |

**Recommended: systemd service on host (port 8003)**. It avoids Docker networking complexity and page-rendering issues inside containers.

---

### Environment Variables

| Variable | Used By | Purpose |
|----------|---------|---------|
| `CF_PROXY` | cf-submit (Python) | Residential proxy URL (`http://user:pass@host:port` or `socks5://user:pass@host:port`). **Required on VPS.** |
| `CF_SUBMIT_URL` | Backend (Go) | Override the cf-submit base URL. Default: `http://host.docker.internal:8003` |
| `PORT` | cf-submit (Python) | Listen port for the FastAPI server. Default: `8002` |

---

### Option A: systemd Service on Host (Recommended)

The cf-submit service runs as a systemd unit directly on the VPS host, listening on port 8003. The Docker backend container reaches it via `host.docker.internal:8003`.

#### 1. Install Dependencies

```bash
# Install Python packages (use --break-system-packages on Ubuntu 24.04+)
pip install "cloakbrowser[geoip]" fastapi uvicorn requests --break-system-packages

# Install Chromium runtime deps and fonts
# Ubuntu 24.04 (t64 suffix):
sudo apt-get install -y xvfb \
    libglib2.0-0t64 libgobject-2.0-0 libnspr4 libnss3 \
    libatk1.0-0t64 libatk-bridge2.0-0t64 libcups2t64 libxkbcommon0 \
    libatspi2.0-0t64 libxcomposite1 libxdamage1 libxfixes3 \
    libcairo2 libpango-1.0-0 libasound2t64 \
    fonts-noto-color-emoji fonts-freefont-ttf fonts-unifont

# Ubuntu 22.04:
# sudo apt-get install -y xvfb \
#     libglib2.0-0 libgobject-2.0-0 libnspr4 libnss3 \
#     libatk1.0-0 libatk-bridge2.0-0 libcups2 libxkbcommon0 \
#     libatspi2.0-0 libxcomposite1 libxdamage1 libxfixes3 \
#     libcairo2 libpango-1.0-0 libasound2 \
#     fonts-noto-color-emoji fonts-freefont-ttf fonts-unifont

# Install Chromium browser binary
python3 -m cloakbrowser install
```

#### 2. Configure the Proxy

Edit the systemd service file before creating it if using a proxy:
```bash
# Edit the Environment line in the service file below to include:
# Environment=CF_PROXY=http://user:pass@residential-proxy-host:port
```

#### 3. Create the systemd Service

```bash
sudo nano /etc/systemd/system/cf-submit.service
```

```ini
[Unit]
Description=AIOJ cf-submit service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/aioj/deploy/cf-submit
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

Start it:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now cf-submit
sudo systemctl status cf-submit
```

#### 4. Configure UFW (Firewall)

Docker containers use a private bridge network. The backend container must reach the host's port 8003. Allow Docker-originated traffic:

```bash
sudo ufw allow from 172.16.0.0/12 to any port 8003 proto tcp
sudo ufw reload
```

> `172.16.0.0/12` covers all Docker bridge subnets (`172.17.x.x` through `172.31.x.x`).

#### 5. Configure Docker to Reach the Host

The backend container uses `host.docker.internal` to reach the host. Docker Compose needs `extra_hosts` for this to resolve on Linux. The default `docker-compose.yml` already includes this:

```yaml
backend:
  extra_hosts:
    - "host.docker.internal:host-gateway"
```

#### 6. Verify

```bash
# Health check (confirms service is up and proxy is loaded)
curl -s http://localhost:8003/health
# → {"status":"ok","proxy":true}

# Test login directly (replace with your credentials)
curl -s -X POST http://localhost:8003/login \
  -H "Content-Type: application/json" \
  -d '{"username":"YOUR_BOT_HANDLE","password":"YOUR_BOT_PASSWORD"}'
# → {"status":"ok","message":"login successful"}

# Test reachability from inside the Docker backend container
docker compose exec backend sh -c "wget -qO- http://host.docker.internal:8003/health 2>&1"
```

---

### Option B: Docker Container

The `cf-submit` Docker container runs with `network_mode: host` to avoid Docker bridge networking issues. It binds directly to the host's port 8003.

#### 1. Configure `.env`

```bash
# In /opt/aioj/.env
CF_PROXY=http://user:pass@residential-proxy-host:port
CF_SUBMIT_URL=http://host.docker.internal:8003   # or omit (default)
```

#### 2. Start the Container

```bash
docker compose up -d --build cf-submit
docker compose logs cf-submit --tail 20
```

#### 3. Stop the systemd Service (If Running)

```bash
sudo systemctl stop cf-submit
sudo systemctl disable cf-submit
```

---

### Setting Up a Bot Account

1. Log into AIOJ as admin.
2. Go to **Admin Panel → Bot Accounts**.
3. Add a new Codeforces bot account:
   - **Platform:** `codeforces`
   - **Handle:** The Codeforces username (e.g., `JUST_Analytics`)
   - **Password:** The Codeforces account password
   - **Status:** `active`
4. The backend will use the first active bot account for submissions.

Alternatively, insert directly into PostgreSQL:
```bash
docker compose exec postgres psql -U aioj -d aioj -c "
  INSERT INTO bot_accounts (id, platform, platform_user, platform_pass, status, consecutive_failures)
  VALUES (gen_random_uuid(), 'codeforces', 'YOUR_HANDLE', 'YOUR_PASSWORD', 'active', 0);
"
```

#### Resetting a Locked Bot Account

After 3 consecutive failures, the bot is skipped. Reset it:
```bash
docker compose exec postgres psql -U aioj -d aioj -c "
  UPDATE bot_accounts SET consecutive_failures = 0, status = 'active';
"
```

---

### Debugging cf-submit

#### Check Service Health

```bash
curl -s http://localhost:8003/health
# {"status":"ok","proxy":false}  → proxy NOT configured!
# {"status":"ok","proxy":true}   → proxy is active
```

#### Check Backend Logs for Submission Issues

```bash
docker compose logs backend --tail 50 -f
```

Key log messages and what they mean:

| Log Message | Meaning |
|-------------|---------|
| `async submit: submitting to remote OJ` | Backend picked up the submission, routing to vjudge |
| `cf-submit failed, falling back to direct POST` | cf-submit service errored; falling back to direct HTTP (will fail on VPS) |
| `dial tcp ... i/o timeout` | Network issue — backend container can't reach cf-submit (check UFW) |
| `lookup host.docker.internal: no such host` | `extra_hosts` not configured in docker-compose.yml |
| `login form not found. CF blocked: False` | Browser rendered page but inputs didn't appear (proxy or rendering issue) |
| `CF blocked: True` | Cloudflare challenge page returned (proxy not working or missing) |
| `vjudge submitted via bot` | **Success!** Submission sent to Codeforces |
| `all bots failed` | All bot accounts exhausted or locked |

#### Check cf-submit Logs

```bash
# systemd service
sudo journalctl -u cf-submit -n 50 --no-pager -f

# Docker container
docker compose logs cf-submit --tail 50 -f
```

#### View Browser Screenshots

CloakBrowser captures screenshots at each step. They're saved to `/tmp/cf-screenshots/`:

```bash
ls -la /tmp/cf-screenshots/
```

| Screenshot | Captured When |
|------------|---------------|
| `01_enter.png` | After navigating to login page |
| `02_after_login.png` | After submitting login credentials |
| `03_submit.png` | After navigating to submit page |
| `04_before_submit.png` | Before clicking submit |
| `05_after_submit.png` | After clicking submit |
| `99_no_form.png` | Login form not found (Cloudflare or render issue) |
| `99_login_failed.png` | Login credentials rejected |
| `99_no_submit_form.png` | Submit form not found |

Download them to inspect visually:
```bash
# From your LOCAL machine:
scp -r root@YOUR_VPS_IP:/tmp/cf-screenshots/ /tmp/
```

#### Run a Manual Browser Test

To test if CloakBrowser can log into Codeforces directly (bypassing the Go backend):

```bash
DISPLAY=:99 python3 -c "
import time
from cloakbrowser import launch_persistent_context
kw = dict(headless=False, humanize=True, args=['--no-sandbox','--disable-dev-shm-usage'])
ctx = launch_persistent_context('/tmp/cb-profile', **kw)
page = ctx.pages[0]
page.goto('https://codeforces.com/enter', timeout=60000)
time.sleep(5)
print('Page title:', page.title())
print('Login form found:', page.locator('#handleOrEmail').count() > 0)
ctx.close()
"
```

### Proxy Format

`CF_PROXY` accepts HTTP and SOCKS5 proxies:

```
CF_PROXY=http://user:pass@host:port
CF_PROXY=socks5://user:pass@host:port
```

When `CF_PROXY` is set, `geoip=True` is automatically enabled — CloakBrowser will match the browser's timezone and locale to the proxy's exit IP, making the session look more natural. This requires `cloakbrowser[geoip]` to be installed.

### macOS (Local Development)

Run the service directly on your Mac so CloakBrowser uses your residential IP:

```bash
pip install "cloakbrowser[geoip]" fastapi uvicorn requests

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
        <string>/opt/aioj/deploy/cf-submit/server.py</string>
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

launchctl load ~/Library/LaunchAgents/com.aioj.cf-submit.plist
```

Make sure `CF_SUBMIT_URL` is set to `http://host.docker.internal:8003` (the default).
