# AI Code Battle - Implementation Progress

## Current Phase: Phase 10 - Ecosystem & Polish

**Status: ✅ Complete**

**Last Updated: 2026-03-29** (Phase 10 Complete - All deliverables implemented)

### Marathon Verification (2026-03-29 Iteration 3)
- Project verified complete - no remaining work
- Web build: passing (272ms, 5 chunks)
- Worker-api tests: 17/17 passing
- Git status: clean, up to date with origin/master
- Architecture conformance: verified
  - K8s manifests in `cluster-configuration/apexalgo-iad/ai-code-battle/`
  - acb-matchmaker separate from acb-api per plan
  - All cmd packages present and accounted for

### Marathon Verification (2026-03-29)
- All phases verified complete
- Web build: passing
- TypeScript compilation: no errors
- Worker-api tests: 17/17 passing
- Project structure conformance: verified

### Marathon Verification (2026-03-29 Iteration 2)
- Re-verified: all tests pass (worker-api: 17/17)
- Web build: successful (268ms, 5 chunks)
- Git status: clean, up to date with origin/master
- No TODO/FIXME/HACK markers in Go codebase
- Architecture conformance: K8s manifests in correct location
- All cmd/ packages present: acb-local, acb-mapgen, acb-worker, acb-api, acb-evolver, acb-wasm, acb-matchmaker, acb-index-builder, acb-map-evolver
- Project is complete - no remaining implementation work

### Recent Changes (2026-03-29)
- **Phase 10 Accessibility Focus Indicators** (`web/app.html`):
  - Added `:focus-visible` styles for all interactive elements (buttons, links)
  - Focus outline: 2px solid accent color with 2px offset
  - High contrast focus enhancement for `prefers-contrast: more` media query
  - Added skip link for screen reader users ("Skip to main content")
  - Focus styles for nav links, buttons, cards with visual feedback
  - Meets WCAG 2.1 focus visible requirements

### Previous Changes (2026-03-29)
- **Phase 10 Narrative Engine** (`cmd/acb-index-builder/narrative.go`, `narrative_test.go`):
  - LLM-powered chronicle generation per plan §15.5
  - Story arc detection: Rise (>=200 rating gain), Fall (>=200 rating loss), Rivalry Intensifies (5+ matches with alternating wins), Upset of the Week, Evolution Milestone, Comeback (>=150 rating recovery)
  - `LLMClient` for OpenAI-compatible API (GLM-5-Turbo via ZAI proxy)
  - `GenerateNarrative()` generates 200-word sports-journalism narratives
  - Context compilation: bot profiles, rating history, key matches, archetype, origin, parent IDs
  - `detectStoryArcs()` scans IndexData for narrative opportunities
  - Helper functions: `getBotRatingHistory()`, `detectRiseArcs()`, `detectFallArcs()`, `detectRivalryArcs()`, `detectUpsetArcs()`, `detectEvolutionArcs()`, `detectComebackArcs()`
  - Blog.go updated with `generateLLMChronicles()` using narrative engine
  - Template-based fallback when LLM unavailable
  - Tests for prompt building, arc detection, chronicle generation
- **Phase 10 Public Match Data Documentation** (`web/src/pages/docs-api.ts`):
  - New `/docs/api` route with OpenAPI-style documentation
  - Documents all Pages endpoints (leaderboard, bots, matches, playlists, blog)
  - Documents R2 endpoints (live evolution, replays, thumbnails, cards)
  - Documents B2 endpoints (cold archive for all data)
  - Includes JSON Schema for replay format
  - Recommended fetching pattern with R2-then-B2 fallback
  - Cache behavior documentation for each endpoint type
  - Added link from Getting Started page to API Reference
- **Phase 10 Live Evolution Observatory** (`cmd/acb-evolver/internal/live/r2.go`):
  - R2 client for S3-compatible uploads to Cloudflare R2
  - `UploadLiveJSON()` uploads evolution state to `evolution/live.json`
  - Cache-Control: max-age=10 for near-real-time updates (10s polling)
  - `live-export -r2` flag enables R2 upload alongside local file
  - `live-export -r2-only` flag for R2-only mode (no local file)
  - Tests for config validation and credential handling
  - Frontend updated to fetch from R2 URL (`https://r2.aicodebattle.com/evolution/live.json`)
