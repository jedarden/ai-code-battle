// AnnotationOverlay — spatial + text replay annotations per §16.8
// Users add tagged feedback anchored to a (turn, grid position) pair.
// Feedback types match plan §8.3: insight, mistake, idea, highlight.
// Markers render on the canvas; annotations show in a side panel + event timeline.

import type { Position } from '../types';

// ─── Types ──────────────────────────────────────────────────────────────────

export type FeedbackType = 'insight' | 'mistake' | 'idea' | 'highlight';

export interface Annotation {
  id: string;
  match_id: string;
  turn: number;
  type: FeedbackType;
  body: string;
  author: string;
  upvotes: number;
  created_at: string;
  // Spatial data (optional — user may click a grid position)
  position?: Position;
}

export const FEEDBACK_TYPES: { type: FeedbackType; label: string; icon: string; color: string }[] = [
  { type: 'insight',    label: 'Tactical Insight', icon: '\u{1F4A1}', color: '#3b82f6' },
  { type: 'mistake',    label: 'Mistake Spotted',  icon: '⚠️',  color: '#ef4444' },
  { type: 'idea',       label: 'Strategy Idea',    icon: '\u{1F9EA}', color: '#22c55e' },
  { type: 'highlight',  label: 'Highlight',        icon: '⭐',  color: '#fbbf24' },
];

// ─── Storage ────────────────────────────────────────────────────────────────

const LS_KEY = 'acb_annotations_v2';

function saveLocal(ann: Annotation): void {
  try {
    const existing: Annotation[] = JSON.parse(localStorage.getItem(LS_KEY) ?? '[]');
    existing.push(ann);
    localStorage.setItem(LS_KEY, JSON.stringify(existing.slice(-200)));
  } catch { /* ignore */ }
}

export function loadLocalAnnotations(matchId?: string): Annotation[] {
  try {
    const all: Annotation[] = JSON.parse(localStorage.getItem(LS_KEY) ?? '[]');
    if (matchId) return all.filter(a => a.match_id === matchId);
    return all;
  } catch {
    return [];
  }
}

// ─── Fetch feedback.json from pre-built data ────────────────────────────────

export async function fetchFeedback(matchId: string): Promise<Annotation[]> {
  try {
    const resp = await fetch(`/data/matches/${matchId}/feedback.json`);
    if (!resp.ok) return [];
    return resp.json();
  } catch {
    return [];
  }
}

// ─── Submit (POST to API, localStorage fallback) ────────────────────────────

const API_BASE = '/api';

export async function submitAnnotation(ann: Annotation): Promise<boolean> {
  saveLocal(ann);
  try {
    const resp = await fetch(`${API_BASE}/feedback`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(ann),
    });
    return resp.ok;
  } catch {
    return false;
  }
}

// ─── AnnotationOverlay class ────────────────────────────────────────────────

export interface AnnotationOverlayOptions {
  onAnnotationAdd?: (ann: Annotation) => void;
  onTurnClick?: (turn: number) => void;
}

export class AnnotationOverlay {
  private container: HTMLElement;
  private annotations: Annotation[] = [];
  private currentTurn: number = 0;
  private totalTurns: number = 0;
  private matchId: string = '';
  private options: AnnotationOverlayOptions;

  constructor(container: HTMLElement, options: AnnotationOverlayOptions = {}) {
    this.container = container;
    this.options = options;
  }

  loadAnnotations(matchId: string, annotations: Annotation[], totalTurns: number): void {
    this.matchId = matchId;
    this.totalTurns = totalTurns;
    this.annotations = annotations.filter(a => a.match_id === this.matchId);
    this.render();
  }

  setCurrentTurn(turn: number): void {
    this.currentTurn = turn;
    this.updateHighlight();
  }

  addAnnotation(ann: Annotation): void {
    this.annotations.push(ann);
    this.render();
    if (this.options.onAnnotationAdd) this.options.onAnnotationAdd(ann);
  }

  getAnnotationsForTurn(turn: number): Annotation[] {
    return this.annotations.filter(a => a.turn === turn);
  }

  getAllAnnotations(): Annotation[] {
    return [...this.annotations];
  }

  // Get turns that have annotations (for rendering markers on canvas/timeline)
  getAnnotatedTurns(): number[] {
    const turns = new Set(this.annotations.map(a => a.turn));
    return [...turns].sort((a, b) => a - b);
  }

  // ── Render ────────────────────────────────────────────────────────────────

