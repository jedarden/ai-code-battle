// §16.14 Performance trifecta: preload-on-hover + instant back-cache
// Preload fetches data into both the browser HTTP cache (via <link rel=prefetch>)
// and the SWR application cache (via manual fetch + seedSwrCache).

import { seedSwrCache } from '../api-types';

// ─── Route → data URL + SWR key mapping ────────────────────────────────────────
// Maps SPA routes to the JSON data files they fetch so we can prefetch on hover.
// Each entry includes the SWR cache key so preloaded data populates the app cache.

interface DataMapping {
  url: string;
  swrKey: string;
}

type DataMappingFactory = (params: Record<string, string>) => DataMapping[];

const ROUTE_DATA: Array<{ pattern: RegExp; paramNames: string[]; mappings: DataMappingFactory }> = [];

function registerRouteData(pattern: string, mappings: DataMappingFactory): void {
  const paramNames: string[] = [];
  const regexPattern = pattern.replace(/:(\w+)/g, (_, name) => {
    paramNames.push(name);
    return '([^/]+)';
  });
  ROUTE_DATA.push({ pattern: new RegExp(`^${regexPattern}$`), paramNames, mappings });
}

// Static routes
registerRouteData('/', () => [
  { url: '/data/leaderboard.json', swrKey: 'leaderboard' },
  { url: '/data/playlists/index.json', swrKey: 'playlist-index' },
  { url: '/data/evolution/meta.json', swrKey: 'evolution-meta' },
]);
registerRouteData('/leaderboard', () => [
  { url: '/data/leaderboard.json', swrKey: 'leaderboard' },
]);
registerRouteData('/watch', () => [
  { url: '/data/playlists/index.json', swrKey: 'playlist-index' },
  { url: '/data/matches/index.json', swrKey: 'match-index' },
]);
registerRouteData('/watch/replays', () => [
  { url: '/data/matches/index.json', swrKey: 'match-index' },
]);
registerRouteData('/watch/playlists', () => [
  { url: '/data/playlists/index.json', swrKey: 'playlist-index' },
]);
registerRouteData('/watch/predictions', () => [
  { url: '/data/predictions/leaderboard.json', swrKey: 'predictions-leaderboard' },
]);
registerRouteData('/evolution', () => [
  { url: '/data/evolution/meta.json', swrKey: 'evolution-meta' },
  { url: '/data/evolution/lineage.json', swrKey: 'evolution-lineage' },
]);
registerRouteData('/blog', () => [
  { url: '/data/blog/index.json', swrKey: 'blog-index' },
]);
registerRouteData('/seasons', () => [
  { url: '/data/seasons/index.json', swrKey: 'season-index' },
]);

// Parameterized routes
registerRouteData('/watch/replay/:id', () => []);
registerRouteData('/watch/series/:id', (p) => [
  { url: `/data/series/${p.id}.json`, swrKey: `series-${p.id}` },
]);
registerRouteData('/watch/playlists/:slug', (p) => [
  { url: `/data/playlists/${p.slug}.json`, swrKey: `playlist-${p.slug}` },
]);
registerRouteData('/blog/:slug', (p) => [
  { url: `/data/blog/posts/${p.slug}.json`, swrKey: `blog-${p.slug}` },
]);
registerRouteData('/bot/:id', (p) => [
  { url: `/data/bots/${p.id}.json`, swrKey: `bot-${p.id}` },
]);
registerRouteData('/compete/bot/:id', (p) => [
  { url: `/data/bots/${p.id}.json`, swrKey: `bot-${p.id}` },
]);
registerRouteData('/season/:id', (p) => [
  { url: `/data/seasons/${p.id}.json`, swrKey: `season-${p.id}` },
]);

function resolveDataMappings(path: string): DataMapping[] {
  for (const entry of ROUTE_DATA) {
    const match = path.match(entry.pattern);
    if (match) {
      const params: Record<string, string> = {};
      entry.paramNames.forEach((name, idx) => {
        params[name] = decodeURIComponent(match[idx + 1]);
      });
      return entry.mappings(params);
    }
  }
  return [];
}

