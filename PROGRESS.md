# AI Code Battle - Implementation Progress

## Current Phase: Phase 12 - Deep Plan Gap Closure

**Status: ✅ Complete**

**Last Updated: 2026-04-23** (Phase 12 gap verification — all items already implemented)

### Phase 12 Gap Verification (2026-04-23)
Second-pass gap review found all 16 listed items already implemented in prior phases:

| Gap Item | Status | Location |
|----------|--------|----------|
| §4.4 HMAC strict verification | ✅ | `engine/bot_http.go` — strict sig check, no lenient fallback |
| §3.3 Deterministic spawn priority | ✅ | `engine/turn.go` — `LastSpawnedTurn` tracking, `sortCoresByPriority()` |
| §4.2 Config forward-compat fields | ✅ | `engine/types.go` — `SeasonID`, `RulesVersion` in Config struct |
| §4.5 Multi-match crash cooldown | ✅ | `cmd/acb-worker/db.go` — 3 strikes, 30-min cooldown |
| §7.1 Gzip replay upload | ✅ | `cmd/acb-worker/main.go` — gzip + B2 upload |
| §7.3 Fog-of-war toggle + minimap | ✅ | `web/src/replay-viewer.ts` — `fogOfWarPlayer`, minimap canvas |
| §10.2 Island cross-pollination | ✅ | `cmd/acb-evolver/internal/crosspoll/` — every 50 generations |
| §10.2 MAP-Elites 4-D grid | ✅ | `cmd/acb-evolver/internal/mapelites/` — 4 dims, 81 cells |
| §9.2 K8s manifests | ✅ | `manifests/acb-evolver-deployment.yml`, `acb-api-deployment.yml` |
| §9.9 Prometheus metrics + alerts | ✅ | `manifests/acb-metrics-monitoring.yml` — 9 alert rules |
| §13.5 Rivalry detection | ✅ | `cmd/acb-index-builder/generator.go` — `computeRivalries()` |
| §13.6 /api/feedback naming | ✅ | `cmd/acb-api/server.go` — `/api/feedback`, test rejects old name |
| §14.1 Debug telemetry visibility | ✅ | `debug_public` column, toggle endpoint, result filtering |
| §15.1/§15.5 LLM prompt alignment | ✅ | `cmd/acb-evolver/internal/prompt/builder.go` — Nash + meta weaknesses |
| §15.2 Static JSON meta files | ✅ | Index builder generates archetypes, rivalries, community_hints |

Verification: `go test ./...` pass, `go vet ./...` clean, `npm run build` succeeds.

## Phase 10 - Ecosystem & Polish

**Status: ✅ Complete**

### Series & Season Scheduler Verification (2026-04-21)
Verified that series scheduling and seasonal ELO reset (§11) are fully implemented:
- **Series scheduler** (`cmd/acb-matchmaker/series_season.go`, 970 lines): 5-step pipeline — propagate match results, finalize completed series, schedule next games with map variety and slot alternation, auto-create series for top-20 bots, advance championship bracket
- **Seasonal ELO reset**: End-of-season detection via `ends_at`, ELO snapshot into `season_snapshots`, decay formula `new_mu = 1500 + (mu-1500)*factor`, championship bracket for top 8, auto-start 28-day seasons with cycling themes
- **Series bracket display** (`web/src/pages/series.ts`, 807 lines): bracket progress dots with connectors, bracket tree visualization, game-by-game results with map type labels, spoiler toggle, championship round badges
- **Season leaderboard** (`web/src/pages/seasons.ts`, 819 lines): per-season rankings table with win rate bars, active season progress bar, mini-leaderboard from live data, championship bracket visualization
- **Tests** (`series_season_test.go`, 628 lines): all 27 tests passing covering decay formula, bracket seeding, format selection, draw handling, all-played finalization, scheduler ordering
- `go vet ./...` clean, `go test ./...` all pass, `npm run build` succeeds