  private render(): void {
    if (this.annotations.length === 0) {
      this.container.innerHTML = '<div class="ann-overlay-empty">No annotations yet</div>';
      return;
    }

    const turnMarkers = this.renderTimelineMarkers();

    this.container.innerHTML = `
      <div class="ann-overlay-header">
        <span class="ann-overlay-title">Annotations</span>
        <span class="ann-overlay-count">${this.annotations.length}</span>
      </div>
      <div class="ann-overlay-track">
        ${turnMarkers}
      </div>
      <div class="ann-overlay-list">
        ${this.renderCurrentTurnAnnotations()}
      </div>
    `;

    this.wireClickHandlers();
    this.updateHighlight();
  }

  private renderTimelineMarkers(): string {
    const grouped = new Map<number, Annotation[]>();
    for (const ann of this.annotations) {
      const list = grouped.get(ann.turn) ?? [];
      list.push(ann);
      grouped.set(ann.turn, list);
    }

    return [...grouped.entries()].map(([turn, anns]) => {
      const pct = (turn / Math.max(1, this.totalTurns - 1)) * 100;
      const primaryType = anns[0].type;
      const config = FEEDBACK_TYPES.find(f => f.type === primaryType);
      const color = config?.color ?? '#94a3b8';
      const count = anns.length > 1 ? `<span class="ann-marker-count">${anns.length}</span>` : '';
      return `<div class="ann-marker" data-turn="${turn}" style="left:${pct.toFixed(1)}%;--ann-color:${color}">
        <span class="ann-marker-dot"></span>${count}
      </div>`;
    }).join('');
  }

  private renderCurrentTurnAnnotations(): string {
    const current = this.getAnnotationsForTurn(this.currentTurn);
    if (current.length === 0) {
      return '<div class="ann-no-current">No annotations at this turn</div>';
    }

    return current.map(ann => {
      const config = FEEDBACK_TYPES.find(f => f.type === ann.type);
      const color = config?.color ?? '#94a3b8';
      const icon = config?.icon ?? '';
      const label = config?.label ?? ann.type;
      const pos = ann.position ? `(${ann.position.row}, ${ann.position.col})` : '';
      return `<div class="ann-item" data-ann-id="${ann.id}">
        <div class="ann-item-header">
          <span class="ann-item-type" style="color:${color}">${icon} ${escapeHtml(label)}</span>
          <span class="ann-item-author">${escapeHtml(ann.author)}</span>
        </div>
        <div class="ann-item-body">${escapeHtml(ann.body)}</div>
        ${pos ? `<div class="ann-item-pos">@ ${pos}</div>` : ''}
        <div class="ann-item-meta">
          <span class="ann-item-upvotes">${ann.upvotes} upvotes</span>
          <span class="ann-item-time">${formatTime(ann.created_at)}</span>
        </div>
      </div>`;
    }).join('');
  }

  private wireClickHandlers(): void {
    this.container.querySelectorAll('.ann-marker').forEach(el => {
      el.addEventListener('click', () => {
        const turn = parseInt((el as HTMLElement).dataset.turn || '0', 10);
        if (this.options.onTurnClick) this.options.onTurnClick(turn);
      });
    });
  }

  private updateHighlight(): void {
    this.container.querySelectorAll('.ann-marker').forEach(el => {
      const turn = parseInt((el as HTMLElement).dataset.turn || '0', 10);
      el.classList.toggle('active', turn === this.currentTurn);
    });

    // Update annotation list for current turn
    const listEl = this.container.querySelector('.ann-overlay-list');
    if (listEl) listEl.innerHTML = this.renderCurrentTurnAnnotations();
  }

  // ── Canvas marker rendering ───────────────────────────────────────────────
  // Call this from ReplayViewer's render loop to draw annotation markers on the canvas

  static drawCanvasMarkers(
    ctx: CanvasRenderingContext2D,
    annotations: Annotation[],
    currentTurn: number,
    cellSize: number,
    _mapRows: number,
  ): void {
    const currentAnns = annotations.filter(a => a.turn === currentTurn);
    if (currentAnns.length === 0) return;

    ctx.save();

    for (const ann of currentAnns) {
      const config = FEEDBACK_TYPES.find(f => f.type === ann.type);
      const color = config?.color ?? '#94a3b8';

      if (ann.position) {
        // Draw marker at grid position
        const x = ann.position.col * cellSize + cellSize / 2;
        const y = ann.position.row * cellSize + cellSize / 2;
        const r = cellSize / 2 + 2;

        ctx.globalAlpha = 0.6;
        ctx.strokeStyle = color;
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.arc(x, y, r, 0, Math.PI * 2);
        ctx.stroke();

        ctx.globalAlpha = 0.15;
        ctx.fillStyle = color;
        ctx.beginPath();
        ctx.arc(x, y, r, 0, Math.PI * 2);
        ctx.fill();
      } else {
        // Draw a small indicator at top-right corner of the map
        const x = 0;
        const y = 0;
        ctx.globalAlpha = 0.8;
        ctx.fillStyle = color;
        ctx.beginPath();
        ctx.arc(x + cellSize - 2, y + 2, 3, 0, Math.PI * 2);
        ctx.fill();
      }
    }

    ctx.restore();
  }

