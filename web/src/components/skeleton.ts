// §16.14 Skeleton screens — per-page placeholder layouts matching final content.
// Each skeleton function returns HTML that mirrors the real page layout.
// The shimmer CSS animation is in components.css.

const shimmer = 'skeleton-bar';

function bar(w: string, h: string = '16px', extra = ''): string {
  return `<div class="${shimmer}" style="width:${w};height:${h};${extra}"></div>`;
}

function circle(size: string): string {
  return `<div class="skeleton-circle" style="width:${size};height:${size}"></div>`;
}

// ─── Per-page skeletons ────────────────────────────────────────────────────────

export function skeletonLeaderboard(): string {
  const rows = Array.from({ length: 10 }, () => {
    return `<div class="skeleton-row">
      ${bar('40px', '16px', 'border-radius:4px')}
      ${circle('32px')}
      ${bar('120px')}
      ${bar('60px', '16px', 'margin-left:auto')}
      ${bar('50px')}
      ${bar('40px')}
    </div>`;
  }).join('');
  return `
    <div class="skeleton-page">
      <h1 class="page-title">Leaderboard</h1>
      ${bar('200px', '14px', 'margin-bottom:24px')}
      <div class="skeleton-table-header">
        ${bar('40px', '12px')} ${bar('60px', '12px')} ${bar('100px', '12px')} ${bar('60px', '12px', 'margin-left:auto')} ${bar('50px', '12px')} ${bar('40px', '12px')}
      </div>
      ${rows}
    </div>`;
}

export function skeletonBotProfile(): string {
  return `
    <div class="skeleton-page">
      ${bar('160px', '14px', 'margin-bottom:16px')}
      <div class="skeleton-profile-header">
        ${circle('64px')}
        <div class="skeleton-profile-info">
          ${bar('180px', '24px')}
          ${bar('120px', '14px', 'margin-top:8px')}
        </div>
        <div class="skeleton-profile-stats">
          ${bar('80px', '32px')} ${bar('80px', '32px')} ${bar('80px', '32px')}
        </div>
      </div>
      <div style="margin-top:24px">
        ${bar('100%', '200px', 'border-radius:8px')}
      </div>
      <div style="margin-top:24px">
        ${Array.from({ length: 5 }, () => `<div class="skeleton-row" style="margin-bottom:8px">${bar('100%')}</div>`).join('')}
      </div>
    </div>`;
}

export function skeletonReplay(): string {
  return `
    <div class="skeleton-page">
      ${bar('200px', '24px', 'margin-bottom:16px')}
      <div class="skeleton-canvas" style="width:100%;aspect-ratio:1/1;border-radius:8px"></div>
      <div style="margin-top:12px">
        ${bar('100%', '24px', 'border-radius:4px')}
      </div>
      <div class="skeleton-row" style="margin-top:16px">
        ${bar('60px', '32px')} ${bar('60px', '32px')} ${bar('80px', '32px')} ${bar('80px', '32px', 'margin-left:auto')}
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
