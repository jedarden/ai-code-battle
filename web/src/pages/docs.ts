// Docs/Getting Started page

export function renderDocsPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="docs-page">
      <h1 class="page-title">Getting Started</h1>

      <div class="docs-content">
        <section>
          <h2>Overview</h2>
          <p>AI Code Battle is a competitive bot programming platform. You write an HTTP server that controls units on a grid world, competing against other bots for supremacy.</p>
        </section>

        <section>
          <h2>Game Basics</h2>
          <ul>
            <li><strong>Grid:</strong> The game is played on a toroidal (wrapping) grid</li>
            <li><strong>Units:</strong> Each player controls bots that move one tile per turn</li>
            <li><strong>Resources:</strong> Collect energy from nodes to spawn new bots</li>
            <li><strong>Objectives:</strong> Capture enemy cores, eliminate opponents, or dominate through numbers</li>
          </ul>
        </section>

        <section>
          <h2>HTTP Protocol</h2>
          <p>Your bot must expose an HTTPS endpoint that accepts POST requests with JSON game state and returns JSON move commands.</p>

          <h3>Request Format</h3>
          <pre><code>{
  "match_id": "abc123",
  "turn": 42,
  "player_id": 0,
  "config": { ... },
  "visible_grid": { ... },
  "my_bots": [
    { "id": "bot-1", "position": {"row": 10, "col": 20} }
  ],
  "my_energy": 5,
  "my_score": 3
}</code></pre>

          <h3>Response Format</h3>
          <pre><code>{
  "moves": [
    { "bot_id": "bot-1", "direction": "N" }
  ]
}</code></pre>

          <h3>Valid Directions</h3>
          <p><code>N</code> (North), <code>E</code> (East), <code>S</code> (South), <code>W</code> (West)</p>
        </section>

        <section>
          <h2>Authentication</h2>
          <p>Requests from the game engine are signed with HMAC-SHA256. The signature is sent in the <code>X-Signature</code> header.</p>
          <p>Format: <code>{match_id}.{turn}.{timestamp}.{sha256(body)}</code></p>
          <p>Your bot should verify signatures using your API key to ensure requests are authentic.</p>
        </section>

        <section>
          <h2>Requirements</h2>
          <ul>
            <li>HTTPS endpoint accessible from the internet</li>
            <li>Response time under 3 seconds per turn</li>
            <li>Handle concurrent requests (multiple matches)</li>
            <li>Return valid JSON for every request</li>
          </ul>
        </section>

        <section>
          <h2>Starter Kits</h2>
          <p>Fork a starter kit to get a working bot in minutes. Each includes an HTTP server scaffold, HMAC authentication, game types, and a random strategy you can replace with your own.</p>
          <ul class="starter-links">
            <li><a href="https://github.com/jedarden/acb-starter-python" target="_blank">Python 3</a> — stdlib HTTP server, zero dependencies</li>
            <li><a href="https://github.com/jedarden/acb-starter-go" target="_blank">Go</a> — net/http, single-binary deploy</li>
            <li><a href="https://github.com/jedarden/acb-starter-javascript" target="_blank">JavaScript (Node.js)</a> — zero dependencies, built-in http module</li>
            <li><a href="https://github.com/jedarden/acb-starter-rust" target="_blank">Rust</a> — axum + serde, minimal binary</li>
            <li><a href="https://github.com/jedarden/acb-starter-java" target="_blank">Java</a> — Javalin, Maven-based</li>
            <li><a href="https://github.com/jedarden/acb-starter-csharp" target="_blank">C# (.NET)</a> — ASP.NET Core minimal API</li>
          </ul>
        </section>

        <section>
          <h2>Register Your Bot</h2>
          <p>Once your bot is deployed and accessible via HTTPS, register it:</p>
          <pre><code>curl -X POST https://api.aicodebattle.com/api/register \\
  -H "Content-Type: application/json" \\
  -d '{
    "name": "my-bot",
    "endpoint_url": "https://my-bot.example.com",
    "owner": "your-name",
    "description": "My awesome bot"
  }'</code></pre>
          <p>The response contains your <code>bot_id</code> and <code>shared_secret</code>. Save the secret — it's shown only once.</p>
        </section>

        <section>
          <h2>Data &amp; API</h2>
          <p>All match data (leaderboards, replays, bot profiles) is exposed as static JSON files served from CDN.</p>
          <p><a href="#/compete/docs" class="btn secondary">View API Reference</a></p>
        </section>
      </div>
    </div>

    <style>
      .docs-content { max-width: 800px; }
      .docs-content section { background-color: var(--bg-secondary); border-radius: 8px; padding: 20px; margin-bottom: 20px; }
      .docs-content h2 { color: var(--text-primary); margin-bottom: 12px; }
      .docs-content h3 { color: var(--text-primary); margin: 16px 0 8px; font-size: 1rem; }
      .docs-content p { color: var(--text-muted); margin-bottom: 10px; }
      .docs-content ul { color: var(--text-muted); margin-left: 20px; }
      .docs-content li { margin-bottom: 6px; }
      .docs-content pre { background-color: var(--bg-primary); border-radius: 6px; padding: 16px; overflow-x: auto; margin: 10px 0; }
      .docs-content code { font-family: 'Fira Code', 'Monaco', monospace; font-size: 0.875rem; color: var(--text-secondary); }
      .docs-content a { color: var(--accent); }
      .starter-links { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 8px; }
      .starter-links li { margin-bottom: 0; }
    </style>
  `;
}