### Legacy Code Cleanup (2026-03-29)
Removed superseded code that no longer matches the architecture:
- **Removed `worker-api/`**: Cloudflare Worker with D1, superseded by K8s-based matchmaker + direct PostgreSQL
- **Removed `cmd/acb-indexer/`**: TypeScript index builder, superseded by Go `cmd/acb-index-builder/`
- **Removed `deploy/k8s/`**: Old K8s manifest location (already migrated to ardenone-cluster repo)
- **Removed `cluster-configuration/`**: K8s manifests belong in ardenone-cluster repo at `declarative-config/k8s/apexalgo-iad/ai-code-battle/`
- **Gutted `cmd/acb-api/`**: Removed registration, job claim/result endpoints (deferred for v1), removed dead code (predictions.go, seasons.go, series.go, register.go, jobs.go, glicko2.go)
  - API is now a stub with only health/ready endpoints
  - Matchmaker and workers handle the core loop without it

### Marathon Verification (2026-03-29)
- Project verified complete - no remaining work
- Web build: passing
- Git status: clean, up to date with origin/master
- K8s manifests: in ardenone-cluster repo at `declarative-config/k8s/apexalgo-iad/ai-code-battle/`
- cmd packages: 9 present (acb-api stub, acb-evolver, acb-index-builder, acb-local, acb-map-evolver, acb-mapgen, acb-matchmaker, acb-wasm, acb-worker)
- All phases 1-10 complete - project finished

### Phase 10 Completed ✅

- [x] Accessibility suite (`web/src/replay-viewer.ts`, `web/src/app.ts`, `web/app.html`)
  - Paul Tol color-blind safe palette (8 distinct colors)
  - Shapes per player (circle, square, triangle, diamond, pentagon, hexagon)
  - High contrast mode (brighter colors, darker walls)
  - Reduced motion support (auto-detect prefers-reduced-motion)
  - Accessibility controls UI in replay page
  - Keyboard shortcuts: Space (play/pause), ArrowLeft/Right (step), Home/End (start/end)
  - Screen reader region for turn announcements
  - Focus indicators (`:focus-visible` styles) for all interactive elements
  - Skip link for screen reader navigation
  - High contrast focus enhancement (`prefers-contrast: more` media query)
- [x] Weekly meta report blog infrastructure
  - Blog generation module in Go index builder (`cmd/acb-index-builder/blog.go`)
  - Meta report content generation (leaderboard, strategies, rising/falling bots, rivalries)
  - Chronicle generation (rise stories, upset narratives, rivalry chronicles)
  - Blog page component with filtering and post rendering (`web/src/pages/blog.ts`)
  - Individual post page with markdown rendering
  - Blog routes added to SPA router
  - Blog link added to navigation
- [x] Live evolution observatory (evolver writes live.json to R2)
  - R2 client module (`cmd/acb-evolver/internal/live/r2.go`) for S3-compatible uploads
  - `live-export -r2` and `live-export -r2-only` flags for R2 upload
  - Frontend fetches from R2 (`https://r2.aicodebattle.com/evolution/live.json`)
  - Cache-Control: max-age=10 for near-real-time updates
  - Tests for R2 config validation and credential handling
- [x] Narrative engine (weekly story arc detection + LLM chronicles)
- [x] Public match data documentation (OpenAPI-style)
  - New `/docs/api` route with comprehensive endpoint documentation
  - Documents Pages, R2, and B2 static JSON endpoints
  - Includes JSON Schema for replay format
  - Fetching pattern with R2-then-B2 fallback
  - Cache behavior documentation

### Phase 10 Exit Criteria

| Criterion | Status |
|-----------|--------|
| Weekly editorial content (meta reports + story arcs) as blog posts | ✅ Complete |
| All match data exposed as documented static JSON | ✅ Complete |
| WCAG accessibility standards for color and keyboard navigation | ✅ Complete |
| Live evolution observatory streaming | ✅ Complete |

### Phase 9 Completed ✅

