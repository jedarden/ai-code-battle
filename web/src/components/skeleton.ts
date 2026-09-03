// §16.14 Skeleton screens — per-page placeholder layouts matching final content.
// Base Skeleton component with shimmer animation and reusable variants.
// The shimmer CSS animation is in components.css with 1.5s sweep interval.

// ─── Base Skeleton Component ─────────────────────────────────────────────────────

/**
 * Skeleton component variants
 */
export type SkeletonVariant = 'bar' | 'circle' | 'rectangle';

/**
 * Props for the Skeleton component
 */
export interface SkeletonProps {
  variant: SkeletonVariant;
  width?: string;
  height?: string;
  extra?: string;
}

/**
 * Base reusable Skeleton component with shimmer animation
 * Returns HTML string with proper styling and animation classes
 *
 * @param props - Skeleton configuration
 * @returns HTML string for skeleton element
 */
export function Skeleton(props: SkeletonProps): string {
  const { variant, width = '100%', height = '16px', extra = '' } = props;

  // Dimensions first, then the variant default, then extra — so an extra can
  // override the default (last declaration wins) and every declaration stays
  // semicolon-terminated when the pieces are concatenated.
  const dims = `width:${width};height:${height};`;
  const tail = extra && !extra.endsWith(';') ? `${extra};` : extra;

  switch (variant) {
    case 'circle':
      return `<div class="skeleton-circle" style="${dims}${tail}"></div>`;

    case 'rectangle':
      return `<div class="skeleton-bar" style="${dims}border-radius:var(--radius-md);${tail}"></div>`;

    case 'bar':
    default:
      return `<div class="skeleton-bar" style="${dims}${tail}"></div>`;
  }
}

// ─── Convenience Functions (backward compatible) ───────────────────────────────────

const shimmer = 'skeleton-bar';

/**
 * @deprecated Use Skeleton({ variant: 'bar', width, height, extra }) instead
 */
function bar(w: string, h: string = '16px', extra = ''): string {
  return `<div class="${shimmer}" style="width:${w};height:${h};${extra}"></div>`;
}

/**
 * Canvas skeleton component matching the replay canvas dimensions
 * Matches: .canvas-wrapper styling (var(--bg-secondary), border-radius: 8px, padding: 10px)
 * Canvas: 100% width, 400px height (matches replay skeleton placeholder)
 *
 * @returns HTML string for canvas skeleton element
 */
export function CanvasSkeleton(): string {
  return Skeleton({
    variant: 'rectangle',
    width: '100%',
    height: '400px',
    extra: 'background:var(--bg-tertiary)'
  });
}

/**
 * Scrubber skeleton component matching the replay scrubber dimensions
 * Matches: input[type="range"] styling (width: 100%)
 * Height: 4px (thin bar for timeline scrubber)
 *
 * @returns HTML string for scrubber skeleton element
 */
export function ScrubberSkeleton(): string {
  return Skeleton({
    variant: 'bar',
    width: '100%',
    height: '4px',
    extra: 'border-radius:2px;background:var(--bg-tertiary)'
  });
}

// ─── Per-page skeletons ────────────────────────────────────────────────────────

