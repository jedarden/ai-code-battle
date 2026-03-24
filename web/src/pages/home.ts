// Home page - landing page with overview

export function renderHomePage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="home-page">
      <section class="hero">
        <h1>AI Code Battle</h1>
        <p class="tagline">Program your bot. Compete for supremacy.</p>
        <p class="description">
          Write an HTTP server that controls units on a grid world.
          Collect energy, capture cores, and eliminate your opponents.
        </p>
        <div class="cta-buttons">
          <button class="btn primary" onclick="window.location.hash='/register'">Register Your Bot</button>
          <button class="btn secondary" onclick="window.location.hash='/docs'">Get Started</button>
        </div>
      </section>

      <section class="features">
        <h2>How It Works</h2>
        <div class="feature-grid">
          <div class="feature">
            <h3>Write Code</h3>
            <p>Create a bot in any language that exposes an HTTP endpoint.
               Your bot receives game state and returns moves each turn.</p>
          </div>
          <div class="feature">
            <h3>Deploy</h3>
            <p>Host your bot anywhere - cloud, container, or bare metal.
               Just make sure it's accessible via HTTP.</p>
          </div>
          <div class="feature">
            <h3>Compete</h3>
            <p>Your bot plays matches automatically against other registered bots.
               Climb the leaderboard with victories.</p>
          </div>
          <div class="feature">
            <h3>Watch</h3>
            <p>View replays of every match. Analyze strategies,
               learn from defeats, and improve your bot.</p>
          </div>
        </div>
      </section>

      <section class="quick-links">
        <h2>Explore</h2>
        <div class="link-grid">
          <a href="#/leaderboard" class="link-card">
            <h3>Leaderboard</h3>
            <p>See how bots rank on the competitive ladder</p>
          </a>
          <a href="#/matches" class="link-card">
            <h3>Match History</h3>
            <p>Browse recent matches and watch replays</p>
          </a>
          <a href="#/bots" class="link-card">
            <h3>Bot Directory</h3>
            <p>View all registered bots and their profiles</p>
          </a>
          <a href="#/replay" class="link-card">
            <h3>Replay Viewer</h3>
            <p>Load and watch match replays</p>
          </a>
        </div>
      </section>
    </div>
  `;
}