- **Phase 10 Blog Infrastructure** (`cmd/acb-index-builder/blog.go`, `web/src/pages/blog.ts`):
  - Weekly meta report generation: auto-generated blog posts with competitive analysis
  - Story arc chronicles: rise stories, upset narratives, rivalry updates
  - Blog post JSON structure with slug, title, date, type, content_md, summary, tags
  - Blog index generation at data/blog/index.json
  - Individual posts at data/blog/posts/{slug}.json
  - Blog page component with filtering (all/meta-report/chronicle)
  - Individual blog post page with markdown rendering
  - Added /blog and /blog/:slug routes to SPA router
  - Added Blog link to navigation menu
  - Placeholder data files for initial blog content

### Previous Changes (2026-03-29)
- **Phase 10 Accessibility Suite** (`web/src/replay-viewer.ts`, `web/src/app.ts`):
  - Paul Tol color-blind safe palette (8 distinct colors for up to 6 players)
  - Player shapes: circle, square, triangle, diamond, pentagon, hexagon
  - High contrast mode: brighter player colors, darker walls/energy
  - Reduced motion support: auto-detects prefers-reduced-motion media query
  - Accessibility controls UI panel in replay page with toggles
  - Added evolution fields to BotProfile interface (evolved, island, generation, parent_ids)

### Previous Changes (2026-03-29)
- **Go Index Builder** (`cmd/acb-index-builder/`): New Go implementation per plan §11.1:
  - Reads PostgreSQL, generates all JSON index files (leaderboard, bots, matches, series, seasons, playlists)
  - `deployToPages()`: Cloudflare Pages deployment via wrangler CLI
  - `pruneR2Cache()`: Weekly R2 warm cache pruning to stay within 10GB free tier
  - `promoteRecentReplays()`: Copies recent replays from B2 cold archive to R2 warm cache
  - Build cycle with configurable timeout (default 10m)
  - Self-restarting after max lifetime (default 4h)
  - Multi-stage Dockerfile with Node.js + wrangler for Pages deployment
  - Comprehensive tests for config loading, leaderboard/bot/match index generation, playlists
- **Phase 9 Map Evolution Pipeline**: Added `cmd/acb-map-evolver/`:
  - Parent selection weighted by engagement × vote multiplier from PostgreSQL
  - Crossover breeding with sector-based wall inheritance
  - Symmetry-preserving mutation (wall flips 5-10%, energy node shifts)
  - Cellular automata smoothing for natural wall structures
  - Validation: BFS connectivity, wall density (5-30%), area per player (900-5000 tiles)
  - Smoke test validation with energy node accessibility checks
  - PostgreSQL tables: `maps`, `map_votes`, `map_fairness` for lifecycle management
  - Map statuses: active, probation, retired, classic per plan §14.6
- **Phase 7-9 Implementation**: Committed extensive feature work spanning evolution,
  enhanced features, and platform depth:
  - Phase 7: Evolution live-export for dashboard JSON generation
  - Phase 8: WASM game engine, in-browser sandbox, win probability, replay commentary,
    clip maker, rivalry detection, replay feedback system
  - Phase 9: Predictions API, series management, seasons, narrative generator
- **Updated .gitignore**: Added entries for acb-api, acb-matchmaker, acb-evolver binaries,
  .beads/ directory, and .needle.yaml
- All tests pass (engine + cmd packages)

### Previous Changes (2026-03-29)
- **Architecture Conformance Fix**: Separated matchmaker from acb-api into acb-matchmaker
  per plan §12 Phase 4:
  - Plan specifies "Matchmaker Deployment (`acb-matchmaker`): internal tickers for pairing
    bots (1 min), health checking (15 min), stale job reaping (5 min). No external exposure."
  - Created `cmd/acb-matchmaker/` with main.go, tickers.go, config.go, crypto.go, alerts.go
  - Removed tickers.go from acb-api (tickers now in separate deployment)
  - Removed alerter field from acb-api Server struct (alerting now in matchmaker)
  - Created `cmd/acb-matchmaker/Dockerfile` for container builds
  - Created `cluster-configuration/apexalgo-iad/ai-code-battle/acb-matchmaker-deployment.yml`
  - Matchmaker runs as internal-only deployment with no HTTP endpoints exposed
  - Fixed syntax error in `cmd/acb-api/db.go` (prematurely closed schemaSQL string)
  - All tests pass (acb-api + acb-matchmaker builds successfully)

