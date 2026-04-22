// Ambient activity awareness — §16.18
// Favicon badges, tab title updates, haptic feedback.
// Non-intrusive signals that keep users aware of platform activity.

import type { SeasonIndex, LiveJSON } from '../types';
import { applySeasonTheme } from './season-theme';

// ─── State ──────────────────────────────────────────────────────────────────────

let unreadCount = 0;
let pollTimer: ReturnType<typeof setInterval> | null = null;
const BASE_TITLE = 'AI Code Battle';
const STORAGE_KEY = 'acb_ambient_haptic';

// ─── Favicon badge ──────────────────────────────────────────────────────────────
// Draws a small colored dot on a 32×32 canvas favicon and swaps the <link>.

function drawFavicon(color: string | null, count?: number): void {
  const size = 32;
  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d');
  if (!ctx) return;

  // Base: dark circle with sword glyph
  ctx.fillStyle = '#0f172a';
  ctx.beginPath();
  ctx.arc(size / 2, size / 2, size / 2, 0, Math.PI * 2);
  ctx.fill();

  // Sword character
  ctx.fillStyle = '#f8fafc';
  ctx.font = 'bold 18px sans-serif';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.fillText('⚔', size / 2, size / 2);

  // Badge dot + count
  if (color && count && count > 0) {
    const badgeR = count > 9 ? 9 : 7;
    const bx = size - badgeR - 1;
    const by = badgeR + 1;

    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.arc(bx, by, badgeR, 0, Math.PI * 2);
    ctx.fill();

    if (count > 0) {
      ctx.fillStyle = '#fff';
      ctx.font = `bold ${count > 9 ? 9 : 10}px sans-serif`;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(String(Math.min(count, 99)), bx, by + 1);
    }
  }

  // Swap favicon
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
  if (!link) {
    link = document.createElement('link');
    link.rel = 'icon';
    link.type = 'image/png';
    document.head.appendChild(link);
  }
  link.href = canvas.toDataURL('image/png');
}

/** Update favicon to show a numeric badge with the given color. */
export function setFaviconBadge(count: number, color: string): void {
  unreadCount = count;
  drawFavicon(count > 0 ? color : null, count);
}

/** Clear the favicon badge back to default. */
export function clearFaviconBadge(): void {
  unreadCount = 0;
  drawFavicon(null);
}

// ─── Tab title ──────────────────────────────────────────────────────────────────

/** Update the document title with an unread counter. */
export function updateTabTitle(suffix?: string): void {
  if (unreadCount > 0) {
    document.title = suffix
      ? `${suffix} — ${BASE_TITLE}`
      : `(${unreadCount}) ${BASE_TITLE}`;
  } else {
    document.title = suffix ? `${suffix} — ${BASE_TITLE}` : BASE_TITLE;
  }
}

/** Reset title to default (called on visibility change). */
function resetTabTitle(): void {
  document.title = BASE_TITLE;
  clearFaviconBadge();
}

// ─── Haptic feedback ────────────────────────────────────────────────────────────

function hapticEnabled(): boolean {
  return localStorage.getItem(STORAGE_KEY) !== 'false';
}

/**
 * Trigger a light haptic pulse on supported mobile devices.
 * §16.18: 50ms vibration on key events (prediction resolved, followed bot wins).
 */
export function hapticPulse(duration = 50): void {
  if (!hapticEnabled()) return;
  if ('vibrate' in navigator) {
    navigator.vibrate(duration);
  }
}

/** Toggle haptic preference. */
export function setHapticEnabled(enabled: boolean): void {
  localStorage.setItem(STORAGE_KEY, String(enabled));
}

/** Check if haptics are currently enabled. */
export function isHapticEnabled(): boolean {
  return hapticEnabled();
}

// ─── Notification events ────────────────────────────────────────────────────────

export type AmbientEventType =
  | 'match_result'
  | 'prediction_resolved'
  | 'rivalry_update'
  | 'season_event';