// ─── Preload-on-hover ──────────────────────────────────────────────────────────
// Tracks which URLs have been prefetched to avoid duplicate requests.

const prefetched = new Set<string>();
const PRELOAD_DELAY = 150; // ms — debounce per §16.14 (120–200ms range)

function prefetchMapping(mapping: DataMapping): void {
  if (prefetched.has(mapping.url)) return;
  prefetched.add(mapping.url);

  // 1. <link rel=prefetch> for browser HTTP cache (low priority)
  const link = document.createElement('link');
  link.rel = 'prefetch';
  link.href = mapping.url;
  document.head.appendChild(link);

  // 2. Manual fetch into SWR application cache (medium priority)
  fetch(mapping.url)
    .then(r => r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`)))
    .then(data => seedSwrCache(mapping.swrKey, data))
    .catch(() => { /* prefetch failures are non-critical */ });
}

function prefetchRoute(path: string): void {
  const mappings = resolveDataMappings(path);
  for (const m of mappings) {
    prefetchMapping(m);
  }
}

// Attach hover/touch listeners to all internal hash links
function setupLinkListeners(): void {
  let timer: ReturnType<typeof setTimeout> | undefined;

  document.addEventListener('mouseover', (e) => {
    const anchor = (e.target as HTMLElement).closest('a[href^="#/"]') as HTMLAnchorElement | null;
    if (!anchor) return;
    const path = anchor.getAttribute('href')!.slice(2); // strip "#/"
    if (!path) return;

    clearTimeout(timer);
    timer = setTimeout(() => prefetchRoute(path), PRELOAD_DELAY);
  }, { passive: true });

  document.addEventListener('touchstart', (e) => {
    const anchor = (e.target as HTMLElement).closest('a[href^="#/"]') as HTMLAnchorElement | null;
    if (!anchor) return;
    const path = anchor.getAttribute('href')!.slice(2);
    if (!path) return;
    // On touch, prefetch immediately (no debounce — tap is intentional)
    prefetchRoute(path);
  }, { passive: true });
}

// ─── Instant back-cache ────────────────────────────────────────────────────────
// Caches the last N rendered pages with their scroll position so back/forward
// navigation restores instantly without refetching.

interface CachedPage {
  html: string;
  scrollY: number;
}

const MAX_CACHE_SIZE = 8;
const pageCache = new Map<string, CachedPage>();

export function savePageCache(path: string): void {
  const app = document.getElementById('app');
  if (!app) return;

  pageCache.set(path, {
    html: app.innerHTML,
    scrollY: window.scrollY,
  });

  // Evict oldest entries beyond cap
  if (pageCache.size > MAX_CACHE_SIZE) {
    const firstKey = pageCache.keys().next().value;
    if (firstKey !== undefined) pageCache.delete(firstKey);
  }
}

export function hasPageCache(path: string): boolean {
  return pageCache.has(path);
}

export function restorePageFromCache(path: string): boolean {
  const cached = pageCache.get(path);
  if (!cached) return false;

  const app = document.getElementById('app');
  if (!app) return false;

  app.innerHTML = cached.html;
  window.scrollTo(0, cached.scrollY);

  // Remove from cache so a forward-navigate re-renders fresh
  pageCache.delete(path);
  return true;
}

// ─── Skeleton → content fade-in ────────────────────────────────────────────────
// When skeleton is replaced by real content, apply a fade-in transition.

export function fadeInContent(container: HTMLElement): void {
  container.style.opacity = '0';
  // Force reflow so the browser registers the initial state
  container.offsetHeight; // eslint-disable-line no-unused-expressions
  container.style.transition = 'opacity 150ms ease';
  container.style.opacity = '1';
}

// ─── Initialization ────────────────────────────────────────────────────────────

export function initPerformanceFeatures(): void {
  setupLinkListeners();
}