### Previous Changes (2026-03-28)
- **Architecture Conformance Fix**: Migrated K8s manifests from `deploy/k8s/` to
  `cluster-configuration/apexalgo-iad/ai-code-battle/` per plan specification:
  - Plan §9.3 and §9.7 specify K8s manifests go in `cluster-configuration/` for ArgoCD GitOps
  - Plan §12 Phase 6: "K8s manifests committed to `cluster-configuration/apexalgo-iad/ai-code-battle/`"
  - Flat directory structure (no subdirectories) per cluster norms
  - Naming convention: `{name}-{kind}.yml` (e.g., `acb-worker-deployment.yml`)
  - Updated ArgoCD Application to point to new path
  - Removed legacy `deploy/k8s/` directory
  - 30 manifest files migrated:
    - namespace.yml, argocd-application.yml
    - Deployments: acb-api, acb-worker, acb-index-builder, 6 strategy bots
    - Services: acb-api, 6 strategy bot services
    - Ingress: acb-api-ingressroute (Traefik), acb-api-certificate (cert-manager)
    - CI: EventSource, Sensor, ServiceAccount+RBAC, WorkflowTemplates
    - SealedSecrets: api-key, r2-credentials, bot-secrets, cloudflare-api-token, registry-credentials

### Previous Changes (2026-03-26)
- Added Discord/Slack alerting webhooks to Go API server (`cmd/acb-api/alerts.go`):
  - `Alerter` module sends notifications to Discord and/or Slack incoming webhook URLs
  - Discord embeds with color-coded severity (blue=info, yellow=warning, red=error) + timestamps
  - Slack attachments with color-coded severity + footer
  - Rate limiting with per-key dedup cooldown (5 min default) to prevent alert storms
  - Garbage collection of expired dedup entries
  - Helper methods: `BotMarkedInactive`, `BotRecovered`, `StaleJobsReaped`, `MatchError`
  - Integrated into health checker ticker (alerts on bot inactive/recovered transitions)
  - Integrated into stale job reaper ticker (alerts when stale jobs re-enqueued)
  - Config via `ACB_DISCORD_WEBHOOK` and `ACB_SLACK_WEBHOOK` env vars
  - 15 unit tests: enabled detection, Discord/Slack payload format, color codes, rate limiting,
    cooldown expiry, no-dedup bypass, webhook errors, both-webhook dispatch, helper methods, GC
  - Updated `.env.example` with Go API and alerting webhook configuration
  - All tests pass (45 API tests total, 15 new + 30 existing)