  // Draw annotation count badges on the event timeline
  static drawTimelineBadges(
    ctx: CanvasRenderingContext2D,
    annotations: Annotation[],
    totalTurns: number,
    width: number,
    y: number,
    height: number,
  ): void {
    const grouped = new Map<number, Annotation[]>();
    for (const ann of annotations) {
      const list = grouped.get(ann.turn) ?? [];
      list.push(ann);
      grouped.set(ann.turn, list);
    }

    ctx.save();
    ctx.globalAlpha = 0.7;
    for (const [turn, anns] of grouped) {
      const pct = turn / Math.max(1, totalTurns - 1);
      const x = pct * width;
      const config = FEEDBACK_TYPES.find(f => f.type === anns[0].type);
      const color = config?.color ?? '#94a3b8';

      ctx.fillStyle = color;
      ctx.beginPath();
      ctx.arc(x, y + height / 2, anns.length > 1 ? 4 : 3, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.restore();
  }

  destroy(): void {
    this.container.innerHTML = '';
  }
}

// ─── Annotation form (embeddable in any panel) ──────────────────────────────

export interface AnnotationFormOptions {
  matchId: string;
  currentTurn: number;
  authorName: string;
  onSubmit?: (ann: Annotation) => void;
}

export function createAnnotationForm(
  container: HTMLElement,
  getTurn: () => number,
  getMatchId: () => string,
  getGridPosition: () => Position | undefined,
): void {
  const authorKey = 'acb_author_name';
  const savedAuthor = localStorage.getItem(authorKey) || '';

  container.innerHTML = `
    <div class="ann-form">
      <div class="ann-form-types">
        ${FEEDBACK_TYPES.map(ft =>
          `<button class="ann-type-btn" data-type="${ft.type}" style="--ann-color:${ft.color}" title="${ft.label}">
            <span class="ann-type-icon">${ft.icon}</span>
            <span class="ann-type-label">${ft.label}</span>
          </button>`,
        ).join('')}
      </div>
      <div class="ann-form-fields">
        <input type="text" class="ann-author-input" placeholder="Your name" value="${escapeHtml(savedAuthor)}" maxlength="64">
        <textarea class="ann-body-input" placeholder="What happened here? (max 500 chars)" maxlength="500" rows="2"></textarea>
        <span class="ann-char-count">0 / 500</span>
      </div>
      <button class="btn primary ann-submit-btn" disabled>Add Annotation</button>
    </div>
  `;

  let selectedType: FeedbackType | null = null;
  const typeBtns = container.querySelectorAll('.ann-type-btn');
  const bodyInput = container.querySelector('.ann-body-input') as HTMLTextAreaElement;
  const charCount = container.querySelector('.ann-char-count') as HTMLSpanElement;
  const authorInput = container.querySelector('.ann-author-input') as HTMLInputElement;
  const submitBtn = container.querySelector('.ann-submit-btn') as HTMLButtonElement;

  typeBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      typeBtns.forEach(b => b.classList.remove('selected'));
      btn.classList.add('selected');
      selectedType = (btn as HTMLElement).dataset.type as FeedbackType;
      updateSubmitState();
    });
  });

  bodyInput.addEventListener('input', () => {
    charCount.textContent = `${bodyInput.value.length} / 500`;
    updateSubmitState();
  });

  authorInput.addEventListener('input', () => {
    localStorage.setItem(authorKey, authorInput.value.trim());
  });

  function updateSubmitState(): void {
    submitBtn.disabled = !selectedType || bodyInput.value.trim().length === 0;
  }

  submitBtn.addEventListener('click', async () => {
    if (!selectedType || !bodyInput.value.trim()) return;

    const author = authorInput.value.trim() || 'Anonymous';
    const matchId = getMatchId();
    const turn = getTurn();
    const position = getGridPosition();

    const ann: Annotation = {
      id: `ann_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`,
      match_id: matchId,
      turn,
      type: selectedType,
      body: bodyInput.value.trim(),
      author,
      upvotes: 0,
      created_at: new Date().toISOString(),
      position,
    };

    submitBtn.disabled = true;
    submitBtn.textContent = 'Submitting...';

    await submitAnnotation(ann);

    // Reset form
    typeBtns.forEach(b => b.classList.remove('selected'));
    selectedType = null;
    bodyInput.value = '';
    charCount.textContent = '0 / 500';
    submitBtn.textContent = 'Add Annotation';
    submitBtn.disabled = true;

    // Dispatch custom event so the replay page can handle it
    container.dispatchEvent(new CustomEvent('annotation-added', { detail: ann, bubbles: true }));
  });
}