export function skeletonLeaderboard(): string {
  // Desktop rows reuse the live .lb-row rule (styles/components.css): geometry
  // (gap, padding, border-bottom, min-height, box-sizing) comes from that rule,
  // and the --lb-col-* custom properties declared on it stay in scope for the
  // bar widths below, so skeleton and live rows cannot drift. Columns mirror
  // renderDesktopRow (web/src/pages/leaderboard.ts):
  // rank + name(flex:1) + rating + wl + winrate + status + expand.
  // Rows live in #lb-desktop so the live responsive rules govern visibility.
  const rows = Array.from({ length: 15 }, () => {
    return `<div class="lb-row">
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-rank)', extra: 'flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-name-min)', extra: 'flex:1;min-width:var(--lb-col-name-min)' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-rating)', extra: 'flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-wl)', extra: 'flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-winrate)', extra: 'flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-status)', extra: 'flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-expand)', extra: 'flex-shrink:0' })}
    </div>`;
  }).join('');
  // Mobile cards mirror renderMobileCard (web/src/pages/leaderboard.ts): the
  // placeholders reuse the live .leaderboard-mobile-card / .mobile-card-toggle /
  // .leaderboard-mobile-* classes (styles/mobile.css phone block + components.css)
  // so gap, padding, background, radius, margin-bottom and the collapsed-details
  // geometry come from the same rules the real cards use and cannot drift. Only
  // the placeholder bars carry inline dimensions. Toggle and arrow are divs, not
  // button/span — a skeleton has nothing to activate — and the cards carry no
  // role or data attributes, matching the decorative rows above.
  const card = `
    <div class="leaderboard-mobile-card">
      <div class="mobile-card-toggle">
        <div class="leaderboard-mobile-rank">${Skeleton({ variant: 'bar', width: '24px', height: '20px', extra: 'margin:0 auto' })}</div>
        <div class="leaderboard-mobile-info">
          ${Skeleton({ variant: 'bar', width: '70%', height: '16px', extra: 'margin-bottom:8px' })}
          <div class="leaderboard-mobile-rating">${Skeleton({ variant: 'bar', width: '48px', height: '16px', extra: 'display:inline-block;vertical-align:middle' })}${Skeleton({ variant: 'bar', width: '28px', height: '12px', extra: 'display:inline-block;vertical-align:middle;margin-left:4px' })}</div>
        </div>
        <div class="mobile-card-arrow">${Skeleton({ variant: 'bar', width: '8px', height: '12px' })}</div>
      </div>
      <div class="leaderboard-mobile-details">
        ${Array.from({ length: 4 }, () => `
        <div class="leaderboard-mobile-stat">
          ${Skeleton({ variant: 'bar', width: '44px', height: '12px' })}
          ${Skeleton({ variant: 'bar', width: '56px', height: '12px' })}
        </div>`).join('')}
        ${Skeleton({ variant: 'rectangle', width: '100%', height: '40px', extra: 'margin-top:10px' })}
      </div>
    </div>`;
  const cards = Array.from({ length: 8 }, () => card).join('');
  return `
    <div class="skeleton-page">
      <h1 class="page-title">Leaderboard</h1>
      ${Skeleton({ variant: 'bar', width: '200px', height: '14px', extra: 'margin-bottom:24px' })}
      <div id="lb-desktop">${rows}</div>
      <div id="lb-mobile" class="mobile-cards">${cards}</div>
    </div>`;
}

