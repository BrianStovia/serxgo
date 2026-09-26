# 📜 Changelog

All notable changes to **SearXGo** (`serxgo`) are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [v1.3.0] - 2026-09-27

### 🚀 Major New Capabilities
- 🔍 **All-in-One Domain Recon & Security Auditor (`/recon`)**:
  - Full domain security inspection suite with SSL/TLS certificate validity checks and expiry countdown.
  - HTTP Security Headers compliance evaluation with automated letter grade scoring (`A+` to `F`).
  - Authoritative DNS resolution (A, AAAA, MX, NS, TXT) with SPF and DMARC mail protection verification.
  - Web server fingerprinting and technology stack detection (Nginx, Apache, Cloudflare, WordPress, Next.js, etc.).
  - Search query instant trigger (`recon: example.com`, `audit example.com`).
- 🎬 **Ad-Free Floating Video & Music Mini-Player**:
  - Picture-in-Picture floating media player with minimize, expand, and close states.
  - Supports privacy-focused YouTube embeds (`youtube-nocookie.com`), Vimeo, and Dailymotion.
  - 1-click `▶️ Play` button added to video and media search results.
- 📖 **Distraction-Free Clean Reader Mode (`/reader?url=...`)**:
  - Pure Go article extractor using `golang.org/x/net/html` that parses main content and metadata (reading time, author, title).
  - Sanitizes and purges tracking scripts, inline ads, and cookie banners.
  - Modern reading UI with light/dark/sepia theme toggles, serif/sans typography, font size controls, and reading progress bar.
- 🧰 **Developer Instant Power Cards**:
  - **JWT Decoder & Inspector**: Validates token format, decodes header & claims payloads, checks expiration timestamps.
  - **Cron Schedule Explainer**: Translates 5-field cron expressions into human-readable schedules with next run approximations.
  - **Linux Chmod Permission Calculator**: Converts octal permissions (`755`, `644`) to symbolic (`rwxr-xr-x`) and breakdowns for Owner, Group, and Public.
  - **Go Regex Syntax Validator**: Tests and validates regular expressions on-the-fly with group/class breakdowns.
- 🎯 **Search Relevance & Precision Overhaul**:
  - **Dynamic Localization & Region Detection**: DuckDuckGo, Bing, Brave, and Wikipedia automatically adapt region (`kl=id-id`, `mkt=id-ID`, `Accept-Language: id-ID`) when querying in Indonesian.
  - **Disabled Artificial Cross-Language Translation**: Natural user queries are searched as-is without inserting distorted English synonyms.
  - **Eliminated False-Positive Breach Dumps**: General everyday words (like "bocor", "masalah") no longer trigger security ransomware/leak scrapers.
  - **Authentic Domain & Snippet Ranking Boost**: Upgraded scoring algorithm with +35.0 boost for authentic local domains (`.id`), +25.0 boost for language matching, and higher weight on snippet density over keyword-stuffed titles.

## [v1.0.0] - 2026-09-05

### 🚀 Major Highlights
- **100% Feature Parity with SearXNG in Pure Golang**:
  - Full implementation of 260+ search engines across 9 core categories: *General, Images, Videos, IT & Code, Science & Academic, News, Social, Files & Torrents, Music, and Maps*.
  - Single standalone binary compilation with zero runtime database or interpreter dependencies (no Python, no Redis).
  - Idle memory footprint reduced to `<30MB RAM` with instantaneous sub-millisecond route dispatching.

### 🧠 Smart Semantic Topic Clustering
- **Zero-Latency Client-side Topic Filtering**:
  - Dynamic classification of search results into 8 semantic clusters:
    - 📚 **Docs & Specs**: Official documentation, API references, specifications.
    - 💻 **Code & Repos**: GitHub, GitLab, Codeberg, Crates, NPM, PyPI.
    - 💬 **Discussions**: StackOverflow, Reddit, Hacker News, discourse forums.
    - 🎬 **Media & Video**: YouTube, Vimeo, Bilibili, Dailymotion, audio streams.
    - 🔬 **Research Papers**: arXiv, PubMed, ScienceDirect, Nature, IEEE, Springer.
    - 📰 **News & Articles**: Reuters, BBC, The Guardian, TechCrunch, Verge, Dev.to.
    - 📦 **Tools & Downloads**: Binaries, CLI tools, Docker Hub, package managers.
    - 🌐 **All Results**: Unfiltered view of all aggregated results.
  - Interactive topic pills bar with instant click-to-filter event delegation.

### 🛡️ Privacy & Security Architecture
- **Uncensored & Unfiltered Defaults**:
  - Default `SafeSearch=Off` (0) across all upstream aggregators for zero keyword filtering and authentic, uncensored search results.
- **SSRF-Safe HMAC Image Proxy**:
  - Built-in authenticated image proxy (`/proxy/image`) protecting client IPs with private IP range blocking (`127.0.0.0/8`, `10.0.0.0/8`, `192.168.0.0/16`, `172.16.0.0/12`).
- **Surveillance Parameter Stripping**:
  - Automatic scrubbing of over 20 tracking parameters (`utm_*`, `fbclid`, `gclid`, `msclkid`, `igshid`, `yclid`, `mc_eid`).
- **Privacy Frontend Rewriting**:
  - Option to rewrite URLs to Invidious, Libreddit, Nitter, Scribe, and Rimgo with Wayback Machine cached snapshot links.

### 📱 Progressive Web App (PWA) & Private Workspace
- **PWA Ready**:
  - Offline Service Worker (`web/static/sw.js`) with Network-First caching strategy.
  - Web App Manifest (`web/static/manifest.webmanifest`) supporting standalone app installation across Android, iOS, Windows, and macOS.
- **Private Local Bookmarks & Saved Searches**:
  - 100% client-side bookmarks workspace stored in `localStorage`.
  - Live badge counter in header, search pin toggles, and JSON export.

### 📐 Instant Answer & Multi-format API
- **Instant Answers Suite**:
  - Mathematical expression evaluator (`calculator`).
  - Real-time currency exchange rates.
  - Live weather forecast cards.
  - Cryptographic hash generators (MD5, SHA-1, SHA-256, SHA-512) and Base64 encoder/decoder.
  - Interactive Leaflet.js map integration for OpenStreetMap geolocation searches.
- **REST API & Telemetry**:
  - Standard JSON API at `/api/search` and `/search?format=json`.
  - RSS 2.0 query feeds at `/search?format=rss`.
  - CSV tabular data export at `/search?format=csv`.
  - RFC OpenSearch autocompleter at `/autocompleter` and `/api/suggest`.
  - Real-time engine telemetry dashboard at `/stats` and Prometheus OpenMetrics exporter at `/metrics`.

### 🏗️ Build & Deployment
- **Port 8184 Default**:
  - Default HTTP port set to `8184` across configuration, binary defaults, Dockerfile, and docker-compose.
- **Cross-Platform Compilation Matrix**:
  - Statically compiled Linux binaries for `amd64` and `arm64` via `Makefile` (`make build-all`).
  - Windows standalone executable (`searxgo.exe`).
  - Production-ready multi-stage `Dockerfile` and `docker-compose.yml`.

---

[v1.0.0]: https://github.com/BrianStovia/serxgo/releases/tag/v1.0.0
