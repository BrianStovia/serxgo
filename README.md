# 🪐 SearXGo - Privacy Metasearch Engine in Golang

> **SearXGo** is an ultra-fast, standalone, zero-telemetry privacy metasearch engine written in **Golang**, providing **100% feature parity with SearXNG** (`searxng/searxng`) and `priv.au`.

---

## ✨ Key Features & Parity Matrix

| Feature | Official SearXNG (Python) | SearXGo (Golang) |
| :--- | :---: | :---: |
| **Engines Catalog** | 260 Engines across 9 categories | **260 Engines (100% Complete)** |
| **Categories** | `general`, `images`, `videos`, `news`, `music`, `it`, `science`, `files`, `social`, `maps` | **All 10 Categories Supported** |
| **Query Bangs** | 499 Bangs (`!gh`, `!yt`, `!ddg`, `!osm`) | **499 Bangs + Direct Redirects (`!yt!`)** |
| **Query Syntax** | `:lang`, `<timeout`, `-!exclude`, `!~exclude` | **100% Exact Syntax Parsed** |
| **Ranking Algorithm** | Exact Reciprocal Rank Formula | **Exact Reciprocal Rank Formula** |
| **Deduplication** | URL normalization + Tracking Stripper | **ResultContainer Tracking Stripper** |
| **Plugins** | DOI resolver, Privacy Rewriters, Tor check | **All Plugins Built-in** |
| **Instant Answers** | Calculator, Weather, Currency, Hashes, QR | **Full Instant Tools Suite** |
| **Interactive Maps** | OpenStreetMap + Leaflet.js | **OpenStreetMap + Leaflet.js** |
| **Output Formats** | HTML, JSON, RSS 2.0, CSV, OpenSearch XML | **HTML, JSON, RSS 2.0, CSV, OpenSearch** |
| **Autocompleter** | RFC OpenSearch JSON Format | **`/autocompleter` & `/api/suggest`** |
| **Deployment** | Python WSGI + Redis requirement | **Standalone Single Binary (Zero DB)** |

---

## 🚀 Quickstart

### 1. Run from Source
```bash
git clone https://github.com/searxng/searxng.git # or local repo
cd searxng
go run ./cmd/server -port 8184
```

### 2. Build Single Standalone Binary

**For Current OS (Windows / Linux / macOS):**
```bash
go build -ldflags="-s -w" -o searxgo ./cmd/server
./searxgo -port 8184
```

**Cross-Compile for Linux (AMD64 / ARM64 Server):**
```bash
# Linux x86_64 / amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o searxgo-linux-amd64 ./cmd/server

# Linux ARM64 / Raspberry Pi / AWS Graviton / Apple Silicon Server
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o searxgo-linux-arm64 ./cmd/server

# Run on Linux server:
chmod +x searxgo-linux-amd64
./searxgo-linux-amd64 -port 8184
```

### 3. Run with Docker / Docker Compose
```bash
docker-compose up -d
```

Open your browser at **`http://localhost:8184`**.

---

## 🔍 Search Bangs & Syntax Cheatsheet

SearXGo supports the entire query syntax of SearXNG:

- **Category Bangs**:
  - `!images <query>` or `!videos <query>` or `!news <query>`
  - `!it <query>` or `!science <query>` or `!files <query>` or `!music <query>` or `!maps <query>`
- **Engine Bangs**:
  - `!gh kubernetes` ➔ Search GitHub repositories
  - `!yt lo-fi beats` ➔ Search YouTube videos
  - `!so memory leak` ➔ Search StackOverflow
  - `!mar retro computing` ➔ Search Marginalia blog engine
  - `!arx quantum telemetry` ➔ Search arXiv preprints
- **Direct Redirects**:
  - `!yt! lo fi music` ➔ Immediately jumps directly to YouTube's search page
  - `!gh! golang` ➔ Immediately jumps directly to GitHub's search page
- **Language Modifiers**:
  - `quantum computing :id` ➔ Filter Indonesian language
  - `machine learning :de` ➔ Filter German language
- **Timeout Modifiers**:
  - `data structures <2s` ➔ Enforce a 2-second timeout
  - `search query <850ms` ➔ Enforce a strict 850ms timeout
- **Engine Exclusions**:
  - `linux -!google` ➔ Run all category engines except Google
  - `coding !~bing` ➔ Exclude Bing from aggregation

---

## 🌐 API & Integration Endpoints

- **HTML Search Interface**: `GET /search?q={query}&category={cat}&time_range={time}`
- **REST JSON API**: `GET /api/search?q={query}` or `GET /search?q={query}&format=json`
- **RSS 2.0 Feed**: `GET /search?q={query}&format=rss`
- **CSV Data Export**: `GET /search?q={query}&format=csv`
- **OpenSearch Autocompleter**: `GET /autocompleter?q={prefix}`
- **Engine Directory JSON**: `GET /engine_descriptions.json` or `GET /engines`
- **Instance Configuration**: `GET /config`
- **Instance Checker Status**: `GET /status.json` and `GET /status`
- **Health Check**: `GET /healthz`
- **Robots Directives**: `GET /robots.txt`
- **SSRF-Safe Image Proxy**: `GET /proxy/image?url={encoded_url}` or `GET /image_proxy?url={encoded_url}`

---

## 🛡️ Privacy & Security Design

1. **Zero Server Logging**: No query logs, IP logs, or session tokens are stored on disk or memory.
2. **SSRF-Safe Anonymous Image Proxy**: Strips upstream headers and blocks loopback/LAN private IP requests.
3. **Tracking Parameter Stripper**: Automatically strips 20+ surveillance trackers (`utm_*`, `fbclid`, `gclid`, `msclkid`, `igshid`, `ref`, etc.) before presenting results.
4. **Privacy Frontend Rewriters**:
   - `YouTube` ➔ `Invidious` (`yewtu.be`)
   - `Reddit` ➔ `Libreddit` (`libreddit.kavin.rocks`)
   - `Twitter/X` ➔ `Nitter` (`nitter.net`)
   - `Medium` ➔ `Scribe` (`scribe.rip`)
   - `Imgur` ➔ `Rimgo` (`rimgo.totaldarkness.net`)
   - Wayback Machine snapshot links attached to every search result.
5. **Open Access DOI Resolver**: Automatically injects direct open-access scientific publication links via `oadoi.org`, `sci-hub.se`, `unpaywall.org`, and `libgen.is`.

---

## 📄 License
AGPL-3.0 License — Identical to the official SearXNG project.
