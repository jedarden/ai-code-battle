// Main SPA entry point with routing
// Code splitting: all pages are loaded on-demand via dynamic import() to keep
// the initial bundle small.  The app entry chunk contains only the router,
// navigation, and lazy-loading wrappers — no page renderers.
import { router } from './router';
import type { RouteHandler } from './router';

// ─── Lazy loaders for code splitting ─────────────────────────────────────────────
// Each loader creates its own chunk, loaded only when the route is visited

// Core pages - loaded frequently
const loadHomePage = () => import('./pages/home').then(m => m.renderHomePage);
const loadLeaderboardPage = () => import('./pages/leaderboard').then(m => m.renderLeaderboardPage);

// Watch section - replay viewer and related pages
const loadMatchesPage = () => import('./pages/matches').then(m => m.renderMatchesPage);
const loadPlaylistsPage = () => import('./pages/playlists').then(m => m.renderPlaylistsPage);
const loadSeriesPage = () => import('./pages/series').then(m => m.renderSeriesPage);
const loadPredictionsPage = () => import('./pages/predictions').then(m => m.renderPredictionsPage);
const loadReplayPage = () => import('./pages/replay').then(m => m.renderReplayPage);
const loadWatchHubPage = () => import('./pages/watch-hub').then(m => m.renderWatchHubPage);

// Compete section - sandbox, register, docs
const loadSandboxPage = () => import('./pages/sandbox').then(m => m.renderSandboxPage);
const loadRegisterPage = () => import('./pages/register').then(m => m.renderRegisterPage);
const loadCompeteHubPage = () => import('./pages/compete-hub').then(m => m.renderCompeteHubPage);
const loadDocsPage = () => import('./pages/docs').then(m => m.renderDocsPage);

// Bot-related pages
const loadBotProfilePage = () => import('./pages/bot-profile').then(m => m.renderBotProfilePage);
const loadEvolutionPage = () => import('./pages/evolution').then(m => m.renderEvolutionPage);

// Blog & seasons
const loadBlogPages = () => import('./pages/blog').then(m => ({ renderBlogPage: m.renderBlogPage, renderBlogPostPage: m.renderBlogPostPage }));
const loadSeasonsPage = () => import('./pages/seasons').then(m => m.renderSeasonsPage);
const loadSeasonDetailPage = () => import('./pages/season-detail').then(m => m.renderSeasonDetailPage);

// Feedback & docs (separate chunk - includes replay viewer for feedback page)
// Feedback page lazy-loads with agentation (loaded on /#/feedback or explicit enable)
// Agentation is NOT imported here — only loaded when feedback page is visited
const loadFeedbackPage = () => import('./pages/feedback').then(async m => {
  const { initAgentation } = await import('./agentation-overlay');
  initAgentation();
  return m.renderFeedbackPage;
});
// Docs API page (separate chunk from compete docs)
const loadDocsApiPage = () => import('./pages/docs-api').then(m => m.renderDocsApiPage);

// 404
const loadNotFoundPage = () => import('./pages/not-found').then(m => m.renderNotFoundPage);

// ─── Helper: wrap async page loader in sync RouteHandler ────────────────────────
function lazyRoute(loader: () => Promise<(params: Record<string, string>) => void>): RouteHandler {
  return (params: Record<string, string>) => {
    loader().then(handler => handler(params));
  };
}

// ─── Backwards compatibility redirects ────────────────────────────────────────────
function redirect(to: string): RouteHandler {
  return (params: Record<string, string>) => {
    const fullPath = Object.entries(params).reduce(
      (path, [key, value]) => path.replace(`:${key}`, encodeURIComponent(value)),
      to
    );
    router.navigate(fullPath);
  };
}

// ─── Navigation & UI ───────────────────────────────────────────────────────────────

