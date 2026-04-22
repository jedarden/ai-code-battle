// Main SPA entry point with routing
// Code splitting: all pages are loaded on-demand via dynamic import() to keep
// the initial bundle small.  The app entry chunk contains only the router,
// navigation, lazy-loading wrappers — no page renderers.
// §16.14: preload-on-hover, skeleton screens, instant back-cache

import { router } from './router';
import type { RouteHandler } from './router';
import {
  initPerformanceFeatures,
  savePageCache,
  restorePageFromCache,
  hasPageCache,
  fadeInContent,
} from './lib/preload';
import {
  skeletonLeaderboard,
  skeletonBotProfile,
  skeletonReplay,
  skeletonPlaylists,
  skeletonMatches,
  skeletonEvolution,
  skeletonBlog,
  skeletonSeasons,
  skeletonGeneric,
} from './components/skeleton';

// ─── Skeleton route mapping ──────────────────────────────────────────────────
// Returns skeleton HTML for a given path so the user sees layout immediately.

function getSkeletonHtml(path: string): string {
  if (path === '/leaderboard' || path === '/bots') return skeletonLeaderboard();
  if (path.startsWith('/bot/') || path.startsWith('/compete/bot/')) return skeletonBotProfile();
  if (path.startsWith('/watch/replay/') || path.startsWith('/replay/')) return skeletonReplay();
  if (path.startsWith('/watch/playlists')) return skeletonPlaylists();
  if (path === '/watch/replays' || path === '/matches') return skeletonMatches();
  if (path === '/evolution') return skeletonEvolution();
  if (path.startsWith('/blog')) return skeletonBlog();
  if (path === '/seasons' || path.startsWith('/season/')) return skeletonSeasons();
  if (path === '/watch/predictions' || path === '/predictions') return skeletonGeneric('Predictions');
  if (path === '/watch') return skeletonGeneric('Watch');
  if (path === '/') return ''; // Home page has its own rich skeleton built in
  return skeletonGeneric('Loading');
}

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
// Rivalries page (pre-computed from index builder §13.5)
const loadRivalriesPage = () => import('./pages/rivalries').then(m => m.renderRivalriesPage);

// 404
const loadNotFoundPage = () => import('./pages/not-found').then(m => m.renderNotFoundPage);

// ─── Helper: wrap async page loader in sync RouteHandler ────────────────────────
// Shows skeleton immediately, then loads the real page async with fade-in.
function lazyRoute(loader: () => Promise<(params: Record<string, string>) => void>): RouteHandler {
  return (params: Record<string, string>) => {
    const targetPath = router.getCurrentPath();

    // Check back-cache for instant restore
    if (hasPageCache(targetPath)) {
      restorePageFromCache(targetPath);
      return;
    }

    // Show skeleton immediately while loading the chunk
    const skeleton = getSkeletonHtml(targetPath);
    if (skeleton) {
      const app = document.getElementById('app');
      if (app) app.innerHTML = skeleton;
    }

    loader().then(handler => {
      handler(params);
      // Fade in real content over the skeleton
      const app = document.getElementById('app');
      if (app && skeleton) fadeInContent(app);
    });
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

// ─── Back-cache: save current page before navigating away ──────────────────────

router.beforeNavigate((from: string, _to: string) => {
  // Only cache pages that have rendered content (not initial load)
  if (from && from !== '/') {
    savePageCache(from);
  }

  // Cleanup VirtualList instances to prevent leaked ResizeObservers
  const app = document.getElementById('app');
  if (app) {
    app.querySelectorAll<HTMLElement>('[data-virtual-list]').forEach(el => {
      const vl = (el as any)._virtualList;
      if (vl && typeof vl.destroy === 'function') vl.destroy();
    });
  }
});

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
  .on('/rivalries', lazyRoute(loadRivalriesPage))
  .on('/feedback', lazyRoute(loadFeedbackPage))
  .on('/compete/feedback', lazyRoute(loadFeedbackPage))
  .on('/compete/docs/api', lazyRoute(loadDocsApiPage))
  .notFound(lazyRoute(loadNotFoundPage));

// ─── Initialization ────────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  updateActiveNavLink();
  router.start();
  // §16.14: activate hover preloading
  initPerformanceFeatures();
});

window.addEventListener('load', () => {
  updateActiveNavLink();
});
