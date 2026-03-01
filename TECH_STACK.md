# heCsTackForse — Tech Stack

## Backend

| Layer | Technology | Notes |
|-------|-----------|-------|
| Language | [Go 1.21+](https://go.dev/) | Compiled, type-safe, high concurrency |
| HTTP Router | [go-chi/chi v5](https://github.com/go-chi/chi) | Lightweight, idiomatic router with middleware support |
| Auth | [golang-jwt/jwt v5](https://github.com/golang-jwt/jwt) | JWT tokens stored in HttpOnly cookies |
| Passwords | [golang.org/x/crypto (bcrypt)](https://pkg.go.dev/golang.org/x/crypto/bcrypt) | Industry-standard password hashing |
| Database | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | Pure-Go SQLite — zero CGO, single binary |
| Database | [lib/pq](https://github.com/lib/pq) | PostgreSQL driver — production-grade, used via `database/sql` |
| Config | [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) | YAML configuration files |

## Frontend

| Layer | Technology | Notes |
|-------|-----------|-------|
| UI | Vanilla HTML5 / CSS3 / JavaScript | Zero build-step, no framework dependencies |
| HTTP | `fetch()` API | Built-in browser API for REST calls |
| Theme | Custom dark theme | CTF-style dark UI in pure CSS |

## Deployment

| Tool | Purpose |
|------|---------|
| [Docker](https://docs.docker.com/) | Containerise the application (multi-stage Alpine build) |
| [Docker Compose](https://docs.docker.com/compose/) | One-command local/server spin-up (`compose.yaml`) |
| [PostgreSQL 16](https://www.postgresql.org/) | Production database (default in compose) |

## Deployment Backends

| Backend | Technology | Use Case |
|---------|-----------|---------|
| Docker | Docker CLI (via `os/exec`) | Container-based challenges (CTF, Attack & Defence) |
| Kubernetes | `kubectl` (via `os/exec`) | Scalable container orchestration for large events |
| Proxmox VE | [Proxmox REST API](https://pve.proxmox.com/wiki/Proxmox_VE_API) | VM-based challenges (HackTheBox style) |

## Challenge / Deploy Types

| Type | Description |
|------|-------------|
| `no_deploy` | Static challenge — flag is a fixed string, no infrastructure |
| `single_instance` | One shared container/VM for all participants |
| `per_user` | Isolated container/VM per user with configurable TTL |
| `attack_defence` | Attack & Defence: per-team instances, automatic flag rotation, flag check scoring |

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        Frontend (HTML/JS)                   │
│     Scoreboard │ Challenges │ Profile │ Admin               │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTP REST (JSON)
┌──────────────────────────▼──────────────────────────────────┐
│                     REST API (Go / chi)                      │
│  /api/auth  │  /api/challenges  │  /api/scoreboard          │
│  JWT middleware  │  Rate limiter  │  Admin guard             │
└──────┬──────────────────────────────────┬───────────────────┘
       │                                  │
┌──────▼──────┐                  ┌────────▼────────┐
│  SQLite DB  │                  │ Deployer Manager │
│  (modernc)  │                  │  Plugin System   │
└─────────────┘                  └────────┬─────────┘
                                          │
               ┌──────────────────────────┼───────────────────┐
               ▼                          ▼                   ▼
          Docker CLI               kubectl CLI         Proxmox REST API
```

## Data Storage

All persistent state lives in a single SQLite database file (default: `./ctf.db`):

| Table | Purpose |
|-------|---------|
| `users` | Accounts, roles, scores |
| `teams` | Teams with invite codes |
| `team_members` | User ↔ Team membership |
| `challenges` | Challenge definitions (all types) |
| `submissions` | All flag submission attempts |
| `instances` | Running container/VM instances |
| `ad_flags` | Attack & Defence flag store per round |
| `ad_services` | AD service availability/scoring per round |

## Security Design

- Passwords hashed with **bcrypt** (cost 12)
- JWT tokens in **HttpOnly + SameSite=Strict cookies** (XSS-safe, no localStorage)
- **Rate limiting** on flag submissions (per-user, configurable)
- Admin actions protected by **role-based middleware**
- Secrets (JWT key, PVE token) loaded from **config file or env vars**
- HTTPS recommended via reverse proxy (nginx / Caddy)

## Running

### Docker Compose (recommended)

```bash
# 1. Edit the config (change secret_key at minimum)
#    docker/config.yaml is already wired to PostgreSQL inside compose
nano docker/config.yaml

# 2. Start everything (builds the image, starts PostgreSQL, then the app)
docker compose up -d

# 3. Open http://localhost:8080
#    First registered user is automatically made admin.

# Logs
docker compose logs -f app

# Stop
docker compose down
```

### Local (SQLite, no Docker)

```bash
# Copy example config
cp config.example.yaml config.yaml

# Edit config.yaml as needed, then run:
go run ./cmd/server config.yaml

# Or build a binary:
go build -o heCsTackForse ./cmd/server
./heCsTackForse config.yaml
```

### PostgreSQL without Docker

```bash
# Create database
createdb ctf

# In config.yaml:
# database:
#   driver: postgres
#   dsn: "postgres://user:pass@localhost:5432/ctf?sslmode=disable"

go run ./cmd/server config.yaml
```