// ─── Styles ─────────────────────────────────────────────────────────────────

export const ANNOTATION_OVERLAY_STYLES = `
  .ann-overlay-empty { color: var(--text-muted); font-size: 0.8rem; padding: 8px; text-align: center; }
  .ann-overlay-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
  .ann-overlay-title { font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; font-weight: 600; }
  .ann-overlay-count { font-size: 0.7rem; background: var(--bg-tertiary); color: var(--text-muted); padding: 2px 6px; border-radius: 8px; }
  .ann-overlay-track { position: relative; height: 16px; background: var(--bg-tertiary); border-radius: 4px; margin-bottom: 10px; }
  .ann-marker { position: absolute; top: 1px; bottom: 1px; display: flex; align-items: center; justify-content: center; transform: translateX(-50%); cursor: pointer; }
  .ann-marker-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--ann-color, #94a3b8); transition: transform 0.15s; }
  .ann-marker:hover .ann-marker-dot { transform: scale(1.5); }
  .ann-marker.active .ann-marker-dot { transform: scale(1.8); box-shadow: 0 0 4px var(--ann-color); }
  .ann-marker-count { position: absolute; top: -8px; font-size: 0.55rem; color: var(--text-muted); font-weight: 600; }
  .ann-overlay-list { display: flex; flex-direction: column; gap: 6px; max-height: 180px; overflow-y: auto; }
  .ann-no-current { color: var(--text-muted); font-size: 0.75rem; font-style: italic; }
  .ann-item { background: var(--bg-tertiary); border-radius: 6px; padding: 8px; border-left: 3px solid var(--ann-color, #475569); }
  .ann-item-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
  .ann-item-type { font-size: 0.75rem; font-weight: 600; }
  .ann-item-author { font-size: 0.65rem; color: var(--text-muted); }
  .ann-item-body { font-size: 0.8rem; color: var(--text-secondary); line-height: 1.4; margin-bottom: 4px; }
  .ann-item-pos { font-size: 0.65rem; color: var(--text-muted); font-family: monospace; margin-bottom: 4px; }
  .ann-item-meta { display: flex; gap: 10px; font-size: 0.65rem; color: var(--text-muted); }
  /* Annotation form */
  .ann-form { display: flex; flex-direction: column; gap: 10px; }
  .ann-form-types { display: flex; flex-wrap: wrap; gap: 4px; }
  .ann-type-btn { display: flex; align-items: center; gap: 4px; background: var(--bg-primary); border: 2px solid var(--ann-color, #475569); color: var(--ann-color, #94a3b8); padding: 4px 10px; border-radius: 16px; cursor: pointer; font-size: 0.75rem; transition: all 0.15s; }
  .ann-type-btn:hover { background: color-mix(in srgb, var(--ann-color, #475569) 15%, transparent); }
  .ann-type-btn.selected { background: color-mix(in srgb, var(--ann-color, #475569) 25%, transparent); font-weight: 600; }
  .ann-type-icon { font-size: 0.85rem; }
  .ann-type-label { font-size: 0.7rem; }
  .ann-form-fields { display: flex; flex-direction: column; gap: 6px; }
  .ann-author-input, .ann-body-input { width: 100%; background: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 6px 8px; border-radius: 6px; font-size: 0.8rem; font-family: inherit; }
  .ann-body-input { resize: vertical; }
  .ann-char-count { font-size: 0.6rem; color: var(--text-muted); text-align: right; }
  .ann-submit-btn { align-self: flex-end; }
`;

// ─── Utilities ──────────────────────────────────────────────────────────────

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function formatTime(iso: string): string {
  try {
    const d = new Date(iso);
    const now = new Date();
    const diffMs = now.getTime() - d.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 1) return 'just now';
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffHr = Math.floor(diffMin / 60);
    if (diffHr < 24) return `${diffHr}h ago`;
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  } catch {
    return '';
  }
}
