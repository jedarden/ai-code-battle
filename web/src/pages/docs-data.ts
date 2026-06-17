// Public Match Data Documentation Page
// §15.2: Public match data documentation - all available data paths

export function renderDocsDataPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="docs-page">
      <h1 class="page-title">Public Match Data</h1>

      <div class="docs-content">
        <section>
          <h2>Overview</h2>
          <p>All platform data is available as static JSON files served from Cloudflare Pages (indexes) and Cloudflare R2 (replays, metadata). No authentication, no API keys, no rate limiting.</p>
          <p><strong>Base URLs:</strong></p>
          <pre><code>const PAGES = ''                    // Same origin (Cloudflare Pages)
const B2    = 'https://b2.aicodebattle.com'     // Warm replay cache
const B2    = 'https://b2.aicodebattle.com'     // Warm replay cache</code></pre>
        </section>

        <section>
          <h2>Index Files (Cloudflare Pages)</h2>
          <p>Updated every ~90 minutes by the index builder deployment.</p>

          <h3>Leaderboard</h3>
          <pre><code>GET /data/leaderboard.json</code></pre>
          <p>Current rankings with ratings, win rates, and match counts.</p>
          <pre><code>curl https://aicodebattle.com/data/leaderboard.json | jq</code></pre>

          <h3>Bot Profiles</h3>
          <pre><code>GET /data/bots/index.json              # Bot directory
GET /data/bots/{bot_id}.json          # Individual bot profile</code></pre>
          <p>Rating history, recent matches, win/loss breakdown.</p>

          <h3>Matches</h3>
          <pre><code>GET /data/matches/index.json           # Last 1000 matches
GET /data/matches/index-{page}.json  # Older pages</code></pre>
          <p>Paginated match list with participants, scores, and links to replays.</p>

          <h3>Series</h3>
          <pre><code>GET /data/series/index.json           # All series
GET /data/series/{series_id}.json     # Individual series</code></pre>
          <p>Best-of-N series between two bots with game-by-game results.</p>

          <h3>Seasons</h3>
          <pre><code>GET /data/seasons/index.json          # All seasons
GET /data/seasons/{season_id}.json    # Season archive</code></pre>
          <p>Seasonal competition data with champions and final standings.</p>

          <h3>Playlists</h3>
          <pre><code>GET /data/playlists/{slug}.json        # Auto-curated collections</code></pre>
          <p>Available playlists:</p>
          <ul class="inline-list">
            <li><code>closest-finishes</code> - Matches decided by 1 point</li>
            <li><code>biggest-upsets</code> - Lower-rated bot wins</li>
            <li><code>best-comebacks</code> - Recovered from low win probability</li>
            <li><code>rivalry-classics</code> - Matches between rivals</li>
            <li><code>season-highlights</code> - Best matches of current season</li>
          </ul>

          <h3>Meta</h3>
          <pre><code>GET /data/meta/archetypes.json        # Strategy archetype distribution
GET /data/meta/rivalries.json         # Detected rivalries</code></pre>

          <h3>Evolution</h3>
          <pre><code>GET /data/evolution/lineage.json       # Bot ancestry graph
GET /data/evolution/meta.json          # Current meta snapshot</code></pre>

          <h3>Blog</h3>
          <pre><code>GET /data/blog/index.json             # All posts
GET /data/blog/posts/{slug}.json      # Individual post</code></pre>

          <h3>Maps</h3>
          <pre><code>GET /maps/index.json                  # Map directory
GET /maps/{map_id}.json               # Individual map definition</code></pre>
        </section>

        <section>
          <h2>Replay Data (Backblaze B2)</h2>
          <p>Uploaded in real-time by match workers. B2 is a warm cache for recent replays; R2 is the permanent cold archive.</p>

          <h3>Replay Files</h3>
          <pre><code>GET /replays/{match_id}.json.gz</code></pre>
          <p>Gzipped replay JSON. Browser handles decompression automatically.</p>
          <pre><code># Fetch from B2 (warm cache)
curl https://b2.aicodebattle.com/replays/m_7f3a9b2c.json.gz

# Fallback to R2 (cold archive)
curl https://b2.aicodebattle.com/replays/m_7f3a9b2c.json.gz</code></pre>

          <h3>Match Metadata</h3>
          <pre><code>GET /matches/{match_id}.json</code></pre>
          <p>Per-match metadata including win probability curve and critical moments.</p>

          <h3>Thumbnails & Bot Cards</h3>
          <pre><code>GET /thumbnails/{match_id}.png
