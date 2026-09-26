// API Documentation - Static JSON Endpoints
// OpenAPI-style documentation for all public data endpoints

interface EndpointDoc {
  method: string;
  path: string;
  description: string;
  cache: string;
  responseExample?: string;
  schemaLink?: string;
}

interface Section {
  title: string;
  description: string;
  endpoints: EndpointDoc[];
}

const PAGES_BASE = 'https://ai-code-battle.pages.dev';

const sections: Section[] = [
  {
    title: 'Pages Endpoints (Pre-computed JSON)',
    description: 'All data on Pages is pre-computed by the index builder and deployed every ~90 minutes. These files are served from the Cloudflare CDN with automatic cache invalidation on deploy.',
    endpoints: [
      {
        method: 'GET',
        path: '/data/leaderboard.json',
        description: 'Current leaderboard with ratings, win rates, and health status for all registered bots.',
        cache: '~90 min (deploy cycle)',
        responseExample: `{
  "updated_at": "2026-03-29T12:00:00Z",
  "season": "season-1",
  "entries": [
    {
      "rank": 1,
      "bot_id": "bot_abc123",
      "name": "SwarmBot",
      "owner_id": "user_xyz",
      "rating": 1847.3,
      "rating_deviation": 42.1,
      "matches_played": 152,
      "matches_won": 98,
      "win_rate": 0.645,
      "health_status": "active"
    }
  ]
}`,
      },
      {
        method: 'GET',
        path: '/data/bots/index.json',
        description: 'Bot directory listing all registered bots with summary stats.',
        cache: '~90 min (deploy cycle)',
        responseExample: `{
  "updated_at": "2026-03-29T12:00:00Z",
  "bots": [
    {
      "id": "bot_abc123",
      "name": "SwarmBot",
      "rating": 1847.3,
      "matches_played": 152,
      "win_rate": 0.645
    }
  ]
}`,
      },
      {
        method: 'GET',
        path: '/data/bots/{bot_id}.json',
        description: 'Full bot profile including rating history and recent matches. For evolved bots, includes lineage information.',
        cache: '~90 min (deploy cycle)',
        responseExample: `{
  "id": "bot_abc123",
  "name": "SwarmBot",
  "owner_id": "user_xyz",
  "rating": 1847.3,
  "rating_deviation": 42.1,
  "rating_volatility": 0.06,
  "matches_played": 152,
  "matches_won": 98,
  "win_rate": 0.645,
  "health_status": "active",
  "created_at": "2026-01-15T08:00:00Z",
  "updated_at": "2026-03-29T12:00:00Z",
  "rating_history": [...],
  "recent_matches": [...],
  "evolved": false
}`,
      },
      {
        method: 'GET',
        path: '/data/matches/index.json',
        description: 'Paginated index of recent matches with participants and results.',
        cache: '~90 min (deploy cycle)',
        responseExample: `{
  "updated_at": "2026-03-29T12:00:00Z",
  "matches": [
    {
      "id": "match_xyz789",
      "completed_at": "2026-03-29T11:45:00Z",
      "participants": [
        {"bot_id": "bot_abc123", "name": "SwarmBot", "score": 8, "won": true},
        {"bot_id": "bot_def456", "name": "HunterBot", "score": 5, "won": false}
      ],
      "winner_id": "bot_abc123",
      "turns": 247,
      "end_reason": "dominance"
    }
  ],
  "pagination": {"page": 1, "per_page": 50, "total": 1250}
}`,
      },
      {
        method: 'GET',
        path: '/data/playlists/index.json',
        description: 'Index of all curated replay playlists (featured, upsets, rivalries, etc.).',
        cache: '~90 min (deploy cycle)',
        responseExample: `{
  "updated_at": "2026-03-29T12:00:00Z",
  "playlists": [
    {
      "slug": "featured",
      "title": "Featured Matches",
      "description": "Editor's picks for the most exciting matches",
      "category": "featured",
      "match_count": 25,
      "updated_at": "2026-03-29T10:00:00Z"
    }
  ]
}`,
      },
      {
        method: 'GET',
        path: '/data/playlists/{slug}.json',
        description: 'Full playlist with match IDs in viewing order.',
        cache: '~90 min (deploy cycle)',
        responseExample: `{
  "slug": "featured",
  "title": "Featured Matches",
  "description": "Editor's picks",
  "category": "featured",
  "match_count": 25,
  "created_at": "2026-01-15T00:00:00Z",
  "updated_at": "2026-03-29T10:00:00Z",
  "matches": [
    {"match_id": "match_xyz789", "order": 1, "title": "Epic Comeback"}
  ]
}`,
      },
      {
        method: 'GET',
        path: '/data/blog/index.json',
        description: 'Blog post index with summaries. Includes weekly meta reports and narrative chronicles.',
        cache: '~90 min (deploy cycle)',
        responseExample: `{
  "updated_at": "2026-03-29T12:00:00Z",
  "posts": [
    {
      "slug": "meta-week-13-season-1",
      "title": "Week 13 Meta Report: Rise of the Swarm",
      "published_at": "2026-03-29T09:00:00Z",
      "summary": "SwarmBot dominance continues..."
    }
  ]
}`,
      },
      {
        method: 'GET',
        path: '/data/blog/posts/{slug}.json',
        description: 'Full blog post with HTML content and weekly stats. Posts live one level below the index that lists their slugs.',
        cache: '~90 min (deploy cycle)',
        responseExample: `{
  "slug": "meta-week-13-season-1",
  "title": "Week 13 Meta Report",
  "published_at": "2026-03-29T09:00:00Z",
  "week_start": "2026-03-23",
  "summary": "SwarmBot dominance...",
  "body_html": "<p>This week...</p>",
  "stats": {
    "matches_played": 1520,
    "top_bot": "SwarmBot",
    "top_bot_rating": 1847
  }
}`,
      },
    ],
  },
  {
    title: 'Replay & Media Assets (Pipeline Offline)',
    description: 'Replays, bot cards, thumbnails, evolution live data, and the map library are produced by the index builder, which bundles the warm set into the Pages deploy and keeps the cold archive in private B2 (B2 has no public hostname). The compute tier that feeds that pipeline is decommissioned, so none of these URLs is served today — each currently answers the SPA HTML fallback, not data. The paths below are the designed contract: the SPA loader (web/src/lib/replay-data.ts) and the builder (cmd/acb-index-builder bundleWarm*) already implement both halves, and they come online unchanged when the pipeline runs. See docs/notes/public-api-descope.md for the deferral decision and revival triggers.',
    endpoints: [
      {
        method: 'GET',
        path: '/data/replays/{match_id}.json.gz',
        description: 'Gzipped replay (compact delta-encoded v2.1). Pages serves the .json.gz bytes verbatim, so the client gunzips with DecompressionStream — see the Fetching Pattern below.',
        cache: 'OFFLINE — not served until the index-builder pipeline runs (designed: immutable, content-addressed)',
        schemaLink: '#replay-schema',
      },
      {
        method: 'GET',
        path: '/data/evolution/live.json',
        description: 'Real-time evolution observatory data, refreshed every evolution cycle (~5 min) once live.',
        cache: 'OFFLINE — not served until the index-builder pipeline runs (designed: 10 seconds)',
      },
      {
        method: 'GET',
        path: '/data/cards/{bot_id}.png',
        description: 'Canvas-rendered bot profile card image (1200x630) for Open Graph social sharing.',
        cache: 'OFFLINE — not served until the index-builder pipeline runs (designed: max-age=86400)',
      },
      {
        method: 'GET',
        path: '/data/thumbnails/{match_id}.png',
        description: 'Auto-generated match thumbnail for embed previews.',
        cache: 'OFFLINE — not served until the index-builder pipeline runs (designed: max-age=86400)',
      },
      {
        method: 'GET',
        path: '/maps/index.json',
        description: 'Map library index with all available maps grouped by player count.',
        cache: 'OFFLINE — not served until the index-builder pipeline runs (designed: ~90 min, deploy cycle)',
      },
      {
        method: 'GET',
        path: '/maps/{map_id}.json',
        description: 'Individual map details including full geometry (walls, cores, energy nodes).',
        cache: 'OFFLINE — not served until the index-builder pipeline runs (designed: ~90 min, deploy cycle)',
      },
    ],
  },
  {
    title: 'Interactive API (Same-Origin)',
    description: 'State-changing endpoints served by this site itself as JSON under /api/* (a Pages Function backed by R2 — see docs/notes/api-transport.md for the transport decision). Replay feedback and map voting are live; match-tier endpoints (bot registration, API key rotation, predictions) are backed by the acb-api service, which is not deployed yet, so they answer 503 with JSON code "match_tier_offline". Everything is rate-limited per IP and caps request bodies at 32 KiB.',
    endpoints: [
      {
        method: 'GET',
        path: '/api/health',
        description: 'Liveness and capability report. A false capability flag means that flow currently renders its offline state in the UI.',
        cache: 'no cache (dynamic)',
        responseExample: `{
  "status": "ok",
  "capabilities": {
    "register": false,
    "rotate_key": false,
    "predictions": false,
    "feedback": true,
    "map_votes": true
  }
}`,
      },
      {
        method: 'POST',
        path: '/api/feedback',
        description: 'Submit replay feedback ({match_id, turn, type: "insight"|"mistake"|"idea"|"highlight", body, author?}) or site feedback from the Agentation overlay ({markdown, ...}).',
        cache: 'no cache (rate-limited)',
        responseExample: `{"status": "recorded", "feedback_id": "fb_3f2a91c04b7e"}`,
      },
      {
        method: 'GET',
        path: '/api/feedback/{match_id}',
        description: 'Feedback entries for one replay. Voter dedupe sets are internal and never served.',
        cache: 'no cache (dynamic)',
        responseExample: `{
  "match_id": "match_xyz789",
  "feedback": [
    {
      "feedback_id": "fb_3f2a91c04b7e",
      "match_id": "match_xyz789",
      "turn": 42,
      "type": "insight",
      "body": "White sacrifices the east core to win the energy race.",
      "author": "Anonymous",
      "upvotes": 3,
      "created_at": "2026-09-25T12:00:00.000Z"
    }
  ]
}`,
      },
      {
        method: 'POST',
        path: '/api/feedback/{id}/upvote',
        description: 'Upvote a feedback entry. Body: {"voter_id": "..."}. One upvote per voter; repeats answer {"status": "already_upvoted"}.',
        cache: 'no cache (rate-limited)',
        responseExample: `{"status": "recorded"}`,
      },
      {
        method: 'POST',
        path: '/api/vote/map',
        description: 'Vote a map up or down. Body: {"map_id", "voter_id", "vote": 1|-1}. One vote per voter per map; voting again replaces it.',
        cache: 'no cache (rate-limited)',
        responseExample: `{"map_id": "map_tq8tx8vk", "vote": 1, "net_votes": 12}`,
      },
      {
        method: 'GET',
        path: '/api/vote/map/{map_id}',
        description: 'Net votes for a map. Pass ?voter_id= to include your own current vote as my_vote.',
        cache: 'no cache (dynamic)',
        responseExample: `{"map_id": "map_tq8tx8vk", "net_votes": 12, "my_vote": 1}`,
      },
      {
        method: 'POST',
        path: '/api/register',
        description: 'Match tier — offline. Answers 503 {"error": "...", "code": "match_tier_offline"} until acb-api is deployed. The request shape is frozen to the service contract (Getting Started → Register Your Bot).',
        cache: 'no cache (offline)',
      },
      {
        method: 'POST',
        path: '/api/predict',
        description: 'Match tier — offline, same 503 "match_tier_offline" answer as /api/register. /api/rotate-key and the /api/predictions/* reads behave identically.',
        cache: 'no cache (offline)',
      },
    ],
  },
  {
    title: 'Archive',
    description: 'The permanent cold archive for ALL replays and match media is private Backblaze B2, reached only by the index builder — B2 has no public hostname (the old b2.aicodebattle.com host is retired) and no archive path is publicly fetchable. Public replay access is the warm set the builder bundles into the Pages deploy above.',
    endpoints: [],
  },
];

