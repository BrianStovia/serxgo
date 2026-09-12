# 🪐 SearXGo (serxgo)

<div align="center">

![SearXGo Logo](https://raw.githubusercontent.com/BrianStovia/serxgo/main/web/static/manifest.webmanifest)

[![Go Report Card](https://goreportcard.com/badge/github.com/BrianStovia/serxgo)](https://goreportcard.com/report/github.com/BrianStovia/serxgo)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![Docker Pulls](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](Dockerfile)
[![Engines](https://img.shields.io/badge/Engines-260+-brightgreen)](internal/engine)
[![Port](https://img.shields.io/badge/Default%20Port-8184-orange)](settings.yml)

**Ultra-fast, hardware-agnostic, privacy-respecting metasearch aggregator written in pure Golang.**  
*100% feature parity with SearXNG, zero telemetry, zero database dependencies, and sub-second multi-engine aggregation.*

[✨ Features](#-key-features) • [🚀 Quickstart](#-quickstart) • [🔍 Search Syntax & Bangs](#-search-bangs--syntax-cheatsheet) • [🌐 REST API](#-api--integration-endpoints) • [⚙️ Configuration](#-configuration) • [🛡️ Privacy Architecture](#-privacy--security-architecture)

</div>

---

## 🌟 Key Features

- ⚡ **Pure Golang & Single Static Binary**: Zero external runtime dependencies (no Python, no Redis, no Node.js). Consumes <30MB RAM idle with instant sub-second startup.
- 🚀 **260+ Search Engines Across 9 Categories**: Native scrapers and API integrations covering *General, Images, Videos, IT & Code, Science & Academic, News, Social, Files & Torrents, Music, and Maps*.
- 🧠 **Smart Semantic Topic Clustering**: Automatically classifies search results into dynamic topics (*Docs & Specs, Code & Repos, Discussions, Media, Research, News, Tools*) with zero-latency client-side filtering.
- 🛡️ **Zero-Censorship & Unfiltered Defaults**: Default `SafeSearch=Off` for unfiltered upstream aggregation without keyword filtering or censorship.
- 📱 **Progressive Web App (PWA) & Offline Mode**: Installable on Android, iOS, Windows, and macOS with Service Worker offline caching.
- 🔖 **Private Client-side Bookmarks & Workspace**: Save searches and pin results directly to browser `localStorage` with JSON export and zero server footprint.
- 🏎️ **SSRF-Safe Image Proxy**: Built-in HMAC SHA-256 authenticated image and thumbnail caching proxy that shields user IP addresses from upstream CDNs.
- 🧹 **Surveillance Tracker Stripper**: Automatically purges UTM tags, Facebook click IDs (`fbclid`), Google click IDs (`gclid`), and tracking parameters.
- 📐 **Instant Answer Engine**: Built-in math evaluator, currency converter, live weather, hash generators, Base64 encoder/decoder, and interactive Leaflet / OpenStreetMap maps.
- 📊 **Real-time Telemetry & Prometheus Metrics**: Live engine response times, reliability scoring, and OpenMetrics at `/stats` and `/metrics`.
- 🔌 **Comprehensive Export & Feeds**: JSON REST API, RSS 2.0 feeds, CSV export, and RFC OpenSearch XML autocompleter.

---

## 📊 SearXNG vs SearXGo Parity Matrix

| Feature / Metric | Official SearXNG (Python) | SearXGo (Golang) |
| :--- | :---: | :---: |
| **Runtime & Dependencies** | Python 3.10+ + Redis + UWSGI | **Single Static Binary (Zero DB)** |
| **Memory Footprint (Idle)** | ~180MB - 350MB | **< 30MB RAM** |
| **Engine Catalog** | 260 Engines across 9 categories | **260 Engines (100% Parity)** |
| **Query Bangs (`!gh`, `!yt`)** | 499 Bangs | **499 Bangs + Direct Redirects (`!yt!`)** |
| **Smart Topic Clustering** | ❌ Not available | ** Dynamic 8-Cluster Engine** |
| **PWA & Offline Installation** | Partial | ** Full Web Manifest + Service Worker** |
| **Private Client Workspace** | ❌ Not available | ** LocalStorage Bookmarks + Export** |
| **Ranking & Deduplication** | Reciprocal Rank Algorithm | ** Reciprocal Rank + Score Normalizer** |
| **Cross-Platform Compilation** | Complex virtualenv setup | ** Linux (x86/ARM), Windows, macOS** |

---

## 🚀 Quickstart

### ⚡ 1-Line Automated Installer

**Linux & macOS (Systemd + Service Auto-Setup):**
```bash
curl -fsSL https://raw.githubusercontent.com/BrianStovia/serxgo/main/install.sh | bash
```

**Windows (PowerShell Auto-Setup & PATH Configuration):**
```powershell
irm https://raw.githubusercontent.com/BrianStovia/serxgo/main/install.ps1 | iex
```

### 🔄 1-Line Automated Updater

**Linux & macOS (Auto-Update & Service Reload):**
```bash
curl -fsSL https://raw.githubusercontent.com/BrianStovia/serxgo/main/update.sh | bash
```

**Windows (PowerShell Auto-Update):**
```powershell
irm https://raw.githubusercontent.com/BrianStovia/serxgo/main/update.ps1 | iex
```

---

### 1. Run from Precompiled Binaries

Download the latest binary for your architecture from [Releases](https://github.com/BrianStovia/serxgo/releases):

```bash
# Linux x86_64 / amd64
chmod +x ./searxgo-linux-amd64
./searxgo-linux-amd64 -port 8184

# Linux ARM64 / Raspberry Pi / AWS Graviton
chmod +x ./searxgo-linux-arm64
./searxgo-linux-arm64 -port 8184

# Windows x86_64
.\searxgo.exe -port 8184
```

Access the web interface at **`http://localhost:8184`**.

---

### 2. Build from Source

Ensure you have [Go 1.22+](https://go.dev/dl/) installed:

```bash
git clone https://github.com/BrianStovia/serxgo.git
cd serxgo

# Build for current OS
go build -ldflags="-s -w" -o searxgo ./cmd/server
./searxgo -port 8184
```

---

### 3. Cross-Compile for Linux Servers

```bash
# Linux x86_64 / amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o searxgo-linux-amd64 ./cmd/server

# Linux ARM64 / Apple Silicon / Graviton Server
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o searxgo-linux-arm64 ./cmd/server
```

Or simply use the `Makefile`:
```bash
make build-all
```

---

### 4. Run with Docker / Docker Compose

```bash
# Start container in background
docker-compose up -d

# View logs
docker-compose logs -f
```

---

## 🔍 Search Bangs & Syntax Cheatsheet

SearXGo supports complete SearXNG advanced search query syntax:

### 1. Category Bangs
| Bang | Category | Example |
| :--- | :--- | :--- |
| `!images` or `!img` | Image search | `!images mountain sunset` |
| `!videos` or `!vid` | Video search | `!videos golang concurrency` |
| `!news` | News articles | `!news space exploration` |
| `!it` | Developer & IT code | `!it linux kernel memory` |
| `!science` | Academic & Preprints | `!science quantum entanglement` |
| `!files` | Files & Torrents | `!files open source linux iso` |
| `!social` | Social discussions | `!social decentralized social network` |
| `!music` | Music & Lyrics | `!music synthwave chill` |
| `!maps` | OpenStreetMap POI | `!maps tokyo tower` |

### 2. Engine Bangs & Direct Redirects
- `!qw privacy browser` ➔ Search Qwant (European index)
- `!sp zero knowledge` ➔ Search Startpage (Google-grade results)
- `!eco reforestation` ➔ Search Ecosia (Green search)
- `!mo independent web` ➔ Search Mojeek (Independent crawler)
- `!wa speed of light` ➔ Compute with Wolfram|Alpha (Science/Math facts)
- `!gh kubernetes` ➔ Search GitHub repositories
- `!yt lofi hip hop` ➔ Search YouTube videos
- `!so goroutine leak` ➔ Search StackOverflow
- `!arx gravitational waves` ➔ Search arXiv papers
- `!ddg privacy tools` ➔ Search DuckDuckGo
- `!qw! privacy` ➔ **Direct Jump:** Immediately navigates to `https://www.qwant.com/?q=privacy`
- `!sp! security` ➔ **Direct Jump:** Immediately navigates to `https://www.startpage.com/sp/search?query=security`
- `!gh! golang` ➔ **Direct Jump:** Immediately navigates to `https://github.com/search?q=golang`
- `!yt! synthwave` ➔ **Direct Jump:** Immediately navigates to `https://www.youtube.com/results?search_query=synthwave`

### 3. Advanced Operators, Booleans & Modifiers
- **Boolean Logic**: `golang AND concurrency NOT rust` or `database OR storage +performance -slow`
- **Date Ranges**: `after:2024-01-01`, `before:2024-12-31`, `since:2023`, `year:2024`
- **Regional & Language**: `country:id`, `region:id-id`, `lang:id`, `:id` (Indonesian), `:de` (German), `:en` (English)
- **Time Range**: Filter via UI or parameter `&time_range=day|week|month|year`
- **Results per Page**: `&page_size=25` or `&count=30` or `&limit=50`
- **Timeout Enforcer**: `distributed systems <1.5s` (Force 1.5s timeout)
- **Engine Exclusion**: `linux kernel -!google` or `coding !~bing`

---

## 🌐 API & Integration Endpoints

| Endpoint | Method | Description | Output Format |
| :--- | :---: | :--- | :--- |
| `/search` | `GET` | Web search interface & query processor | `HTML` |
| `/api/search` | `GET` | REST API search endpoint | `application/json` |
| `/search?format=json` | `GET` | SearXNG compliant JSON export | `application/json` |
| `/search?format=rss` | `GET` | RSS 2.0 query feed | `application/rss+xml` |
| `/search?format=csv` | `GET` | CSV tabular data export | `text/csv` |
| `/autocompleter` | `GET` | RFC OpenSearch JSON autocomplete | `application/json` |
| `/api/suggest` | `GET` | Lightweight search bar suggestion API | `application/json` |
| `/proxy/image` | `GET` | SSRF-safe anonymous image proxy | Image binary |
| `/stats` | `GET` | Engine telemetry dashboard | `HTML` |
| `/metrics` | `GET` | Prometheus OpenMetrics exporter | `text/plain` |
| `/healthz` | `GET` | Healthcheck endpoint | `application/json` |
| `/opensearch.xml` | `GET` | Browser search engine provider descriptor | `application/opensearchdescription+xml` |
| `/manifest.webmanifest` | `GET` | Progressive Web App manifest | `application/manifest+json` |

---

## ⚙️ Configuration

SearXGo can be configured via [`settings.yml`](settings.yml), CLI arguments, or environment variables:

| Environment Variable | CLI Flag | Default | Description |
| :--- | :--- | :---: | :--- |
| `SEARXNG_PORT` / `PORT` | `-port` | `8184` | HTTP server listening port |
| `SEARXNG_BIND_ADDRESS` | `-host` | `0.0.0.0` | Network binding interface |
| `SEARXNG_TIMEOUT_MS` | `-timeout` | `3500` | Aggregation timeout in milliseconds |
| `SEARXNG_PAGE_SIZE` / `PAGE_SIZE` | - | `20` | Default results count per page (max 50) |
| `SEARXNG_SECRET_KEY` | - | `searxgo-secret-key...` | HMAC secret for image proxy |
| `SEARXNG_DEBUG` | - | `false` | Enable verbose debug logging |
| `SEARXNG_LIMITER` | - | `false` | Enable rate limiting (Unlimited by default) |

---

## 🛡️ Privacy & Security Architecture

1. **Zero Logging Guarantee**: SearXGo does not log search queries, client IP addresses, user-agents, or timestamps to disk or stdout.
2. **SSRF-Safe Image Proxy**: Upstream image requests validate DNS resolution to prohibit access to private IPv4/IPv6 ranges (`127.0.0.0/8`, `10.0.0.0/8`, `192.168.0.0/16`, `172.16.0.0/12`, `::1`).
3. **Surveillance Parameter Stripping**: Clean URLs before rendering by scrubbing tracking tokens (`utm_*`, `fbclid`, `gclid`, `msclkid`, `igshid`, `yclid`, `mc_eid`).
4. **Privacy Alternative Frontends**: Automatically rewrites commercial URLs to privacy-respecting open-source frontends (*Invidious, Libreddit, Nitter, Scribe, Rimgo*).
5. **No Database & Ephemeral State**: User settings and bookmarks reside exclusively inside the user's browser `localStorage`.

---

## 📄 License

Distributed under the **MIT License**. See [`LICENSE`](LICENSE) for more information.

---

<div align="center">
Built with ❤️ for Privacy, Speed, and Freedom.
</div>