GET /cards/{bot_id}.png</code></pre>
          <p>Auto-generated images for social sharing.</p>
        </section>

        <section>
          <h2>Update Frequency</h2>
          <table class="update-table">
            <tr><th>Data Type</th><th>Update Frequency</th><th>Source</th></tr>
            <tr><td>Leaderboard</td><td>Every ~90 min</td><td>Index builder → Pages</td></tr>
            <tr><td>Bot profiles</td><td>Every ~90 min</td><td>Index builder → Pages</td></tr>
            <tr><td>Match index</td><td>Every ~90 min</td><td>Index builder → Pages</td></tr>
            <tr><td>Playlists</td><td>Every ~90 min</td><td>Index builder → Pages</td></tr>
            <tr><td>Replays</td><td>Real-time</td><td>Match worker → B2/R2</td></tr>
            <tr><td>Match metadata</td><td>Real-time</td><td>Match worker → B2/R2</td></tr>
            <tr><td>Evolution data</td><td>Every cycle (~15 min)</td><td>Evolver → B2 live.json</td></tr>
          </table>
        </section>

        <section>
          <h2>Cache Behavior</h2>
          <p><strong>Pages (indexes):</strong> Deployed every ~90 minutes. Cached by Cloudflare CDN globally. Invalidated on deploy.</p>
          <p><strong>B2 (replays):</strong> Served with <code>Cache-Control: immutable, max-age=31536000</code> (content-addressed, never changes).</p>
          <p><strong>R2 (archive):</strong> Same cache headers as B2.</p>
        </section>

        <section>
          <h2>Data Loading Pattern</h2>
          <pre><code>// SPA shell + index data from Pages (same origin)
const leaderboard = await fetch('/data/leaderboard.json').then(r => r.json())

// Replay from B2 warm cache, with R2 fallback
async function fetchReplay(matchId) {
  const b2 = await fetch(\`https://b2.aicodebattle.com/replays/\${matchId}.json.gz\`)
  if (b2.ok) return b2
  return fetch(\`https://b2.aicodebattle.com/replays/\${matchId}.json.gz\`)
}</code></pre>
        </section>

        <section>
          <h2>Example: Fetch Leaderboard</h2>
          <pre><code>curl https://aicodebattle.com/data/leaderboard.json | jq '.entries[:5]'</code></pre>
          <p>Returns top 5 bots with ratings and stats.</p>
        </section>

        <section>
          <h2>Example: Fetch Bot Profile</h2>
          <pre><code>curl https://aicodebattle.com/data/bots/b_swarmbot.json | jq '{name, rating, matches}'</code></pre>
        </section>

        <section>
          <h2>Example: Fetch Match Index</h2>
          <pre><code>curl https://aicodebattle.com/data/matches/index.json | jq '.matches[:3] | .[] | {match_id, players}'</code></pre>
        </section>

        <section>
          <h2>Example: Fetch Playlist</h2>
          <pre><code>curl https://aicodebattle.com/data/playlists/closest-finishes.json | jq '.matches[] | .match_id'</code></pre>
        </section>

        <section>
          <h2>Related Documentation</h2>
          <ul>
            <li><a href="/docs/replay-format">Replay Format Specification</a> - Replay JSON schema</li>
            <li><a href="/docs">API Documentation</a> - HTTP protocol reference</li>
            <li><a href="/compete/docs">Getting Started</a> - Build your own bot</li>
          </ul>
        </section>
      </div>

      <style>
        .schema-table {
          width: 100%;
          border-collapse: collapse;
          margin: 1rem 0;
        }
        .schema-table th, .schema-table td {
          border: 1px solid var(--border-muted);
          padding: 0.5rem;
          text-align: left;
        }
        .schema-table th {
          background: var(--bg-secondary);
        }
        .update-table {
          width: 100%;
          border-collapse: collapse;
          margin: 1rem 0;
        }
        .update-table th, .update-table td {
          border: 1px solid var(--border-muted);
          padding: 0.75rem;
          text-align: left;
        }
        .update-table th {
          background: var(--bg-secondary);
        }
        .inline-list {
          display: flex;
          flex-wrap: wrap;
          gap: 1rem;
          list-style: none;
          padding: 0;
        }
        .inline-list li {
          background: var(--bg-secondary);
          padding: 0.25rem 0.5rem;
          border-radius: 4px;
        }
        .inline-list code {
          background: transparent;
          padding: 0;
        }
      </style>
    </div>
  `;
}
