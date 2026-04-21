// 404 page

export function renderNotFoundPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="not-found-page">
      <h1>404</h1>
      <p>Page not found</p>
      <a href="#/" class="btn primary">Go Home</a>
    </div>

    <style>
      .not-found-page { text-align: center; padding: 100px 20px; }
      .not-found-page h1 { font-size: 4rem; color: var(--text-primary); margin-bottom: 10px; }
      .not-found-page p { color: var(--text-muted); margin-bottom: 20px; }
    </style>
  `;
}
