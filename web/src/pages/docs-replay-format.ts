// Replay Format Documentation Page
// §15.2: Public match data documentation - replay format specification

export function renderDocsReplayFormatPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="docs-page">
      <h1 class="page-title">Replay Format Specification</h1>

      <div class="docs-content">
        <section>
          <h2>Overview</h2>
          <p>AI Code Battle replays are JSON files containing the complete state of a match. Each replay includes the initial configuration, map data, and turn-by-turn events that allow the game to be reconstructed and visualized.</p>
          <p><strong>Version:</strong> v1 (additive changes only - see <a href="#changelog">Changelog</a> below)</p>
        </section>

        <section>
          <h2>Fetching Replays</h2>
          <p>Replays are served from Cloudflare R2 (warm cache) with B2 (cold archive) fallback:</p>
          <pre><code># Try warm cache first
curl https://r2.aicodebattle.com/replays/\${match_id}.json.gz

# Fallback to cold archive
curl https://b2.aicodebattle.com/replays/\${match_id}.json.gz</code></pre>
          <p>Replays are gzip-compressed. The browser handles decompression automatically when you fetch with <code>Accept-Encoding: gzip</code>.</p>
        </section>

        <section>
          <h2>Replay Schema</h2>
          <p>Download the JSON Schema: <a href="/replay-schema-v1.json" target="_blank">replay-schema-v1.json</a></p>
          <p>Use the schema to validate replays programmatically:</p>
          <pre><code># Validate a replay file
