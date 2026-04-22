// §16.14 Performance trifecta: preload-on-hover + instant back-cache

// ─── Route → data URL mapping ──────────────────────────────────────────────────
// Maps SPA routes to the JSON data files they fetch so we can prefetch on hover.

type DataUrlFactory = (params: Record<string, string>) => string[];

const ROUTE_DATA: Array<{ pattern: RegExp; paramNames: string[]; urls: DataUrlFactory }> = [];

function registerRouteData(pattern: string, urls: DataUrlFactory): void {
  const paramNames: string[] = [];
  const regexPattern = pattern.replace(/:(\w+)/g, (_, name) => {
    paramNames.push(name);
    return '([^/]+)';
  });
  ROUTE_DATA.push({ pattern: new RegExp(`^${regexPattern}$`), paramNames, urls });
}

// Static routes — single data file
registerRouteData('/', () => ['/data/leaderboard.json', '/data/playlists/index.json', '/data/evolution/meta.json']);
registerRouteData('/leaderboard', () => ['/data/leaderboard.json']);
registerRouteData('/watch', () => ['/data/playlists/index.json', '/data/matches/index.json']);
registerRouteData('/watch/replays', () => ['/data/matches/index.json']);
registerRouteData('/watch/playlists', () => ['/data/playlists/index.json']);
registerRouteData('/watch/predictions', () => ['/data/predictions/leaderboard.json']);
registerRouteData('/evolution', () => ['/data/evolution/meta.json', '/data/evolution/lineage.json']);
registerRouteData('/blog', () => ['/data/blog/index.json']);
registerRouteData('/seasons', () => ['/data/seasons/index.json']);
registerRouteData('/compete', () => []);
registerRouteData('/compete/register', () => []);
registerRouteData('/compete/docs', () => []);

// Parameterized routes
registerRouteData('/watch/replay/:id', () => []);
registerRouteData('/watch/series/:id', (p) => [`/data/series/${p.id}.json`]);
registerRouteData('/watch/playlists/:slug', (p) => [`/data/playlists/${p.slug}.json`]);
registerRouteData('/blog/:slug', (p) => [`/data/blog/${p.slug}.json`]);
registerRouteData('/bot/:id', (p) => [`/data/bots/${p.id}.json`]);
registerRouteData('/compete/bot/:id', (p) => [`/data/bots/${p.id}.json`]);
registerRouteData('/season/:id', (p) => [`/data/seasons/${p.id}.json`]);

function resolveDataUrls(path: string): string[] {
  for (const entry of ROUTE_DATA) {
    const match = path.match(entry.pattern);
    if (match) {
      const params: Record<string, string> = {};
      entry.paramNames.forEach((name, idx) => {
        params[name] = decodeURIComponent(match[idx + 1]);
      });
      return entry.urls(params);
    }
  }
  return [];
}

// ─── Preload-on-hover ──────────────────────────────────────────────────────────
// Tracks which URLs have been prefetched to avoid duplicate requests.

const prefetched = new Set<string>();
const PRELOAD_DELAY = 150; // ms — debounce per §16.14 (120–200ms range)

function prefetchUrl(url: string): void {
  if (prefetched.has(url)) return;
  prefetched.add(url);
  // Use <link rel=prefetch> for low-priority background fetch
  const link = document.createElement('link');
  link.rel = 'prefetch';
  link.href = url;
  document.head.appendChild(link);
}

function prefetchRoute(path: string): void {
  const urls = resolveDataUrls(path);
  for (const url of urls) {
    prefetchUrl(url);
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
  data: unknown;
}

const MAX_CACHE_SIZE = 8;
const pageCache = new Map<string, CachedPage>();

export function savePageCache(path: string): void {
  const app = document.getElementById('app');
  if (!app) return;

  pageCache.set(path, {
    html: app.innerHTML,
    scrollY: window.scrollY,
    data: null,
  });

  // Evict oldest entries beyond cap
  if (pageCache.size > MAX_CACHE_SIZE) {
    const firstKey = pageCache.keys().next().value;
    if (firstKey !== undefined) pageCache.delete(firstKey);
  }
}

export function getPageCache(path: string): CachedPage | undefined {
  return pageCache.get(path);
}

export function hasPageCache(path: string): boolean {
  return pageCache.has(path);
}

export function clearPageCache(path: string): void {
  pageCache.delete(path);
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

// ─── Initialization ────────────────────────────────────────────────────────────

export function initPerformanceFeatures(): void {
  setupLinkListeners();
}
