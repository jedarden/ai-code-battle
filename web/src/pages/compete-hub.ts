// Compete hub page - participant hub with sandbox, register, docs

export function renderCompeteHubPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="compete-hub-page">
      <h1 class="page-title">Compete</h1>
      <p class="page-subtitle">Build your bot and climb the ranks</p>

      <div class="getting-started">
        <h2>Getting Started</h2>
        <p>AI Code Battle is a competitive programming platform where you write HTTP bots that control units on a grid world.</p>
      </div>

      <div class="compete-grid">
        <a href="#/compete/sandbox" class="compete-card primary">
          <div class="card-icon">🧪</div>
          <h2>Test in Sandbox</h2>
          <p>Write code and run matches in-browser with no server needed</p>
        </a>

        <a href="#/compete/register" class="compete-card primary">
          <div class="card-icon">🤖</div>
          <h2>Register Your Bot</h2>
          <p>Sign up your HTTP bot and start competing</p>
        </a>

        <a href="#/compete/docs" class="compete-card">
          <div class="card-icon">📖</div>
          <h2>Documentation</h2>
          <p>Read the protocol spec and starter kit guides</p>
        </a>

        <a href="https://github.com/aicodebattle/acb" class="compete-card" target="_blank" rel="noopener">
          <div class="card-icon">💻</div>
          <h2>Starter Kits</h2>
          <p>Example bots in Python, Go, Rust, TypeScript, and more</p>
        </a>

        <a href="#/leaderboard" class="compete-card">
          <div class="card-icon">🏆</div>
          <h2>Leaderboard</h2>
          <p>See current standings and top performers</p>
        </a>

        <a href="#/evolution" class="compete-card">
          <div class="card-icon">🧬</div>
          <h2>Evolution</h2>
          <p>Watch bots evolve through genetic algorithms</p>
        </a>
      </div>

      <div class="how-it-works">
        <h2>How Competition Works</h2>
        <div class="steps">
          <div class="step">
            <span class="step-number">1</span>
            <h3>Build a Bot</h3>
            <p>Write an HTTP server that receives game state and returns move commands</p>
          </div>
          <div class="step">
            <span class="step-number">2</span>
            <h3>Register</h3>
            <p>Submit your bot's endpoint URL and API key to start competing</p>
          </div>
          <div class="step">
            <span class="step-number">3</span>
            <h3>Climb the Ranks</h3>
            <p>Your bot plays matches automatically and earns rating through Glicko-2</p>
          </div>
        </div>
      </div>
    </div>

    <style>
      .compete-hub-page { max-width: 1200px; margin: 0 auto; }
      .page-subtitle { color: var(--text-muted); margin-bottom: 32px; }
      .getting-started { background-color: var(--bg-secondary); border-radius: 12px; padding: 24px; margin-bottom: 32px; }
      .getting-started h2 { color: var(--text-primary); margin-bottom: 12px; }
      .getting-started p { color: var(--text-muted); }
      .compete-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 20px; margin-bottom: 40px; }
      .compete-card { background-color: var(--bg-secondary); border-radius: 12px; padding: 32px 24px; text-decoration: none; transition: transform 0.2s, box-shadow 0.2s; display: block; border: 2px solid transparent; }
      .compete-card:hover { transform: translateY(-4px); box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3); }
      .compete-card.primary { border-color: var(--accent); background-color: rgba(59, 130, 246, 0.1); }
      .card-icon { font-size: 3rem; margin-bottom: 16px; }
      .compete-card h2 { color: var(--text-primary); margin-bottom: 8px; font-size: 1.25rem; }
      .compete-card p { color: var(--text-muted); font-size: 0.875rem; }
      .how-it-works { background-color: var(--bg-secondary); border-radius: 12px; padding: 32px; }
      .how-it-works h2 { color: var(--text-primary); margin-bottom: 24px; }
      .steps { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 24px; }
      .step { display: flex; flex-direction: column; gap: 12px; }
      .step-number { display: flex; align-items: center; justify-content: center; width: 48px; height: 48px; background-color: var(--accent); color: white; border-radius: 50%; font-weight: 700; font-size: 1.25rem; }
      .step h3 { color: var(--text-primary); }
      .step p { color: var(--text-muted); font-size: 0.875rem; }
    </style>
  `;
}