export function skeletonBotProfile(): string {
  return `
    <div class="skeleton-page">
      <!-- Breadcrumb -->
      <div class="skeleton-row" style="margin-bottom:16px">
        ${Skeleton({ variant: 'bar', width: '80px', height: '14px' })}
        ${Skeleton({ variant: 'bar', width: '12px', height: '14px', extra: 'margin:0 8px' })}
        ${Skeleton({ variant: 'bar', width: '120px', height: '14px' })}
      </div>

      <!-- Profile Header -->
      <div class="skeleton-row" style="justify-content:space-between;align-items:flex-start;margin-bottom:24px">
        <div style="flex:1">
          ${Skeleton({ variant: 'bar', width: '200px', height: '32px', extra: 'margin-bottom:12px' })}
          ${Skeleton({ variant: 'bar', width: '100px', height: '20px' })}
        </div>
        ${Skeleton({ variant: 'rectangle', width: '140px', height: '36px' })}
      </div>

      <!-- Rating Section -->
      <div style="margin-bottom:24px">
        ${Skeleton({ variant: 'bar', width: '80px', height: '24px', extra: 'margin-bottom:16px' })}
        <div class="skeleton-row" style="gap:16px;align-items:flex-end;margin-bottom:16px">
          ${Skeleton({ variant: 'bar', width: '80px', height: '48px' })}
          ${Skeleton({ variant: 'bar', width: '60px', height: '24px' })}
        </div>
        ${Skeleton({ variant: 'rectangle', width: '100%', height: '100px', extra: 'border-radius:8px' })}
      </div>

      <!-- Stats Section -->
      <div style="margin-bottom:24px">
        ${Skeleton({ variant: 'bar', width: '100px', height: '24px', extra: 'margin-bottom:16px' })}
        <div style="display:grid;grid-template-columns:repeat(4,1fr);gap:16px">
          ${Skeleton({ variant: 'rectangle', width: '100%', height: '80px', extra: 'border-radius:8px' })}
          ${Skeleton({ variant: 'rectangle', width: '100%', height: '80px', extra: 'border-radius:8px' })}
          ${Skeleton({ variant: 'rectangle', width: '100%', height: '80px', extra: 'border-radius:8px' })}
          ${Skeleton({ variant: 'rectangle', width: '100%', height: '80px', extra: 'border-radius:8px' })}
        </div>
      </div>

      <!-- Info Section (collapsed) -->
      <div style="margin-bottom:24px">
        ${Skeleton({ variant: 'bar', width: '60px', height: '24px', extra: 'margin-bottom:12px' })}
        ${Skeleton({ variant: 'bar', width: '100%', height: '16px' })}
      </div>

      <!-- Rivals Section (collapsed) -->
      <div style="margin-bottom:24px">
        ${Skeleton({ variant: 'bar', width: '70px', height: '24px', extra: 'margin-bottom:12px' })}
        ${Skeleton({ variant: 'bar', width: '100%', height: '16px' })}
      </div>

      <!-- Recent Matches Section (lazy-loaded placeholder) -->
      <div style="min-height:80px">
        ${Skeleton({ variant: 'bar', width: '120px', height: '24px', extra: 'margin-bottom:12px' })}
        <div class="skeleton-row" style="margin-bottom:8px">
          ${Skeleton({ variant: 'bar', width: '100%', height: '48px' })}
        </div>
      </div>
    </div>`;
}

export function skeletonReplay(): string {
  return `
    <div class="skeleton-page">
      <h1 class="page-title">Replay Viewer</h1>
      <div class="replay-layout" style="display:flex;gap:20px">
        <div class="replay-main" style="flex:1;min-width:0">
          <div class="canvas-wrapper" style="background-color:var(--bg-secondary);border-radius:8px;padding:10px;overflow:auto;max-height:80vh;position:relative">
            ${CanvasSkeleton()}
          </div>
          <div class="mobile-replay-controls" style="margin-top:12px">
            <div class="mobile-playback-bar" style="display:flex;gap:8px;margin-bottom:8px">
              ${Skeleton({ variant: 'bar', width: '40px', height: '32px', extra: 'border-radius:4px' })}
              ${Skeleton({ variant: 'bar', width: '40px', height: '32px', extra: 'border-radius:4px' })}
              ${Skeleton({ variant: 'bar', width: '40px', height: '32px', extra: 'border-radius:4px;background:var(--accent)' })}
              ${Skeleton({ variant: 'bar', width: '40px', height: '32px', extra: 'border-radius:4px' })}
              ${Skeleton({ variant: 'bar', width: '40px', height: '32px', extra: 'border-radius:4px' })}
              ${Skeleton({ variant: 'bar', width: '60px', height: '32px', extra: 'border-radius:4px' })}
              ${Skeleton({ variant: 'bar', width: '70px', height: '32px', extra: 'border-radius:4px' })}
            </div>
            ${ScrubberSkeleton()}
          </div>
          <div class="mobile-event-timeline" style="margin-top:12px;padding:8px;background:var(--bg-secondary);border-radius:6px;min-height:32px">
            ${Skeleton({ variant: 'bar', width: '40%', height: '16px' })}
          </div>
        </div>
        <div class="replay-sidebar" style="width:300px;flex-shrink:0;display:flex;flex-direction:column;gap:15px">
          ${Skeleton({ variant: 'rectangle', width: '100%', height: '150px', extra: 'background:var(--bg-secondary)' })}
          ${Skeleton({ variant: 'rectangle', width: '100%', height: '120px', extra: 'background:var(--bg-secondary)' })}
          ${Skeleton({ variant: 'rectangle', width: '100%', height: '100px', extra: 'background:var(--bg-secondary)' })}
        </div>
      </div>
    </div>`;
}

