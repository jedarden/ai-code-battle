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
const R2_BASE = 'https://r2.aicodebattle.com';
const B2_BASE = 'https://b2.aicodebattle.com';

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
        path: '/data/blog/{slug}.json',
        description: 'Full blog post with HTML content and weekly stats.',
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
    title: 'R2 Endpoints (Warm Cache)',
    description: 'Recent replays and real-time data served from Cloudflare R2. Free tier capped at 10GB. Try R2 first, fall back to B2 for older data.',
    endpoints: [
      {
        method: 'GET',
        path: '/evolution/live.json',
        description: 'Real-time evolution observatory data. Updated every evolution cycle (~5 min) with Cache-Control: max-age=10.',
        cache: '10 seconds',
        responseExample: `{
  "updated_at": "2026-03-29T12:05:00Z",
  "total_programs": 1247,
  "promoted_count": 12,
  "islands": {
    "alpha": {"count": 312, "best_fitness": 0.85, "avg_fitness": 0.62}
  },
  "generation_log": [...],
  "lineage": [...],
  "meta_snapshots": [...]
}`,
      },
      {
        method: 'GET',
        path: '/replays/{match_id}.json.gz',
        description: 'Compressed replay file for recent matches. Contains full turn-by-turn game state.',
        cache: 'immutable (content-addressed)',
        schemaLink: '#replay-schema',
      },
      {
        method: 'GET',
        path: '/matches/{match_id}.json',
        description: 'Per-match metadata including win probability curve and critical moments.',
        cache: 'immutable (content-addressed)',
        responseExample: `{
  "match_id": "match_xyz789",
  "completed_at": "2026-03-29T11:45:00Z",
  "map_id": "map_2p_001",
  "config": {"rows": 60, "cols": 60},
  "participants": [...],
  "result": {"winner": 0, "reason": "dominance", "turns": 247},
  "win_prob": [[0.5, 0.5], [0.52, 0.48], ...],
  "critical_moments": [
    {"turn": 87, "delta": 0.22, "description": "Decisive engagement"}
  ]
}`,
      },
      {
        method: 'GET',
        path: '/cards/{bot_id}.png',
        description: 'Canvas-rendered bot profile card image (1200x630) for Open Graph social sharing.',
        cache: 'max-age=86400 (1 day)',
      },
      {
        method: 'GET',
        path: '/thumbnails/{match_id}.png',
        description: 'Auto-generated match thumbnail for embed previews.',
        cache: 'max-age=86400 (1 day)',
      },
    ],
  },
  {
    title: 'B2 Endpoints (Cold Archive)',
    description: 'Permanent archive for ALL replays and match data. Free egress via Cloudflare Bandwidth Alliance. Use as fallback when R2 returns 404.',
    endpoints: [
      {
        method: 'GET',
        path: '/replays/{match_id}.json.gz',
        description: 'Compressed replay file. All replays are archived permanently on B2.',
        cache: 'immutable (content-addressed)',
        schemaLink: '#replay-schema',
      },
      {
        method: 'GET',
        path: '/matches/{match_id}.json',
        description: 'Per-match metadata. Same structure as R2 endpoint.',
        cache: 'immutable (content-addressed)',
      },
      {
        method: 'GET',
        path: '/cards/{bot_id}.png',
        description: 'Bot profile card images. All cards archived permanently.',
        cache: 'immutable',
      },
      {
        method: 'GET',
        path: '/thumbnails/{match_id}.png',
        description: 'Match thumbnails. All thumbnails archived permanently.',
        cache: 'immutable',
      },
    ],
  },
];

// Replay JSON Schema section
const replaySchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://aicodebattle.com/schemas/replay.json",
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
        All match data is exposed as static JSON files. There is no live API for data access —
        everything is pre-computed and served from CDN. This architecture enables unlimited
        read scale with zero server cost.
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
        <pre><code>${escapeHtml(replaySchema)}</code></pre>
      </section>

      <section id="fetching-pattern" class="pattern-section">
        <h2>Recommended Fetching Pattern</h2>
        <p>For replays and match metadata, always try R2 first and fall back to B2:</p>
        <pre><code>async function fetchReplay(matchId: string): Promise<Replay> {
  // Try R2 warm cache first
  const r2Url = \`https://r2.aicodebattle.com/replays/\${matchId}.json.gz\`;
  const r2Resp = await fetch(r2Url);
  if (r2Resp.ok) {
    return decompress(await r2Resp.arrayBuffer());
  }

  // Fall back to B2 cold archive
  const b2Url = \`https://b2.aicodebattle.com/replays/\${matchId}.json.gz\`;
  const b2Resp = await fetch(b2Url);
  if (!b2Resp.ok) throw new Error(\`Replay not found: \${matchId}\`);
  return decompress(await b2Resp.arrayBuffer());
}</code></pre>

        <h3>Cache Behavior</h3>
        <ul>
          <li><strong>Pages</strong>: ~90 min stale max (deploy cycle)</li>
          <li><strong>R2 replays</strong>: immutable, cache forever</li>
          <li><strong>R2 live.json</strong>: 10 second max-age</li>
          <li><strong>B2</strong>: immutable, cache forever</li>
        </ul>

        <h3>Rate Limits</h3>
        <p>There are no rate limits on static file access. The CDN handles unlimited concurrent requests.</p>
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
      ${section.endpoints.map(e => renderEndpoint(e, section.title)).join('')}
    </section>
  `;
}

function renderEndpoint(endpoint: EndpointDoc, sectionTitle: string): string {
  let baseUrl = '';
  if (sectionTitle.includes('R2')) baseUrl = R2_BASE;
  else if (sectionTitle.includes('B2')) baseUrl = B2_BASE;
  else baseUrl = PAGES_BASE;

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