- [x] Bot profile cards (`cmd/acb-index-builder/cards.go`, `web/src/og-tags.ts`)
  - Canvas-rendered PNG images (1200x630 for Open Graph)
  - Displays: bot name, rating, win rate, W/L record, rank badge
- [x] Map evolution pipeline (`cmd/acb-map-evolver/`)
  - Parent selection by engagement × vote multiplier
  - Crossover breeding with sector-based inheritance
  - Symmetry-preserving mutation
- [x] Replay playlists (`cmd/acb-index-builder/playlists.go`, `web/src/pages/playlists.ts`)
  - Auto-curated collections: featured, upsets, comebacks, domination
- [x] Embeddable replay widget (`web/embed.html`, `web/src/embed.ts`)
- [x] Multi-game series scheduler and bracket display (`cmd/acb-matchmaker/series_season.go`, `web/src/pages/series.ts`)
  - Series scheduler: auto-creates best-of-N series for top-20 bots, schedules games sequentially
  - Round-robin player slot alternation for fairness
  - Varied map selection per game (engagement, wall density, random)
  - Bracket progress dots, bracket tree visualization, game-by-game results
  - Championship bracket: quarterfinals → semifinals → final for top 8 bots
  - Spoiler toggle for hiding results in series detail view
- [x] Seasonal ELO reset and leaderboard (`cmd/acb-matchmaker/series_season.go`, `web/src/pages/seasons.ts`, `web/src/pages/season-detail.ts`)
  - Season end detection via `ends_at` column
  - ELO snapshot into `season_snapshots` table before reset
  - Decay formula: `new_mu = 1500 + (mu - 1500) * decay_factor` (default 0.7)
  - Auto-starts new 28-day season with cycling themes
  - Per-season leaderboard with rank, rating, wins, losses, win-rate bars
  - Active season display with progress bar and mini-leaderboard
  - Championship bracket visualization on season detail page

### Phase 8 Completed ✅

- [x] WASM game engine (`cmd/acb-wasm/`)
- [x] In-browser sandbox (`web/src/pages/sandbox.ts`)
- [x] Win probability computation (`web/src/win-probability.ts`)
- [x] Replay commentary (`web/src/commentary.ts`)
- [x] Clip maker (`web/src/pages/clip-maker.ts`)
- [x] Rivalry detection (`web/src/pages/rivalries.ts`)
- [x] Replay feedback system (`web/src/pages/feedback.ts`)

### Phase 7 Completed ✅

- [x] Evolution pipeline (`cmd/acb-evolver/`)
  - Programs database with island model (4 islands)
  - MAP-Elites behavior grid integration
  - Validation pipeline: syntax → schema → sandbox smoke test
  - Evaluation arena: 10-match mini-tournament
  - Promotion gate: Nash equilibrium computation + MAP-Elites niche fill
  - Live export: generates live.json for dashboard
- [x] LLM integration (`cmd/acb-evolver/internal/llm/`)

### Phase 6 Completed ✅

- [x] Go API server (`cmd/acb-api/`) — now a stub, full API deferred for v1
- [x] Match worker container (`cmd/acb-worker/Dockerfile`)
- [x] Discord/Slack alerting webhooks (`cmd/acb-api/alerts.go`)
- [x] Prometheus metrics endpoint (`cmd/acb-worker/metrics.go`)
- [x] Argo Workflows CI pipeline (`manifests/acb-build.yml`, `manifests/acb-eventsensor.yml`)
  - acb-build-images: clone → go vet + go test -race → Kaniko build 23 images → push to Forgejo registry
  - acb-build-site: clone → npm ci && npm run build → package as container image → push to Forgejo registry
  - Argo Events: EventBus + EventSource (Forgejo webhook) + Sensor (triggers both workflows on push to master)
  - Index builder pulls latest site build from registry via crane (sitebuild.go)

### Phase 5 Completed ✅