const EVENT_COLORS: Record<AmbientEventType, string> = {
  match_result: '#ef4444',
  prediction_resolved: '#f59e0b',
  rivalry_update: '#3b82f6',
  season_event: '#22c55e',
};

const EVENT_TITLES: Record<AmbientEventType, string> = {
  match_result: 'Match result',
  prediction_resolved: 'Prediction resolved',
  rivalry_update: 'Rivalry update',
  season_event: 'Season event',
};

interface PendingEvent {
  type: AmbientEventType;
  detail?: string;
}

let pendingEvents: PendingEvent[] = [];

/**
 * Push an ambient event. Updates favicon badge + tab title if the tab is hidden.
 * Also triggers haptic pulse on mobile.
 */
export function pushAmbientEvent(type: AmbientEventType, detail?: string): void {
  pendingEvents.push({ type, detail });

  const isHidden = document.hidden;
  if (isHidden) {
    setFaviconBadge(pendingEvents.length, EVENT_COLORS[type]);
    updateTabTitle(detail ?? EVENT_TITLES[type]);
  } else {
    // Tab is visible — auto-clear immediately
    pendingEvents = [];
  }

  hapticPulse();
}

// ─── Lifecycle ──────────────────────────────────────────────────────────────────

/**
 * Initialize the ambient awareness system.
 * - Listens for visibility changes to reset badge/title when tab is focused.
 * - Optionally starts a data polling interval for live activity.
 */
export function initAmbient(): void {
  // Draw the default favicon on load
  drawFavicon(null);

  // Clear badge and title when the tab becomes visible
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) {
      pendingEvents = [];
      resetTabTitle();
    }
  });
}

/**
 * Start polling for activity updates.
 * Checks match index for new results and evolution live data.
 */
export function startAmbientPolling(intervalMs = 30_000): void {
  stopAmbientPolling();

  let lastMatchCount = 0;
  let lastGeneration = 0;

  async function poll(): Promise<void> {
    try {
      // Fetch match index to detect new matches
      const matchResp = await fetch('/data/matches/index.json');
      if (matchResp.ok) {
        const matchData = await matchResp.json();
        const currentCount = matchData.pagination?.total ?? matchData.matches?.length ?? 0;
        if (lastMatchCount > 0 && currentCount > lastMatchCount) {
          const newCount = currentCount - lastMatchCount;
          pushAmbientEvent('match_result', `${newCount} new match${newCount > 1 ? 'es' : ''}`);
        }
        lastMatchCount = currentCount;
      }
    } catch {
      // Silently ignore fetch failures
    }

    try {
      // Fetch evolution live data for generation changes
      const evoResp = await fetch(
        'https://r2.aicodebattle.com/evolution/live.json',
      );
      if (evoResp.ok) {
        const evoData: LiveJSON = await evoResp.json();
        const gen = evoData.totals?.generations_total ?? 0;
        if (lastGeneration > 0 && gen > lastGeneration) {
          pushAmbientEvent('rivalry_update', `Evolution gen #${gen}`);
        }
        lastGeneration = gen;
      }
    } catch {
      // Silently ignore
    }
  }

  // Initial fetch to establish baseline
  poll();
  pollTimer = setInterval(poll, intervalMs);
}

/** Stop the ambient polling interval. */
export function stopAmbientPolling(): void {
  if (pollTimer !== null) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

// ─── Seasonal theme integration ─────────────────────────────────────────────────

/**
 * Fetch season index and apply the seasonal color theme.
 * Called once on page load.
 */
export async function applyCurrentSeasonTheme(): Promise<void> {
  try {
    const resp = await fetch('/data/seasons/index.json');
    if (!resp.ok) return;
    const data: SeasonIndex = await resp.json();
    const active = data.active_season;
    applySeasonTheme(active?.theme ?? null);
  } catch {
    // No season data — keep default theme
  }
}