export function skeletonPlaylists(): string {
  const cards = Array.from({ length: 6 }, () =>
    `<div class="skeleton-card">
      ${bar('100%', '140px', 'border-radius:6px 6px 0 0')}
      <div style="padding:12px">
        ${bar('70%')}
        ${bar('100%', '12px', 'margin-top:8px')}
      </div>
    </div>`
  ).join('');
  return `
    <div class="skeleton-page">
      <h1 class="page-title">Replay Playlists</h1>
      ${bar('300px', '14px', 'margin-bottom:24px')}
      <div class="skeleton-grid">${cards}</div>
    </div>`;
}

export function skeletonMatches(): string {
  const rows = Array.from({ length: 8 }, () =>
    `<div class="skeleton-row" style="padding:12px 0;border-bottom:1px solid var(--border)">
      ${bar('200px')} ${bar('60px', '16px', 'margin-left:auto')} ${bar('100px')}
    </div>`
  ).join('');
  return `
    <div class="skeleton-page">
      <h1 class="page-title">Matches</h1>
      ${rows}
    </div>`;
}

export function skeletonEvolution(): string {
  return `
    <div class="skeleton-page">
      <h1 class="page-title">Evolution Observatory</h1>
      <div class="skeleton-row" style="gap:12px;margin-bottom:24px">
        ${Array.from({ length: 4 }, () =>
          `<div class="skeleton-card" style="flex:1;text-align:center;padding:16px">
            ${bar('60px', '14px', 'margin:0 auto 8px')}
            ${bar('40px', '24px', 'margin:0 auto')}
          </div>`
        ).join('')}
      </div>
      ${bar('100%', '300px', 'border-radius:8px')}
      ${bar('100%', '120px', 'border-radius:8px;margin-top:16px')}
    </div>`;
}

export function skeletonBlog(): string {
  const posts = Array.from({ length: 4 }, () =>
    `<div class="skeleton-card">
      ${bar('70%', '20px')}
      ${bar('100%', '12px', 'margin-top:8px')}
      ${bar('100%', '12px', 'margin-top:4px')}
      ${bar('50%', '12px', 'margin-top:4px')}
      ${bar('60px', '14px', 'margin-top:8px')}
    </div>`
  ).join('');
  return `
    <div class="skeleton-page">
      <h1 class="page-title">Blog</h1>
      <div style="display:flex;flex-direction:column;gap:16px">${posts}</div>
    </div>`;
}

export function skeletonSeasons(): string {
  const cards = Array.from({ length: 3 }, () =>
    `<div class="skeleton-card">
      ${bar('50%', '20px')}
      ${bar('100%', '14px', 'margin-top:8px')}
      ${bar('80px', '28px', 'margin-top:12px;border-radius:6px')}
    </div>`
  ).join('');
  return `
    <div class="skeleton-page">
      <h1 class="page-title">Seasons</h1>
      <div class="skeleton-grid">${cards}</div>
    </div>`;
}

export function skeletonGeneric(title: string): string {
  return `
    <div class="skeleton-page">
      <h1 class="page-title">${title}</h1>
      <div style="display:flex;flex-direction:column;gap:12px">
        ${bar('100%')} ${bar('80%')} ${bar('100%')} ${bar('60%')} ${bar('90%')}
      </div>
    </div>`;
}