- [x] SPA application shell (`web/app.html`)
- [x] Hash-based router (`web/src/router.ts`)
- [x] Page components (`web/src/pages/`)
- [x] API client (`web/src/api-types.ts`)
- [x] Cloudflare Pages deployment configuration

### Phase 4 Completed ✅

### Phase 3 Completed ✅

### Phase 2 Completed ✅

### Phase 1 Completed ✅

## File Structure

```
ai-code-battle/
├── go.mod
├── go.sum
├── .env.example              # Environment configuration template
├── DEPLOYMENT.md             # Deployment guide
├── docker-compose.bots.yml   # Bot-host orchestration
├── docker-compose.workers.yml # Worker orchestration
├── .github/
│   └── workflows/
│       └── ci.yml.disabled   # GitHub Actions CI (disabled — Argo Workflows is the CI system)
├── manifests/                  # K8s staging manifests (synced to ardenone-cluster repo)
│   ├── acb-build.yml           # Argo WorkflowTemplates: acb-build-images + acb-build-site
│   ├── acb-eventsensor.yml     # Argo Events: EventBus + EventSource + Sensor
│   ├── acb-evolved-bot-deploy-workflowtemplate.yml  # Evolved bot deploy pipeline
│   ├── acb-api-deployment.yml  # API server Deployment + Service + IngressRoute
│   ├── acb-evolver-deployment.yml  # Evolver Deployment
│   └── acb-metrics-monitoring.yml  # ServiceMonitor + PrometheusRule
├── engine/
│   ├── types.go        # Core data types
│   ├── grid.go         # Toroidal grid implementation
│   ├── game.go         # Game state management
│   ├── turn.go         # Turn execution phases
│   ├── replay.go       # Replay recording
│   ├── match.go        # Match runner
│   ├── bot_local.go    # Local bot interface
│   ├── bot_http.go     # HTTP bot client
│   ├── auth.go         # HMAC authentication
│   └── *_test.go       # Test files
├── cmd/
│   ├── acb-api/        # Go API server (stub - deferred for v1)
│   │   ├── main.go      # Server entry point
│   │   ├── server.go    # Route registration (health/ready only)
│   │   ├── config.go    # Environment configuration
│   │   ├── db.go        # PostgreSQL schema
│   │   ├── health.go    # Health/ready endpoints
│   │   ├── crypto.go    # ID generation, AES-256-GCM encryption
│   │   ├── alerts.go    # Discord/Slack alerts
│   │   ├── Dockerfile   # API container
│   │   └── *_test.go    # Test files
│   ├── acb-local/      # CLI match runner
│   ├── acb-mapgen/     # Map generator
│   ├── acb-worker/     # Match execution worker
│   │   ├── main.go      # Worker entry point
│   │   ├── api.go       # Worker API client
│   │   ├── metrics.go   # Prometheus metrics
│   │   ├── b2.go        # B2 upload client
│   │   └── Dockerfile   # Worker container
│   ├── acb-index-builder/  # Go index builder
│   │   ├── main.go
│   │   ├── blog.go      # Blog generation
│   │   ├── cards.go     # OG image generation
│   │   └── playlists.go # Playlist generation
│   ├── acb-evolver/    # Evolution pipeline
│   │   ├── main.go      # CLI entry point
│   │   └── internal/
│   │       ├── db/       # Programs database
│   │       ├── arena/    # Tournament evaluation
│   │       ├── validator/# 3-stage validation
│   │       ├── promoter/ # Promotion gate
│   │       ├── selector/ # Parent sampling
│   │       ├── prompt/   # LLM prompt builder
│   │       ├── llm/      # LLM client
│   │       ├── mapelites/ # Behavior grid
│   │       └── live/     # Dashboard export
│   ├── acb-wasm/       # WASM game engine
│   │   ├── main.go      # JS bindings
│   │   ├── bots.go      # Bot interface
│   │   ├── build.sh     # Build script
│   │   └── strategies/  # Strategy implementations
│   ├── acb-matchmaker/ # Internal matchmaker
│   │   ├── main.go      # Ticker orchestration
│   │   ├── tickers.go   # Pairing, health, reaping
│   │   ├── series_season.go # Series scheduling, seasonal ELO reset, championship bracket
│   │   ├── series_season_test.go # Tests for decay, bracket seeding, finalization
│   │   ├── config.go    # Configuration
│   │   ├── crypto.go    # Shared crypto
│   │   ├── alerts.go    # Discord/Slack alerts
│   │   └── Dockerfile   # Container build
│   └── acb-map-evolver/ # Map evolution pipeline
│       └── main.go      # CLI entry point
├── web/
│   ├── package.json    # npm dependencies
│   ├── tsconfig.json   # TypeScript config
│   ├── vite.config.ts  # Vite bundler config
│   ├── pages.json      # Cloudflare Pages project config
│   ├── index.html      # Standalone replay viewer
│   ├── app.html        # SPA shell with navigation
│   ├── embed.html      # Embeddable replay widget
│   ├── public/         # Static assets (copied to dist/)
│   │   ├── _headers    # Cloudflare cache headers
│   │   ├── robots.txt  # SEO
│   │   └── data/       # Index files
│   └── src/
│       ├── types.ts        # Replay type definitions
│       ├── api-types.ts    # API client and types
│       ├── router.ts       # Hash-based SPA router
│       ├── replay-viewer.ts # Canvas viewer class
│       ├── engine.ts       # Browser game engine
│       ├── commentary.ts   # AI replay commentary
│       ├── win-probability.ts # Monte Carlo win prob
│       ├── og-tags.ts      # Dynamic OG tag updates
│       ├── main.ts         # Standalone replay viewer
│       ├── app.ts          # SPA entry point
│       ├── embed.ts        # Embeddable widget
│       └── pages/          # SPA page components
│           ├── home.ts
│           ├── leaderboard.ts
│           ├── matches.ts
│           ├── bots.ts
│           ├── bot-profile.ts
│           ├── register.ts
│           ├── sandbox.ts
│           ├── evolution.ts
│           ├── clip-maker.ts
│           ├── rivalries.ts
│           ├── feedback.ts
│           ├── playlists.ts
│           ├── blog.ts
│           ├── series.ts
│           ├── seasons.ts
│           ├── season-detail.ts
│           └── docs-api.ts
├── bots/
│   ├── random/         # Python - RandomBot
│   ├── gatherer/       # Go - GathererBot
│   ├── rusher/         # Rust - RusherBot
│   ├── guardian/       # PHP - GuardianBot
│   ├── swarm/          # TypeScript - SwarmBot
│   └── hunter/         # Java - HunterBot
└── docs/
    └── plan/
        └── plan.md     # Full implementation plan
```

**Note:** K8s manifests are in the ardenone-cluster repo at `declarative-config/k8s/apexalgo-iad/ai-code-battle/`

## Strategy Bot Summary

| Bot | Language | Strategy | Expected Rank |
|-----|----------|----------|---------------|
| RandomBot | Python | Random valid moves | 6th (floor) |
| GathererBot | Go | Energy collection, avoid combat | 4th-5th |
| RusherBot | Rust | Rush enemy cores aggressively | 4th-5th |
| GuardianBot | PHP | Defend cores, cautious expansion | 3rd-4th |
| SwarmBot | TypeScript | Formation cohesion, group advance | 1st-2nd |
| HunterBot | Java | Target isolated enemies | 1st-2nd |

## Running Tests

```bash
# Go engine tests
go test ./engine/... -v

# Web build verification
cd web && npm run build
```

## Building CLI Tools

```bash
go build ./cmd/acb-local
go build ./cmd/acb-mapgen
```

## Running a Match

```bash
./acb-local -seed 42 -max-turns 100 -output replay.json -verbose
```

## Viewing a Replay

```bash
cd web
npm run dev
# Standalone viewer: http://localhost:3000/index.html
# Full SPA: http://localhost:3000/app.html (then go to #/replay)
```
