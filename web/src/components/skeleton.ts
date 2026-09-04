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
  /**
   * Empty string omits the declaration entirely, for the rare bar whose
   * height a shared class owns because it moves with the breakpoint (the
   * bot-profile heading and button stand-ins; see .skeleton-profile-heading /
   * .skeleton-profile-btn in components.css).
   */
  height?: string;
  extra?: string;
  /** Extra classes after the base one, e.g. the live column class a bar stands in for. */
  className?: string;
}

/**
 * Base reusable Skeleton component with shimmer animation
 * Returns HTML string with proper styling and animation classes
 *
 * @param props - Skeleton configuration
 * @returns HTML string for skeleton element
 */
export function Skeleton(props: SkeletonProps): string {
  const { variant, width = '100%', height = '16px', extra = '', className = '' } = props;

  // Dimensions first, then the variant default, then extra — so an extra can
  // override the default (last declaration wins) and every declaration stays
  // semicolon-terminated when the pieces are concatenated.
  const dims = `width:${width};${height ? `height:${height};` : ''}`;
  const tail = extra && !extra.endsWith(';') ? `${extra};` : extra;
  const cls = className ? `skeleton-bar ${className}` : 'skeleton-bar';

  switch (variant) {
    case 'circle':
      return `<div class="skeleton-circle" style="${dims}${tail}"></div>`;

    case 'rectangle':
      return `<div class="${cls}" style="${dims}border-radius:var(--radius-md);${tail}"></div>`;

    case 'bar':
    default:
      return `<div class="${cls}" style="${dims}${tail}"></div>`;
  }
}

// ─── Shared page-section skeletons ──────────────────────────────────────────────

/**
 * Canvas skeleton component matching the replay canvas dimensions
 * The live canvas has no width/height attributes until a replay loads
 * (web/src/pages/replay.ts), so it renders at its intrinsic 300×150 ratio
 * under `.canvas-wrapper canvas { width:100%; height:auto }` — an
 * aspect-ratio bar tracks that height at every viewport, where a fixed
 * pixel height could not. No `background` extra: it would reset the shared
 * gradient and kill the shimmer.
 *
 * @returns HTML string for canvas skeleton element
 */
export function CanvasSkeleton(): string {
  return Skeleton({
    variant: 'rectangle',
    width: '100%',
    height: '',
    extra: 'aspect-ratio:2/1'
  });
}

/**
 * Scrubber skeleton component matching the replay scrubber dimensions
 * Matches: the mobile turn slider, a bare `input[type="range"]` — an `input`,
 * so base.css's tap-target group (`button, a, input, select, textarea`)
 * floors its box at 44px — plus the live inline margin-top:4px.
 *
 * @returns HTML string for scrubber skeleton element
 */