// Exported for the honesty-contract test (docs-api.test.ts): the 2026-09-26
// static-endpoint decision (bead aicodeba-4be62ac3) retired the B2-origin
// listings — b2.aicodebattle.com is dead, and pointing its paths at the Pages
// origin advertised URLs that answer the SPA HTML fallback. Every path a
// section advertises must be either verified-live or explicitly marked
// OFFLINE. See docs/notes/public-api-descope.md.
export const docsApiSections = sections;

// Replay JSON Schema section
const replaySchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://ai-code-battle.pages.dev/schemas/replay.json",
  "title": "Match Replay",
  "description": "Complete replay of an AI Code Battle match",
  "type": "object",
  "required": ["match_id", "config", "map", "players", "turns", "result"],
  "properties": {
    "match_id": {
      "type": "string",
      "description": "Unique match identifier"
    },
    "config": {
      "$ref": "#/definitions/GameConfig"
    },
    "map": {
      "$ref": "#/definitions/Map"
    },
    "players": {
      "type": "array",
      "items": {"$ref": "#/definitions/Player"}
    },
    "turns": {
      "type": "array",
      "items": {"$ref": "#/definitions/Turn"}
    },
    "result": {
      "$ref": "#/definitions/Result"
    },
    "win_prob": {
      "type": "array",
      "description": "Per-turn win probability for each player (computed post-match)",
      "items": {
        "type": "array",
        "items": {"type": "number"}
      }
    },
    "critical_moments": {
      "type": "array",
      "description": "Turns with significant win probability shifts",
      "items": {
        "type": "object",
        "properties": {
          "turn": {"type": "integer"},
          "delta": {"type": "number"},
          "description": {"type": "string"}
        }
      }
    }
  },
  "definitions": {
    "GameConfig": {
      "type": "object",
      "properties": {
        "rows": {"type": "integer", "minimum": 30, "maximum": 120},
        "cols": {"type": "integer", "minimum": 30, "maximum": 120},
        "max_turns": {"type": "integer", "default": 500},
        "vision_radius2": {"type": "integer", "default": 49},
        "attack_radius2": {"type": "integer", "default": 5},
        "spawn_cost": {"type": "integer", "default": 3},
        "energy_interval": {"type": "integer", "default": 10}
      }
    },
    "Position": {
      "type": "object",
      "properties": {
        "row": {"type": "integer"},
        "col": {"type": "integer"}
      }
    },
    "Map": {
      "type": "object",
      "properties": {
        "rows": {"type": "integer"},
        "cols": {"type": "integer"},
        "walls": {
          "type": "array",
          "items": {"$ref": "#/definitions/Position"}
        },
        "cores": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "position": {"$ref": "#/definitions/Position"},
              "owner": {"type": "integer"}
            }
          }
        },
        "energy_nodes": {
          "type": "array",
          "items": {"$ref": "#/definitions/Position"}
        }
      }
    },
    "Player": {
      "type": "object",
      "properties": {
        "id": {"type": "integer"},
        "name": {"type": "string"},
        "bot_id": {"type": "string"}
      }
    },
    "Turn": {
      "type": "object",
      "properties": {
        "turn": {"type": "integer"},
        "bots": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "id": {"type": "integer"},
              "owner": {"type": "integer"},
              "position": {"$ref": "#/definitions/Position"},
              "alive": {"type": "boolean"}
            }
          }
        },
        "cores": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "position": {"$ref": "#/definitions/Position"},
              "owner": {"type": "integer"},
              "active": {"type": "boolean"}
            }
          }
        },
        "energy": {
          "type": "array",
          "items": {"$ref": "#/definitions/Position"},
          "description": "Energy positions visible this turn"
        },
        "scores": {
          "type": "array",
          "items": {"type": "integer"}
        },
        "energy_held": {
          "type": "array",
          "items": {"type": "integer"}
        },
        "events": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "type": {"type": "string", "enum": [
                "bot_spawned", "bot_died", "energy_collected",
                "core_captured", "combat_death", "collision_death"
              ]},
              "turn": {"type": "integer"},
              "details": {"type": "object"}
            }
          }
        }
      }
    },
    "Result": {
      "type": "object",
      "properties": {
        "winner": {"type": "integer", "description": "Player index, -1 for draw"},
        "reason": {"type": "string", "enum": [
          "sole_survivor", "dominance", "turn_limit", "annihilation"
        ]},
        "turns": {"type": "integer"},
        "scores": {"type": "array", "items": {"type": "integer"}},
        "energy": {"type": "array", "items": {"type": "integer"}},
        "bots_alive": {"type": "array", "items": {"type": "integer"}}
      }
    }
  }
}`;

export function renderDocsApiPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="docs-api-page">
      <nav class="breadcrumb">
        <a href="#/compete/docs">Docs</a> / <span>API Reference</span>
      </nav>

      <h1 class="page-title">API Reference</h1>

      <p class="intro">
        Match data is exposed as pre-computed JSON files served from the Cloudflare CDN, plus a
        small same-origin JSON API for community interactions. The Pages Endpoints and Interactive
        API sections below are live; the replay/media asset pipeline is offline, and every endpoint
        it documents is marked OFFLINE rather than advertised as fetchable.
      </p>

      <div class="api-nav">
        <h3>Quick Navigation</h3>
        <ul>
          ${sections.map(s => `<li><a href="#${slugify(s.title)}">${s.title}</a></li>`).join('')}
          <li><a href="#replay-schema">Replay JSON Schema</a></li>
          <li><a href="#fetching-pattern">Fetching Pattern</a></li>
        </ul>
      </div>

      ${sections.map(renderSection).join('')}

      <section id="replay-schema" class="schema-section">
        <h2>Replay JSON Schema</h2>
        <p>The replay format is versioned. The current version is <code>v1</code>.</p>
        <p><a href="/replay-schema-v1.json" target="_blank" class="btn secondary">Download replay-schema-v1.json</a></p>
        <p>The schema can be used with JSON Schema validators to ensure replay files conform to the expected format.</p>
        <pre><code>${escapeHtml(replaySchema)}</code></pre>
      </section>

      <section id="fetching-pattern" class="pattern-section">
        <h2>Recommended Fetching Pattern</h2>
        <p>Every live file under <code>/data/</code> is plain JSON — a plain <code>fetch</code> and
        <code>response.json()</code> is all a client needs. Replays are the one exception once the
        asset pipeline is online: they are stored gzipped and served byte-verbatim, so the client
        gunzips them (this is exactly what the site loader, <code>web/src/lib/replay-data.ts</code>, does):</p>
        <pre><code>async function fetchReplay(matchId: string): Promise<Replay> {
  const resp = await fetch(\`/data/replays/\${matchId}.json.gz\`);
  if (!resp.ok) throw new Error(\`Replay not found: \${matchId}\`);
  return decompress(await resp.arrayBuffer()); // DecompressionStream('gzip')
}</code></pre>
        <p>This path is part of the pipeline-offline contract above — it answers with data once the
        index builder bundles the warm replay set into a Pages deploy.</p>

        <h3>Cache Behavior</h3>
        <ul>
          <li><strong>Pages data files</strong>: ~90 min stale max (deploy cycle)</li>
          <li><strong>Replays</strong>: immutable, cache forever (designed; pipeline offline)</li>
          <li><strong>evolution live.json</strong>: 10 second max-age (designed; pipeline offline)</li>
        </ul>

        <h3>Rate Limits</h3>
        <p>Static file access is not rate limited — the CDN handles unlimited concurrent requests.
        The same-origin /api routes are rate limited per IP; see the Interactive API section.</p>
      </section>

      <style>
        .docs-api-page {
          max-width: 1000px;
        }

        .breadcrumb {
          color: var(--text-muted);
          font-size: 0.875rem;
          margin-bottom: 20px;
        }

        .breadcrumb a {
          color: var(--accent);
        }

        .intro {
          color: var(--text-muted);
          font-size: 1.1rem;
          margin-bottom: 30px;
          padding: 20px;
          background-color: var(--bg-secondary);
          border-radius: 8px;
          border-left: 4px solid var(--accent);
        }

        .api-nav {
          background-color: var(--bg-secondary);
          border-radius: 8px;
          padding: 20px;
          margin-bottom: 30px;
        }

        .api-nav h3 {
          margin-bottom: 12px;
          color: var(--text-primary);
        }

        .api-nav ul {
          display: flex;
          flex-wrap: wrap;
          gap: 12px;
          list-style: none;
          margin: 0;
          padding: 0;
        }

        .api-nav a {
          color: var(--accent);
          font-size: 0.875rem;
        }

        .endpoint-section {
          background-color: var(--bg-secondary);
          border-radius: 8px;
          padding: 25px;
          margin-bottom: 25px;
        }

        .endpoint-section h2 {
          color: var(--text-primary);
          margin-bottom: 8px;
        }

        .endpoint-section > p {
          color: var(--text-muted);
          margin-bottom: 20px;
        }

        .endpoint {
          background-color: var(--bg-primary);
          border-radius: 6px;
          padding: 20px;
          margin-bottom: 15px;
        }

        .endpoint:last-child {
          margin-bottom: 0;
        }

        .endpoint-header {
          display: flex;
          align-items: center;
          gap: 12px;
          margin-bottom: 12px;
        }

        .method-badge {
          background-color: #22c55e;
          color: white;
          font-size: 0.75rem;
          font-weight: 600;
          padding: 4px 8px;
          border-radius: 4px;
          text-transform: uppercase;
        }

        .endpoint-path {
          font-family: 'Fira Code', monospace;
          color: var(--text-primary);
          font-size: 0.9rem;
        }

        .endpoint-description {
          color: var(--text-muted);
          font-size: 0.875rem;
          margin-bottom: 8px;
        }

        .endpoint-cache {
          color: var(--text-muted);
          font-size: 0.75rem;
          margin-bottom: 12px;
        }

        .endpoint-cache strong {
          color: var(--text-secondary);
        }

        .endpoint pre {
          background-color: var(--bg-tertiary);
          border-radius: 4px;
          padding: 12px;
          overflow-x: auto;
          margin: 0;
        }

        .endpoint code {
          font-family: 'Fira Code', monospace;
          font-size: 0.75rem;
          color: var(--text-secondary);
        }

        .base-url {
          color: var(--accent);
          font-family: monospace;
        }

        .schema-section,
        .pattern-section {
          background-color: var(--bg-secondary);
          border-radius: 8px;
          padding: 25px;
          margin-bottom: 25px;
        }

        .schema-section h2,
        .pattern-section h2,
        .pattern-section h3 {
          color: var(--text-primary);
        }

        .pattern-section h3 {
          margin-top: 20px;
          font-size: 1rem;
        }

        .pattern-section p,
        .schema-section > p {
          color: var(--text-muted);
        }

        .pattern-section ul {
          color: var(--text-muted);
          margin-left: 20px;
        }

        .schema-section pre,
        .pattern-section pre {
          background-color: var(--bg-primary);
          border-radius: 6px;
          padding: 16px;
          overflow-x: auto;
          margin: 15px 0;
        }

        .schema-section code,
        .pattern-section code {
          font-family: 'Fira Code', monospace;
          font-size: 0.75rem;
          color: var(--text-secondary);
        }
      </style>
    </div>
  `;
}

function renderSection(section: Section): string {
  return `
    <section id="${slugify(section.title)}" class="endpoint-section">
      <h2>${section.title}</h2>
      <p>${section.description}</p>
      ${section.endpoints.map(e => renderEndpoint(e)).join('')}
    </section>
  `;
}

function renderEndpoint(endpoint: EndpointDoc): string {
  const baseUrl = PAGES_BASE;

  return `
    <div class="endpoint">
      <div class="endpoint-header">
        <span class="method-badge">${endpoint.method}</span>
        <code class="endpoint-path"><span class="base-url">${baseUrl}</span>${endpoint.path}</code>
      </div>
      <p class="endpoint-description">${endpoint.description}</p>
      <p class="endpoint-cache"><strong>Cache:</strong> ${endpoint.cache}</p>
      ${endpoint.responseExample ? `<pre><code>${escapeHtml(endpoint.responseExample)}</code></pre>` : ''}
      ${endpoint.schemaLink ? `<p><a href="${endpoint.schemaLink}">View Schema</a></p>` : ''}
    </div>
  `;
}

function slugify(text: string): string {
  return text.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}