function updateActiveNavLink(): void {
  const currentPath = router.getCurrentPath();

  document.querySelectorAll('.nav-link').forEach(link => {
    link.classList.remove('active');
  });

  document.querySelectorAll('.nav-link').forEach(link => {
    const href = link.getAttribute('href');
    if (href) {
      const linkPath = href.slice(2);
      if (currentPath === linkPath ||
          (linkPath !== '' && currentPath.startsWith(linkPath)) ||
          (linkPath === '/watch' && currentPath.startsWith('/watch')) ||
          (linkPath === '/compete' && currentPath.startsWith('/compete'))) {
        link.classList.add('active');
      }
    }
  });
}

function initMobileMenu(): void {
  const toggle = document.getElementById('mobile-menu-toggle');
  const menu = document.getElementById('mobile-menu');

  if (!toggle || !menu) return;

  toggle.addEventListener('click', () => {
    menu.classList.toggle('open');
  });

  document.addEventListener('click', (e) => {
    if (!menu.contains(e.target as Node) && !toggle.contains(e.target as Node)) {
      menu.classList.remove('open');
    }
  });

  const originalNavigate = router.navigate.bind(router);
  router.navigate = (path: string) => {
    originalNavigate(path);
    menu.classList.remove('open');
  };
}

initMobileMenu();

const originalNavigate = router.navigate.bind(router);
router.navigate = (path: string) => {
  originalNavigate(path);
  updateActiveNavLink();
};

// ─── Route definitions ─────────────────────────────────────────────────────────────

router
  // Main routes
  .on('/', lazyRoute(loadHomePage))
  .on('/watch', lazyRoute(loadWatchHubPage))
  .on('/watch/replays', lazyRoute(loadMatchesPage))
  .on('/watch/playlists', lazyRoute(loadPlaylistsPage))
  .on('/watch/playlists/:slug', lazyRoute(loadPlaylistsPage))
  .on('/watch/replay/:id', lazyRoute(loadReplayPage))
  .on('/watch/series/:id', lazyRoute(loadSeriesPage))
  .on('/watch/predictions', lazyRoute(loadPredictionsPage))
  .on('/watch/series', lazyRoute(loadSeriesPage))
  .on('/compete', lazyRoute(loadCompeteHubPage))
  .on('/compete/sandbox', lazyRoute(loadSandboxPage))
  .on('/compete/register', lazyRoute(loadRegisterPage))
  .on('/compete/bot/:id', lazyRoute(loadBotProfilePage))
  .on('/compete/docs', lazyRoute(loadDocsPage))
  .on('/leaderboard', lazyRoute(loadLeaderboardPage))
  .on('/evolution', lazyRoute(loadEvolutionPage))
  .on('/blog', lazyRoute(async () => (await loadBlogPages()).renderBlogPage))
  .on('/blog/:slug', lazyRoute(async () => (await loadBlogPages()).renderBlogPostPage))
  .on('/season/:id', lazyRoute(loadSeasonDetailPage))
  .on('/seasons', lazyRoute(loadSeasonsPage))
  .on('/bot/:id', lazyRoute(loadBotProfilePage))
  // Backwards compatibility redirects
  .on('/matches', redirect('/watch/replays'))
  .on('/playlists', redirect('/watch/playlists'))
  .on('/replay', redirect('/watch/replay'))
  .on('/predictions', redirect('/watch/predictions'))
  .on('/series', redirect('/watch/series'))
  .on('/sandbox', redirect('/compete/sandbox'))
  .on('/register', redirect('/compete/register'))
  .on('/bots', redirect('/leaderboard'))
  .on('/docs', redirect('/compete/docs'))
  .on('/docs/api', redirect('/compete/docs'))
  .on('/clip-maker', redirect('/watch/replays'))
  .on('/rivalries', redirect('/watch/replays'))
  .on('/feedback', lazyRoute(loadFeedbackPage))
  .on('/compete/feedback', lazyRoute(loadFeedbackPage))
  .on('/compete/docs/api', lazyRoute(loadDocsApiPage))
  .notFound(lazyRoute(loadNotFoundPage));

// ─── Initialization ────────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  updateActiveNavLink();
  router.start();
});

window.addEventListener('load', () => {
  updateActiveNavLink();
});
