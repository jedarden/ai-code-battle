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

  const baseStyle = `width:${width};height:${height};${extra}`;

  switch (variant) {
    case 'circle':
      return `<div class="skeleton-circle" style="${baseStyle}"></div>`;

    case 'rectangle':
      return `<div class="skeleton-bar" style="${baseStyle}border-radius:var(--radius-md);"></div>`;

    case 'bar':
    default:
      return `<div class="skeleton-bar" style="${baseStyle}"></div>`;
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

// ─── Per-page skeletons ────────────────────────────────────────────────────────

export function skeletonLeaderboard(): string {
  // Match exact flexbox layout and column widths from live leaderboard (.lb-row)
  // Columns: rank(40px) + name(flex:1,min120px) + rating(100px) + wl(60px) + winrate(70px) + status(80px) + expand(20px)
  const rows = Array.from({ length: 15 }, () => {
    return `<div class="skeleton-row" style="display:flex;align-items:center;gap:var(--space-md);padding:var(--space-sm) var(--space-md);border-bottom:1px solid var(--border);min-height:48px;box-sizing:border-box;">
      ${Skeleton({ variant: 'bar', width: '40px', height: '16px', extra: 'border-radius:4px;flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: '140px', height: '16px', extra: 'flex:1;min-width:120px' })}
      ${Skeleton({ variant: 'bar', width: '100px', height: '16px', extra: 'flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: '60px', height: '16px', extra: 'flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: '70px', height: '16px', extra: 'flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: '80px', height: '16px', extra: 'flex-shrink:0;font-size:0.8rem' })}
      ${Skeleton({ variant: 'bar', width: '20px', height: '16px', extra: 'flex-shrink:0' })}
    </div>`;
  }).join('');
  return `
    <div class="skeleton-page">
      <h1 class="page-title">Leaderboard</h1>
      ${Skeleton({ variant: 'bar', width: '200px', height: '14px', extra: 'margin-bottom:24px' })}
      ${rows}
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
            ${Skeleton({ variant: 'rectangle', width: '100%', height: '60vh', extra: 'background:var(--bg-tertiary)' })}
          </div>
          <div style="margin-top:12px">
            ${Skeleton({ variant: 'bar', width: '100%', height: '24px', extra: 'border-radius:4px' })}
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