ajv validate -s replay-schema-v1.json -d replay.json</code></pre>
        </section>

        <section>
          <h2>Top-Level Structure</h2>
          <pre><code>{
  "version": 1,                    // Replay format version
  "match_id": "m_7f3a9b2c",       // Unique match identifier
  "date": "2026-03-23T14:30:00Z", // Match completion time
  "players": [...],               // Player metadata
  "result": {...},                // Final scores and winner
  "config": {...},                // Match configuration
  "map": {...},                   // Map definition
  "turns": [...]                  // Turn-by-turn events
}</code></pre>
        </section>

        <section>
          <h2>Player Metadata</h2>
          <pre><code>"players": [
  {
    "bot_id": "b_4e8c1d2f",
    "name": "SwarmBot",
    "owner": "alice",
    "color": "#2196F3"
  }
]</code></pre>
          <table class="schema-table">
            <tr><th>Field</th><th>Type</th><th>Description</th></tr>
            <tr><td>bot_id</td><td>string</td><td>Unique bot identifier</td></tr>
            <tr><td>name</td><td>string</td><td>Bot display name</td></tr>
            <tr><td>owner</td><td>string</td><td>Bot owner's username</td></tr>
            <tr><td>color</td><td>string</td><td>Player color for visualization</td></tr>
          </table>
        </section>

        <section>
          <h2>Match Result</h2>
          <pre><code>"result": {
  "winner": 0,                   // Winning player index
  "condition": "turn_limit",     // Win condition
  "final_scores": [7, 3],        // Final scores per player
  "final_energy": [12, 4],       // Final energy held
  "final_bots": [18, 6]          // Final bot count
}</code></pre>
          <p><strong>Win Conditions:</strong></p>
          <ul>
            <li><code>sole_survivor</code> - Only one player has living bots</li>
            <li><code>annihilation</code> - All players eliminated simultaneously</li>
            <li><code>dominance</code> - One player controls ≥80% of bots for 100 turns</li>
            <li><code>turn_limit</code> - Turn limit reached (default: 500)</li>
          </ul>
        </section>

        <section>
          <h2>Match Configuration</h2>
          <pre><code>"config": {
  "rows": 60,
  "cols": 60,
  "max_turns": 500,
  "vision_radius2": 49,          // Squared vision radius (~7 tiles)
  "attack_radius2": 12,          // Squared attack radius
  "spawn_cost": 3,              // Energy to spawn a bot
  "energy_interval": 10          // Turns between energy spawns
}</code></pre>
          <p>Seasonal variations may introduce optional fields (see <a href="#changelog">Changelog</a>). Bots that don't read new fields continue to work.</p>
        </section>

        <section>
          <h2>Map Definition</h2>
          <pre><code>"map": {
  "walls": [[10,10], [10,11], [10,12]],           // Wall positions
  "energy_nodes": [[20,25], [40,35]],             // Energy spawn locations
  "cores": [                                      // Starting cores
    {"pos": [5,5], "owner": 0},
    {"pos": [55,55], "owner": 1}
  ]
}</code></pre>
        </section>

        <section>
          <h2>Turn Events</h2>
          <p>Each turn contains events that occurred during that turn:</p>
          <pre><code>"turns": [
  {
    "moves": {                      // Moves made by each player
      "0": [{"from": [10,15], "dir": "N"}],
      "1": [{"from": [50,45], "dir": "S"}]
    },
    "spawns": [[5,5,0]],            // New bots spawned
    "deaths": [[30,40,1]],          // Bots that died
    "captures": [],                  // Cores captured
    "energy_collected": {           // Energy gathered
      "0": [[20,25]]
    },
    "energy_spawned": [[35,15]],   // New energy appeared
    "scores": [3, 1],              // Scores after turn
    "events": [                     // Detailed events
      {
        "type": "combat_death",
        "turn": 6,
        "details": {
          "bot_id": 0,
          "owner": 0,
          "position": {"row": 30, "col": 40},
          "killers": [
            {"bot_id": 1, "owner": 1, "position": {"row": 28, "col": 42}}
          ]
        }
      }
    ]
  }
]</code></pre>
        </section>

        <section>
          <h2>Event Types</h2>
          <table class="schema-table">
            <tr><th>Type</th><th>Description</th></tr>
            <tr><td><code>bot_spawned</code></td><td>A new bot was spawned</td></tr>
            <tr><td><code>bot_died</code></td><td>A bot died (legacy, no killer info)</td></tr>
            <tr><td><code>combat_death</code></td><td>A bot died from focus-fire combat (includes killers[] array)</td></tr>
            <tr><td><code>collision_death</code></td><td>Two bots moved to the same tile</td></tr>
            <tr><td><code>zone_death</code></td><td>A bot was killed by the shrinking zone</td></tr>
            <tr><td><code>energy_collected</code></td><td>Energy was gathered</td></tr>
            <tr><td><code>core_captured</code></td><td>An enemy core was razed</td></tr>
          </table>
        </section>

        <section>
          <h2>Win Probability (Optional)</h2>
          <p>Some replays include a win probability curve computed via Monte Carlo rollout:</p>
          <pre><code>"win_prob": [
  [0.50, 0.50],  // Turn 0: even odds
  [0.51, 0.49],  // Turn 1: slight edge to player 0
  ...
]</code></pre>
          <p>Array of [player_0_prob, player_1_prob, ...] for each turn.</p>
        </section>

        <section>
          <h2>Critical Moments (Optional)</h2>
          <p>Turns where win probability shifted significantly (>15%):</p>
          <pre><code>"critical_moments": [
  {
    "turn": 87,
    "delta": 0.22,
    "description": "SwarmBot loses 6 units in eastern engagement"
  }
]</code></pre>
        </section>

        <section id="changelog">
          <h2>Changelog</h2>
          <h3>Version 1 (Current)</h3>
          <ul>
            <li>Initial release</li>
            <li>All core event types supported</li>
            <li>Optional win_prob and critical_moments arrays</li>
          </ul>
          <p><strong>Backward Compatibility Policy:</strong> Future versions will only add optional fields. Existing fields will never be removed or renamed. Bots that don't read new fields continue to function.</p>
        </section>

        <section>
          <h2>Example Replays</h2>
          <p>Download example replays to test your visualization:</p>
          <ul>
            <li><a href="/data/demo-replay-v2.json" download>2-Player Demo Replay</a></li>
            <li><a href="/data/demo-replay-v2-6p.json" download>6-Player Demo Replay</a></li>
            <li><a href="/data/real-replay.json" download>Full-Length Production Replay</a></li>
          </ul>
        </section>

        <section>
          <h2>Related Documentation</h2>
          <ul>
            <li><a href="/docs/data">Public Data Paths</a> - All available JSON endpoints</li>
            <li><a href="/compete/docs">Getting Started</a> - Build your own bot</li>
            <li><a href="/docs">API Documentation</a> - HTTP protocol reference</li>
          </ul>
        </section>
      </div>
    </div>
  `;
}