export function ScrubberSkeleton(): string {
  return Skeleton({
    variant: 'bar',
    width: '100%',
    height: '44px',
    extra: 'margin-top:4px'
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
  // Rows live in #lb-desktop so the live responsive rules govern visibility,
  // and the wl/status bars carry the live .lb-wl/.lb-status classes so the
  // @media (max-width: 768px) rule that collapses those columns on the real
  // row collapses them here too — no skeleton-only breakpoint or display rule.
  const rows = Array.from({ length: 15 }, () => {
    return `<div class="lb-row">
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-rank)', extra: 'flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-name-min)', extra: 'flex:1;min-width:var(--lb-col-name-min)' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-rating)', extra: 'flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-wl)', extra: 'flex-shrink:0', className: 'lb-wl' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-winrate)', extra: 'flex-shrink:0' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-status)', extra: 'flex-shrink:0', className: 'lb-status' })}
      ${Skeleton({ variant: 'bar', width: 'var(--lb-col-expand)', extra: 'flex-shrink:0' })}
    </div>`;
  }).join('');
  // Mobile cards mirror renderMobileCard (web/src/pages/leaderboard.ts): the
  // placeholders reuse the live .leaderboard-mobile-card / .mobile-card-toggle /
  // .leaderboard-mobile-* classes (styles/mobile.css phone block + components.css)
  // so gap, padding, background, radius, margin-bottom and the collapsed-details
  // geometry come from the same rules the real cards use and cannot drift, and
  // the container is the live #lb-mobile.mobile-cards with the live role="list"
  // (renderLeaderboard's markup), whose display comes from the
  // phone/tablet/desktop media blocks in styles/mobile.css. Only the placeholder
  // bars carry inline dimensions. Toggle and arrow are divs, not button/span —
  // a skeleton has nothing to activate — so the toggle carries inline the one
  // quantity it cannot reach through a class: base.css's tap-target rule gives
  // every real button min-height:44px, and that floor is what actually sets the
  // live toggle's height (its 13.33px UA text sits well under it), so the
  // stand-in floor is what makes the swap height-neutral. The name bar carries
  // no margin for the same reason — the live .leaderboard-mobile-name has none
  // — and stays below the floor with the rating line, leaving the toggle's
  // height to the floor on both sides. The cards carry no role or data
  // attributes, so unlike the live listitems they announce nothing.
  const card = `
    <div class="leaderboard-mobile-card">
      <div class="mobile-card-toggle" style="min-height:44px">
        <div class="leaderboard-mobile-rank">${Skeleton({ variant: 'bar', width: '24px', height: '20px', extra: 'margin:0 auto' })}</div>
        <div class="leaderboard-mobile-info">
          ${Skeleton({ variant: 'bar', width: '70%', height: '16px' })}
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
  // Header stand-ins mirror renderLeaderboardPage + renderLeaderboard
  // (web/src/pages/leaderboard.ts) line for line, so the data-load swap keeps
  // every row and card at the y its placeholder occupied (§16.14 #2 zero
  // layout shift). The h1 reuses the live .page-title rule, whose
  // margin-bottom (var(--space-lg)) is what the bare h1 rule's smaller
  // --space-md would otherwise change on the swap; the updated-at and hint
  // bars are sized to the text metrics they stand in for — font-size ×
  // line-height (1.5 for text, base.css) — and the wrappers carry no inline
  // margins: the live .updated-at / .lb-hint rules own those, so the spacing
  // above the first row cannot drift. The wrappers are divs, not p — the HTML
  // parser closes a p around a block child, which would shred the wrapper —
  // and p vs div lays out identically here because both classes declare their
  // own margin-bottom and color (base.css's bare p rule never applies).
  return `
    <div class="skeleton-page">
      <h1 class="page-title">Leaderboard</h1>
      <div class="updated-at">${Skeleton({ variant: 'bar', width: '200px', height: '18px' })}</div>
      <div class="lb-hint">${Skeleton({ variant: 'bar', width: '180px', height: '19.2px' })}</div>
      <div id="lb-desktop">${rows}</div>
      <div id="lb-mobile" class="mobile-cards" role="list">${cards}</div>
    </div>`;
}

export function skeletonBotProfile(): string {
  // Mirrors renderBotProfilePage's swap (web/src/pages/bot-profile.ts) node
  // for node: the live .bot-profile-page (max-width 900px + padding) opens
  // with the nav.breadcrumb — the "Leaderboard / <name>" row the swap renders
  // above the content, whose <a> picks up the base 44px tap-target floor, so
  // the stand-in bar is 44px tall and the row's margin comes from the live
  // .breadcrumb rule the nav itself carries — then .profile-header (the
  // .profile-header-main column of name h1 + .profile-status chip, plus the
  // share-card button; the live header renders no avatar and none is invented
  // here), then .profile-grid with the ratings, stats, meta, rivals and lazy
  // history sections in the live order. The skeleton keeps its .skeleton-page
  // root like every page skeleton; that wrapper is max-width 1200px + auto
  // margins with no padding, so the 900px column it wraps sits exactly where
  // the live page puts it. Every wrapper reuses the live class, so header flex
  // layout, grid columns (auto-fit → 1 column on phone, 2 on tablet), section
  // chrome, toggle rows and the collapsed .section-content boxes all come from
  // the same rules the real page uses and cannot drift. Only the bars carry
  // inline dimensions, derived from the text metrics they stand in for — the
  // stand-in's font-size × line-height (1.5 for text, 1.25 for headings, both
  // declared in base.css), plus its own vertical padding where the live
  // element has any:
  //   breadcrumb bar     44px tap-target floor (base.css, button/a rule)
  //   name h1 bar        2rem × 1.25 (.profile-header h1)         = 40px
  //   status chip bar    0.75rem × 1.5 + 2 × var(--space-xs)      = 26px
  //   share-card button  tap-target floor 44px (.btn content 37px sits below it)
  //   ratings h2 bar     1.5rem × 1.25 (base h2 rule)             = 30px
  //   rating-main bar    2.5rem × 1.5                             = 60px
  //   rating-dev bar     1rem × 1.5                               = 24px
  //   range bars         0.75rem × 1.5 (.rating-range)            = 18px
  //   toggle h2 bar      1rem × 1.25 (.section-toggle h2)         = 20px
  //   toggle icon bar    0.75rem × 1.5 (.section-toggle-icon)     = 18px
  //   stat value/label   1.5rem / 0.75rem × 1.5                   = 36px / 18px
  // Two of those heights move with the breakpoint — the ratings h2 (mobile.css
  // drops the bare h2 to 1.25rem on phone → 25px) and the share-card button
  // (mobile.css raises the .btn floor to 48px) — so those two bars carry no
  // inline height at all: .skeleton-profile-heading and .skeleton-profile-btn
  // own it in components.css, and mobile.css's phone block re-declares it from
  // the phone rules the live elements answer to. The margins and the chip's
  // radius come from the shared .skeleton-profile-* rules (components.css,
  // next to .skeleton-page/.skeleton-row), not inline styles: the live rules
  // declare them through selectors a div stand-in cannot carry (.profile-header
  // h1's margin-bottom, the bare h2's from the base heading rule,
  // .profile-status' border-radius), and each skeleton class re-declares that
  // one quantity from the same custom property the live rule uses, so skeleton
  // and live page cannot drift. No bar here carries an inline margin — the
  // parity guards in skeleton.test.ts hold every bar to that.
  // Shimmer comes from the shared .skeleton-bar rule (components.css) on every
  // placeholder, and from the shared .lazy-placeholder rule — the same one
  // lazySection renders below the fold — on the history block, so this file
  // declares no animation CSS of its own. Toggle rows and the history
  // placeholder are divs, not button/observer — a skeleton has nothing to
  // activate — so they carry no aria, id or data attributes.
  const toggleRow = (labelWidth: string) => `
    <div class="section-toggle">
      ${Skeleton({ variant: 'bar', width: labelWidth, height: '20px' })}
      ${Skeleton({ variant: 'bar', width: '10px', height: '18px' })}
    </div>`;
  return `
    <div class="skeleton-page">
      <div class="bot-profile-page">
        <nav class="breadcrumb">
          ${Skeleton({ variant: 'bar', width: '190px', height: '44px' })}
        </nav>

        <div class="profile-header">
          <div class="profile-header-main">
            ${Skeleton({ variant: 'bar', width: '220px', height: '40px', className: 'skeleton-profile-name' })}
            ${Skeleton({ variant: 'bar', width: '80px', height: '26px', className: 'skeleton-profile-chip' })}
          </div>
          ${Skeleton({ variant: 'rectangle', width: '128px', height: '', className: 'skeleton-profile-btn' })}
        </div>

        <div class="profile-grid">
          <div class="profile-section ratings">
            ${Skeleton({ variant: 'bar', width: '80px', height: '', className: 'skeleton-profile-heading' })}
            <div class="rating-display">
              ${Skeleton({ variant: 'bar', width: '140px', height: '60px' })}
              ${Skeleton({ variant: 'bar', width: '48px', height: '24px' })}
            </div>
            <div class="rating-chart">
              ${Skeleton({ variant: 'rectangle', width: '100%', height: '60px' })}
              <div class="rating-range">
                ${Skeleton({ variant: 'bar', width: '56px', height: '18px' })}
                ${Skeleton({ variant: 'bar', width: '56px', height: '18px' })}
              </div>
            </div>
          </div>

          <div class="profile-section stats expandable-section">
            ${toggleRow('80px')}
            <div class="section-content expanded">
              <div class="stats-grid">
                ${Array.from({ length: 4 }, () => `
                <div class="stat">
                  ${Skeleton({ variant: 'bar', width: '56px', height: '36px' })}
                  ${Skeleton({ variant: 'bar', width: '72px', height: '18px' })}
                </div>`).join('')}
              </div>
            </div>
          </div>

          <div class="profile-section meta expandable-section">
            ${toggleRow('40px')}
            <div class="section-content"></div>
          </div>

          <div class="profile-section rivals expandable-section">
            ${toggleRow('56px')}
            <div class="section-content"></div>
          </div>

          <div class="lazy-section">
            <div class="lazy-placeholder" style="min-height:80px"></div>
          </div>
        </div>
      </div>
    </div>`;
}

export function skeletonReplay(): string {
  // Mirrors the live replay page node for node — initReplayViewerWithClass's
  // markup template (web/src/pages/replay.ts): .skeleton-page wraps the live
  // .replay-page —
  // the bot-profile pattern — so every .replay-page-scoped rule reaches the
  // skeleton's copy of the same markup (mobile.css drops
  // .replay-page .page-title to 1.25rem on phone, where a bare .page-title
  // stays 1.5rem; without the nested wrapper the swap changed the title's
  // size and margin and moved the page under it), then .replay-layout with
  // .replay-main (canvas wrapper, then the two mobile chrome blocks) and
  // .replay-sidebar last. Every wrapper reuses the live class, so the flex
  // row (components.css .replay-layout/.replay-main/.replay-sidebar), the
  // phone column and the ≥640px hiding of the mobile chrome (mobile.css) and
  // the sidebar widths (280px tablet, 300px desktop, 100% phone) come from
  // the same rules the real page answers and cannot drift. Only the stand-in
  // bars carry inline dimensions, derived from the metrics they stand in
  // for:
  //   canvas bar      100% × aspect-ratio 2/1 (CanvasSkeleton) — the live
  //                   canvas is an attribute-default 300×150 element under
  //                   `.canvas-wrapper canvas { width:100%; height:auto }`
  //                   until a replay loads
  //   no-replay bar   1rem × 1.5 = 24px — the live #no-replay div is a bare
  //                   block of body text inside the same wrapper
  //   control bars    44px tap-target floor (base.css button group; the real
  //                   controls are .btn.small)
  //   turn-info bar   0.75rem × 1.5 + 2 × var(--space-xs) = 26px — the
  //                   .mobile-speed-display span's text plus its own padding
  //   scrubber bar    44px (ScrubberSkeleton) + the live input's inline
  //                   margin-top:4px
  //   timeline bar    26px — the live placeholder span's text + its 4px
  //                   vertical padding inside .mobile-event-timeline
  // so the phone swap moves nothing: wrapper, controls, timeline and sidebar
  // all keep the box their placeholder occupied. No stand-in overrides
  // `background` — the shorthand resets the shared gradient and kills the
  // shimmer. The sidebar's panels are content-sized on the live page, so
  // their stand-ins fix their own heights; the column's x/width is what the
  // parity spec (web/layout-tests/replay-swap-parity.spec.ts) holds to.
  return `
    <div class="skeleton-page">
      <div class="replay-page">
        <h1 class="page-title">Replay Viewer</h1>
        <div class="replay-layout">
          <div class="replay-main">
            <div class="canvas-wrapper">
              ${CanvasSkeleton()}
              ${Skeleton({ variant: 'bar', width: '180px', height: '24px' })}
            </div>
            <div class="mobile-replay-controls">
              <div class="mobile-playback-bar">
                ${Skeleton({ variant: 'bar', width: '40px', height: '44px', extra: 'border-radius:4px' })}
                ${Skeleton({ variant: 'bar', width: '40px', height: '44px', extra: 'border-radius:4px' })}
                ${Skeleton({ variant: 'bar', width: '40px', height: '44px', extra: 'border-radius:4px' })}
                ${Skeleton({ variant: 'bar', width: '40px', height: '44px', extra: 'border-radius:4px' })}
                ${Skeleton({ variant: 'bar', width: '52px', height: '26px' })}
                ${Skeleton({ variant: 'bar', width: '60px', height: '44px', extra: 'border-radius:4px' })}
              </div>
              ${ScrubberSkeleton()}
            </div>
            <div class="mobile-event-timeline">
              ${Skeleton({ variant: 'bar', width: '60px', height: '26px' })}
            </div>
          </div>
          <div class="replay-sidebar">
            ${Skeleton({ variant: 'rectangle', width: '100%', height: '150px' })}
            ${Skeleton({ variant: 'rectangle', width: '100%', height: '120px' })}
            ${Skeleton({ variant: 'rectangle', width: '100%', height: '100px' })}
          </div>
        </div>
      </div>
    </div>`;
}

export function skeletonPlaylists(): string {
  const cards = Array.from({ length: 6 }, () =>
    `<div class="skeleton-card">
      ${Skeleton({ variant: 'bar', width: '100%', height: '140px', extra: 'border-radius:6px 6px 0 0' })}
      <div style="padding:12px">
        ${Skeleton({ variant: 'bar', width: '70%' })}
        ${Skeleton({ variant: 'bar', width: '100%', height: '12px', extra: 'margin-top:8px' })}
      </div>
    </div>`
  ).join('');
  return `
    <div class="skeleton-page">
      <h1 class="page-title">Replay Playlists</h1>
      ${Skeleton({ variant: 'bar', width: '300px', height: '14px', extra: 'margin-bottom:24px' })}
      <div class="skeleton-grid">${cards}</div>
    </div>`;
}

export function skeletonMatches(): string {
  const rows = Array.from({ length: 8 }, () =>
    `<div class="skeleton-row" style="padding:12px 0;border-bottom:1px solid var(--border)">
      ${Skeleton({ variant: 'bar', width: '200px' })} ${Skeleton({ variant: 'bar', width: '60px', height: '16px', extra: 'margin-left:auto' })} ${Skeleton({ variant: 'bar', width: '100px' })}
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
            ${Skeleton({ variant: 'bar', width: '60px', height: '14px', extra: 'margin:0 auto 8px' })}
            ${Skeleton({ variant: 'bar', width: '40px', height: '24px', extra: 'margin:0 auto' })}
          </div>`
        ).join('')}
      </div>
      ${Skeleton({ variant: 'bar', width: '100%', height: '300px', extra: 'border-radius:8px' })}
      ${Skeleton({ variant: 'bar', width: '100%', height: '120px', extra: 'border-radius:8px;margin-top:16px' })}
    </div>`;
}

export function skeletonBlog(): string {
  const posts = Array.from({ length: 4 }, () =>
    `<div class="skeleton-card">
      ${Skeleton({ variant: 'bar', width: '70%', height: '20px' })}
      ${Skeleton({ variant: 'bar', width: '100%', height: '12px', extra: 'margin-top:8px' })}
      ${Skeleton({ variant: 'bar', width: '100%', height: '12px', extra: 'margin-top:4px' })}
      ${Skeleton({ variant: 'bar', width: '50%', height: '12px', extra: 'margin-top:4px' })}
      ${Skeleton({ variant: 'bar', width: '60px', height: '14px', extra: 'margin-top:8px' })}
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
      ${Skeleton({ variant: 'bar', width: '50%', height: '20px' })}
      ${Skeleton({ variant: 'bar', width: '100%', height: '14px', extra: 'margin-top:8px' })}
      ${Skeleton({ variant: 'bar', width: '80px', height: '28px', extra: 'margin-top:12px;border-radius:6px' })}
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
        ${Skeleton({ variant: 'bar', width: '100%' })} ${Skeleton({ variant: 'bar', width: '80%' })} ${Skeleton({ variant: 'bar', width: '100%' })} ${Skeleton({ variant: 'bar', width: '60%' })} ${Skeleton({ variant: 'bar', width: '90%' })}
      </div>
    </div>`;
}