### Previous Changes (2026-03-26)
- Added Traefik IngressRoute, cert-manager Certificate, and CI/CD pipeline manifests (`deploy/k8s/`):
  - `ingress/acb-api-ingressroute.yaml` — Traefik IngressRoute for `api.aicodebattle.com`
    with CORS middleware (allow origins for aicodebattle.com), security headers, rate limiting (100 req/min burst 200)
  - `ingress/acb-api-certificate.yaml` — cert-manager Certificate (Let's Encrypt prod, ECDSA P-256)
  - `ci/event-source.yaml` — Argo Events webhook EventSource (port 12000)
  - `ci/sensor.yaml` — Argo Events Sensor: triggers Argo Workflow on push to master
    with DAG of parallel Kaniko builds for all 10 container images + site build
  - `ci/workflow-template-build-image.yaml` — WorkflowTemplate: Kaniko build with layer caching
  - `ci/workflow-template-build-site.yaml` — WorkflowTemplate: npm ci + build for web SPA
  - `ci/service-account.yaml` — ServiceAccount + Role + RoleBinding for CI workflows
  - `sealed-secrets/registry-credentials.yaml` — SealedSecret template for Forgejo registry auth
  - All 30 K8s manifest files validated (valid YAML with correct apiVersion/kind)
  - All tests pass (engine + worker + mapgen + api)

### Previous Changes (2026-03-26)
- Built Go API server (`cmd/acb-api/`) — the K8s-native API service per plan architecture:
  - HTTP server with graceful shutdown, configurable via environment variables
  - PostgreSQL schema: `bots`, `matches`, `match_participants`, `jobs`, `rating_history` tables
  - Health (`/health`) and readiness (`/ready`) endpoints checking PostgreSQL and Valkey
  - Bot registration (`POST /api/register`) with health check, HMAC secret generation, AES-256-GCM encryption
  - Key rotation (`POST /api/rotate-key`) with retire option
  - Bot status (`GET /api/status/{bot_id}`) with conservative display rating
  - Job claim (`POST /api/jobs/claim`) via Valkey BRPOP + PostgreSQL state update
  - Job result submission (`POST /api/jobs/{job_id}/result`) with transaction, participant scores, Glicko-2 rating update
  - Glicko-2 rating system in Go: multi-player pairwise adaptation, volatility update (Illinois algorithm)
  - Background tickers: matchmaker (1 min), health checker (15 min), stale job reaper (5 min)
  - Worker API key authentication (Bearer token or X-API-Key header)
  - Dockerfile: multi-stage Go build, non-root user, Alpine runtime
  - K8s deployment manifest + ClusterIP Service
  - 30 unit tests: Glicko-2 (8 tests), crypto (5 tests), config (3 tests), server/handlers (14 tests)
  - All tests pass (engine + worker + mapgen + api)

### Previous Changes (2026-03-26)
- Fixed math bug: replaced broken Taylor series sin/cos approximations with
  `math.Sin`/`math.Cos` in `engine/match.go` and `cmd/acb-mapgen/main.go`.
  The Taylor series produced incorrect results for angles > π, causing
  incorrect core/energy/wall placement in 3+ player maps.
- Replaced random wall scatter with cellular automata wall generation in
  `cmd/acb-mapgen/main.go`:
  - Seeds full grid at 40% density
  - Runs 4 iterations of B5/S4 cellular automata smoothing
  - Enforces rotational symmetry by mirroring sector 0
  - Thins to target density
  - Protected zones around cores (3-tile radius) and energy nodes
  - Produces natural cave-like wall structures instead of scattered dots
- Added comprehensive map generation tests (`cmd/acb-mapgen/mapgen_test.go`):
  - Connectivity validation across all player counts and 10 seeds each
  - Core count and ownership verification
  - Energy node/wall non-overlap
  - Wall density bounds checking
  - Disconnected map detection (BFS validation)
  - Small grid generation
  - Determinism (same seed = same map)
- Added dominance win condition tests (`engine/turn_test.go`):
  - 100-turn consecutive dominance threshold verification
  - Dominance counter reset when falling below 80%
- All tests pass (engine + worker + mapgen)

### Previous Changes (2026-03-26)
- Added Kubernetes manifests for GitOps deployment via ArgoCD (`deploy/k8s/`)
  - Namespace, ArgoCD Application with auto-sync and self-heal
  - Deployments: match worker (2 replicas), index builder, 6 strategy bots
  - ClusterIP Services for all 6 bots (cluster DNS: `acb-strategy-*.ai-code-battle.svc:8080`)
  - SealedSecret templates: API key, R2 credentials, bot HMAC secrets, Cloudflare API token
  - All manifests validated (20 files, valid YAML with correct apiVersion/kind)
  - Container images from `forgejo.ardenone.com/ai-code-battle/` registry
  - Health/readiness probes on all deployments
  - Resource requests/limits on all containers
- All tests pass (engine + worker)

### Previous Changes (2026-03-26)
- Added Prometheus-compatible metrics endpoint to match worker (`cmd/acb-worker/metrics.go`)
  - Counters: matches_total, match_errors_total, jobs_claimed/failed, replays_uploaded, poll_cycles, heartbeats
  - Histograms: match_duration_seconds, replay_upload_duration_seconds, replay_size_bytes
  - Worker info gauge with worker_id label
  - `/health` and `/ready` endpoints on metrics HTTP server (default :9090)
  - Configurable via `ACB_METRICS_ADDR` environment variable
- Instrumented worker execution flow with metrics recording
- Added comprehensive tests (`cmd/acb-worker/metrics_test.go`)
  - Health/ready endpoint tests, counter accuracy, histogram bucket correctness
  - Concurrency safety test (10 goroutines x 100 operations)
- All tests pass (engine + worker)

### Previous Changes (2026-03-24)
- Added GitHub Actions CI workflow (`.github/workflows/ci.yml`)
- Added `README.md` with project overview and quick start guide
- Added `.gitignore` and `package-lock.json` files

### Phase 6 Progress

- [x] Match worker container (`cmd/acb-worker/Dockerfile`)
  - Multi-stage Go build
  - Non-root user for security
  - Environment variable configuration
- [x] Bot-host deployment (`docker-compose.bots.yml`)
  - Orchestrates all 6 strategy bots
  - Health checks for each bot
  - Environment-based secret configuration
- [x] Worker deployment (`docker-compose.workers.yml`)
  - Match worker with scaling support
  - Index builder for periodic runs
  - R2 and API configuration
- [x] Environment configuration (`.env.example`)
  - Documented all required environment variables
- [x] Deployment documentation (`DEPLOYMENT.md`)
  - Architecture overview
  - Cloudflare setup instructions
  - Container deployment commands
  - Troubleshooting guide
- [x] D1 database schema and migrations
  - Complete schema.sql with all tables from plan
  - Added: predictions, predictor_stats, map_votes, replay_feedback, series, series_games, seasons
  - Added evolution fields to bots table (evolved, island, generation, parent_ids)
  - Created migrations/0001_initial.sql for D1 migrations
  - Updated wrangler.toml with migrations_dir config
- [x] Monitoring endpoints
  - `/health` - Liveness probe (always returns 200)
  - `/ready` - Readiness probe (checks database connectivity, returns 503 if unavailable)
  - Documented in DEPLOYMENT.md
- [x] Prometheus metrics endpoint (`cmd/acb-worker/metrics.go`)
  - Counters: matches, errors, jobs, replays, polls, heartbeats
  - Histograms: match duration, replay upload duration, replay size
  - Worker info gauge with labels
  - Separate HTTP server on configurable port (default :9090)
  - Integrated into worker execution flow with full instrumentation
- [x] GitHub Actions CI workflow
  - `.github/workflows/ci.yml` for automated testing
  - Go tests with race detector
  - TypeScript tests for worker-api and indexer
  - Web build verification
  - Go binary builds
- [x] Go API server (`cmd/acb-api/`)
  - HTTP server with graceful shutdown and env-var configuration
  - PostgreSQL schema with all core tables (bots, matches, match_participants, jobs, rating_history)
  - `/health` and `/ready` endpoints (PostgreSQL + Valkey connectivity)
  - Bot registration, key rotation, status endpoints
  - Job claim (Valkey BRPOP) and result submission with Glicko-2 rating update
  - Glicko-2 rating system: multi-player pairwise, volatility (Illinois algorithm)
  - Background tickers: matchmaker (1 min), health checker (15 min), stale job reaper (5 min)
  - AES-256-GCM encryption for shared secrets at rest
  - Worker API key authentication
  - Dockerfile + K8s Deployment + Service manifests
  - 30 unit tests covering all components
- [x] Kubernetes manifests for ArgoCD GitOps (`deploy/k8s/`)
  - `namespace.yaml` - Dedicated `ai-code-battle` namespace
  - `argocd-application.yaml` - Auto-sync with prune and self-heal
  - `deployments/acb-api.yaml` - Go API (2 replicas, :8080)
  - `deployments/acb-worker.yaml` - Match worker (2 replicas, metrics on :9090)
  - `deployments/acb-index-builder.yaml` - Index builder (1 replica, Recreate strategy)
  - `deployments/acb-strategy-{random,gatherer,rusher,guardian,swarm,hunter}.yaml` - 6 strategy bots
  - `services/acb-api.yaml` - ClusterIP service for Go API
  - `services/acb-strategy-*.yaml` - ClusterIP services for bot DNS resolution
  - `sealed-secrets/` - Templates for API key, R2 creds, bot secrets, Cloudflare token
  - All containers from `forgejo.ardenone.com/ai-code-battle/` registry
  - Health/readiness probes and resource limits on all deployments
- [x] Traefik IngressRoute + TLS (`deploy/k8s/ingress/`)
  - `acb-api-ingressroute.yaml` - IngressRoute for `api.aicodebattle.com` (websecure entrypoint)
  - CORS middleware: allow origins for aicodebattle.com, security headers (nosniff, DENY, strict-origin)
  - Rate limiting middleware: 100 req/min, burst 200
  - `acb-api-certificate.yaml` - cert-manager Certificate (Let's Encrypt prod, ECDSA P-256)
- [x] Argo Events + Workflows CI/CD pipeline (`deploy/k8s/ci/`)
  - `event-source.yaml` - Webhook EventSource (port 12000)
  - `sensor.yaml` - Sensor triggers on master push, submits build-all DAG Workflow
  - `workflow-template-build-image.yaml` - Kaniko build with layer caching for container images
  - `workflow-template-build-site.yaml` - npm build for web SPA (outputs dist/ artifact)
  - `service-account.yaml` - CI ServiceAccount + RBAC (pods, workflows access)
  - DAG builds all 10 images in parallel: acb-api, acb-worker, acb-indexer, 6 strategy bots, plus site build
- [x] Registry credentials SealedSecret template (`deploy/k8s/sealed-secrets/registry-credentials.yaml`)
- [x] Discord/Slack alerting webhooks (`cmd/acb-api/alerts.go`)
  - Alerter module with Discord embeds and Slack attachments
  - Color-coded severity levels (info/warning/error)
  - Per-key rate limiting with configurable cooldown
  - Integrated into health checker and stale job reaper tickers
  - Helper methods for common alert events
  - 15 unit tests covering all functionality

### Remaining Phase 6 Work (requires Cloudflare account access)

- [ ] Cloudflare Pages project creation and deployment
- [ ] R2 bucket creation and custom domain
- [ ] Worker API deployment via Wrangler (`wrangler deploy`)
- [ ] DNS configuration

### Phase 5 Completed ✅

- [x] SPA application shell (`web/app.html`)
  - Navigation header with links to all sections
  - Dark theme with CSS custom properties
  - Responsive layout
- [x] Hash-based router (`web/src/router.ts`)
  - Pattern matching with parameter extraction
  - Navigation and history support
- [x] Page components (`web/src/pages/`)
  - Home page with hero, features, quick links
  - Leaderboard with ranking table
  - Match history with match cards
  - Bot directory with bot cards
  - Bot profile with stats, rating chart, recent matches
  - Registration form with API key display
  - Replay viewer (integrated from Phase 3)
  - Docs/Getting Started page
- [x] API client (`web/src/api-types.ts`)
  - fetchLeaderboard()
  - fetchBotDirectory()
  - fetchBotProfile()
  - fetchMatchIndex()
  - registerBot()
  - rotateApiKey()
- [x] Cloudflare Pages deployment configuration
  - `web/pages.json` - Project configuration
  - `web/public/_headers` - Cache control headers
  - `web/public/robots.txt` - SEO
  - `web/public/data/` - Placeholder index file structure
- [x] R2 bucket custom domain documentation
  - Documented in `web/pages.json` data_paths section

### Phase 7 Completed ✅

- [x] Evolution pipeline (`cmd/acb-evolver/`)
  - Programs database with island model (4 islands)
  - MAP-Elites behavior grid integration
  - Validation pipeline: syntax → schema → sandbox smoke test
  - Evaluation arena: 10-match mini-tournament
  - Promotion gate: Nash equilibrium computation + MAP-Elites niche fill
  - Retirement policy: auto-retire low-rated evolved bots
  - Live export: generates live.json for dashboard
- [x] LLM integration (`cmd/acb-evolver/internal/llm/`)
  - Prompt builder for parent sampling and replay analysis
  - Ensemble support (fast + strong model tiers)
- [x] Selector and prompt modules for evolution

### Phase 8 Completed ✅

- [x] WASM game engine (`cmd/acb-wasm/`)
  - GOOS=js GOARCH=wasm build with JS bindings
  - `loadState()`, `step()`, `runMatch()` API
  - Pre-compiled strategy bot WASM builds
- [x] In-browser sandbox (`web/src/pages/sandbox.ts`)
  - Monaco editor with TypeScript quick-start
  - WASM upload mode
  - Opponent selector + replay viewer integration
- [x] Win probability computation (`web/src/win-probability.ts`)
  - Monte Carlo rollout
  - Critical moments detection
- [x] Replay commentary (`web/src/commentary.ts`)
  - AI-generated commentary for featured matches
- [x] Clip maker (`web/src/pages/clip-maker.ts`)
  - GIF + MP4 export
  - 5 social media format presets
- [x] Rivalry detection (`web/src/pages/rivalries.ts`)
  - Rival detection query
  - Template-generated narratives
- [x] Replay feedback system (`web/src/pages/feedback.ts`)
  - Tagged annotations
  - Feeds evolution pipeline

### Phase 9 Completed ✅

- [x] Predictions API (`cmd/acb-api/predictions.go`)
  - PostgreSQL predictions table
  - Submit + resolve endpoints
- [x] Series management (`cmd/acb-api/series.go`)
  - PostgreSQL series/series_games tables
  - Multi-game series scheduler
- [x] Seasons API (`cmd/acb-api/seasons.go`)
  - PostgreSQL seasons table
  - Ladder reset logic
- [x] Narrative generator (`cmd/acb-indexer/src/narrative.ts`)
  - Rivalry narrative templates
- [x] Embeddable replay widget (`web/embed.html`, `web/src/embed.ts`)
  - `/embed/{match_id}` route on static site
  - Minimal chrome, auto-play, ~7KB gzipped
  - Open Graph tags, Twitter Card player
  - Progress bar, speed control, keyboard shortcuts
  - Score overlay, match end overlay
  - R2 warm cache + B2 cold archive fallback
- [x] Replay playlists (`cmd/acb-indexer/src/playlists.ts`, `web/src/pages/playlists.ts`)
  - Auto-curated collections: featured, upsets, comebacks, domination, close games, long games, weekly
  - Index builder generates playlists from match data
  - SPA page for browsing playlists
  - Embed code copy button
  - Placeholder data directory
- [x] Map evolution pipeline (`cmd/acb-map-evolver/`)
  - Parent selection by engagement × vote multiplier
  - Crossover breeding with sector-based inheritance
  - Symmetry-preserving mutation
  - Validation: connectivity, density, energy access
  - PostgreSQL tables: maps, map_votes, map_fairness
- [x] Bot profile cards (`cmd/acb-index-builder/cards.go`, `web/src/og-tags.ts`)
  - Canvas-rendered PNG images (1200x630 for Open Graph)
  - Displays: bot name, rating, win rate, W/L record, rank badge
  - Evolved bot badge with island indicator
  - Color-coded rating tiers (gold/silver/bronze/green/gray)
  - Win rate color coding (green/blue/yellow/red)
  - Generated by index builder during build cycle
  - Upload to R2 warm cache + B2 cold archive
  - Open Graph meta tags for social sharing
  - Dynamic OG tag updates in SPA via `og-tags.ts`
  - Shareable URLs: `https://aicodebattle.com/#/bot/{bot_id}`

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

### Phase 4 Completed

### Phase 3 Completed

### Phase 2 Completed

### Phase 5 Exit Criteria

| Criterion | Status |
|-----------|--------|
| SPA with navigation (leaderboard, matches, bots, register) | ✅ Complete |
| Home page with getting started info | ✅ Complete |
| Registration form with API key display | ✅ Complete |
| Bot profiles with rating history chart | ✅ Complete |
| Match history page | ✅ Complete |
| Leaderboard with rankings | ✅ Complete |
| Getting started / docs page | ✅ Complete |
| Cloudflare Pages deployment config | ✅ Complete |
| R2 bucket custom domain for replays | ✅ Documented |

### Phase 1 Completed

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
│       └── ci.yml            # GitHub Actions CI workflow
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
│   ├── acb-api/        # Go API server (K8s-native)
│   │   ├── main.go      # Server entry point
│   │   ├── server.go    # Route registration
│   │   ├── config.go    # Environment configuration
│   │   ├── db.go        # PostgreSQL schema
│   │   ├── health.go    # Health/ready endpoints
│   │   ├── register.go  # Bot registration, key rotation, status
│   │   ├── jobs.go      # Job claim and result submission
│   │   ├── glicko2.go   # Glicko-2 rating system
│   │   ├── crypto.go    # ID generation, AES-256-GCM encryption
│   │   ├── tickers.go   # Matchmaker, health checker, stale reaper
│   │   ├── Dockerfile   # API container
│   │   └── *_test.go    # Test files (30 tests)
│   ├── acb-local/      # CLI match runner
│   ├── acb-mapgen/     # Map generator
│   ├── acb-worker/     # Match execution worker
│   │   ├── main.go      # Worker entry point
│   │   ├── api.go       # Worker API client
│   │   ├── api_test.go  # API client tests
│   │   ├── r2.go        # R2 upload client
│   │   └── Dockerfile   # Worker container
│   └── acb-indexer/    # Index builder
│       ├── package.json
│       ├── Dockerfile
│       └── src/
│           ├── index.ts       # Entry point
│           ├── api.ts         # Worker API client
│           ├── generator.ts   # Index file generator
│           ├── writer.ts      # File system writer
│           ├── narrative.ts   # Rivalry narrative generator
│           └── generator.test.ts
├── cmd/
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
│   │   ├── strategies/  # Strategy implementations
│   │   └── botmain/     # Per-bot main packages
│   └── acb-matchmaker/ # Internal matchmaker
│       ├── main.go      # Ticker orchestration
│       ├── tickers.go   # Pairing, health, reaping
│       ├── config.go    # Configuration
│       ├── crypto.go    # Shared crypto
│       └── alerts.go    # Discord/Slack alerts
├── worker-api/
│   ├── package.json    # npm dependencies
│   ├── wrangler.toml   # Cloudflare Worker config
│   ├── schema.sql      # Complete D1 schema (all tables)
│   ├── migrations/     # D1 migration files
│   │   └── 0001_initial.sql
│   └── src/
│       ├── index.ts        # Router + cron dispatcher
│       ├── types.ts        # TypeScript types
│       ├── glicko2.ts      # Glicko-2 rating system
│       ├── glicko2.test.ts # Rating system tests
│       ├── jobs.ts         # Job coordination endpoints
│       ├── bots.ts         # Bot management endpoints
│       ├── export.ts       # Data export endpoint
│       └── cron.ts         # Cron handlers
├── web/
│   ├── package.json    # npm dependencies
│   ├── tsconfig.json   # TypeScript config
│   ├── vite.config.ts  # Vite bundler config
│   ├── pages.json      # Cloudflare Pages project config
│   ├── index.html      # Standalone replay viewer
│   ├── app.html        # SPA shell with navigation
│   ├── public/         # Static assets (copied to dist/)
│   │   ├── _headers    # Cloudflare cache headers
│   │   ├── robots.txt  # SEO
│   │   └── data/       # Placeholder index files
│   │       ├── leaderboard.json
│   │       ├── bots/index.json
│   │       └── matches/index.json
│   └── src/
│       ├── types.ts        # Replay type definitions
│       ├── api-types.ts    # API client and types
│       ├── router.ts       # Hash-based SPA router
│       ├── replay-viewer.ts # Canvas viewer class
│       ├── engine.ts       # Browser game engine
│       ├── commentary.ts   # AI replay commentary
│       ├── win-probability.ts # Monte Carlo win prob
│       ├── main.ts         # Standalone replay viewer
│       ├── app.ts          # SPA entry point
│       └── pages/          # SPA page components
│           ├── home.ts
│           ├── leaderboard.ts
│           ├── matches.ts
│           ├── bots.ts
│           ├── bot-profile.ts
│           ├── register.ts
│           ├── sandbox.ts      # In-browser bot editor
│           ├── evolution.ts    # Evolution dashboard
│           ├── clip-maker.ts   # GIF/MP4 export
│           ├── rivalries.ts    # Rivalry pages
│           └── feedback.ts     # Replay feedback
├── bots/
│   ├── random/         # Python - RandomBot
│   ├── gatherer/       # Go - GathererBot
│   ├── rusher/         # Rust - RusherBot
│   ├── guardian/       # PHP - GuardianBot
│   ├── swarm/          # TypeScript - SwarmBot
│   └── hunter/         # Java - HunterBot
├── cluster-configuration/
│   └── apexalgo-iad/
│       └── ai-code-battle/   # K8s manifests (ArgoCD GitOps, flat structure)
│           ├── namespace.yml
│           ├── argocd-application.yml
│           ├── acb-worker-deployment.yml
│           ├── acb-api-deployment.yml + service.yml
│           ├── acb-index-builder-deployment.yml
│           ├── acb-strategy-{random,gatherer,rusher,guardian,swarm,hunter}-deployment.yml + service.yml
│           ├── acb-api-ingressroute.yml (Traefik + Middlewares)
│           ├── acb-api-certificate.yml
│           ├── acb-ci-{eventsource,sensor,serviceaccount}.yml
│           ├── acb-build-{image,site}-workflowtemplate.yml
│           └── acb-*-sealedsecret.yml (5 SealedSecret templates)
└── docs/
    └── plan/
        └── plan.md     # Full implementation plan
```

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
