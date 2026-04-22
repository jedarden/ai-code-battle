// Community replay feedback: users annotate replay turns with tags.
// Annotations feed the evolution pipeline by surfacing interesting moments.
// Types consolidated with annotation.ts to use shared Annotation schema from plan §8.3.

import { fetchMatchIndex, type MatchSummary } from '../api-types';
import { ReplayViewer } from '../replay-viewer';
import type { Replay } from '../types';
import {
  type Annotation,
  type FeedbackType,
  FEEDBACK_TYPES,
  loadLocalAnnotations,
  submitAnnotation,
} from '../components/annotation';

// Re-export for any consumers
export { FEEDBACK_TYPES as ANNOTATION_TAGS };

// ─── Page render ─────────────────────────────────────────────────────────────

export function renderFeedbackPage(_params: Record<string, string>): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = buildHTML();
  requestAnimationFrame(() => initFeedback());
}

function buildHTML(): string {
  const tagButtons = FEEDBACK_TYPES.map(t =>
    `<button class="tag-btn" data-tag="${t.type}" style="--tag-color:${t.color}" title="${escapeHtml(t.label)}">${t.icon} ${escapeHtml(t.label)}</button>`,
  ).join('');

  return `
    <div class="feedback-page">
      <h1 class="page-title">Community Replay Feedback</h1>
      <p class="feedback-intro">
        Annotate key moments in replays. High-quality annotations are used to seed the
        evolution pipeline with interesting positions.
      </p>

      <div class="feedback-layout">
        <!-- Left: load replay -->
        <div class="feedback-left">
          <div class="fb-panel">
            <div class="panel-header"><span>Load a Replay</span></div>

            <div class="load-tabs">
              <button class="tab-btn active" data-tab="recent">Recent Matches</button>
              <button class="tab-btn" data-tab="file">Upload File</button>
              <button class="tab-btn" data-tab="url">By URL</button>
            </div>

            <!-- Recent matches tab -->
            <div id="tab-recent" class="tab-content">
              <div id="recent-matches-list" class="recent-list">
                <div class="loading">Loading recent matches…</div>
              </div>
            </div>

            <!-- File upload tab -->
            <div id="tab-file" class="tab-content hidden">
              <label class="btn secondary" for="fb-file-input">Choose Replay File (.json)</label>
              <input type="file" id="fb-file-input" accept=".json" style="display:none">
            </div>

            <!-- URL tab -->
            <div id="tab-url" class="tab-content hidden">
              <div class="url-row">
                <input type="text" id="fb-url-input" placeholder="Replay URL…" class="url-input">
                <button id="fb-load-url-btn" class="btn primary small">Load</button>
              </div>
            </div>

            <div id="fb-load-status" class="fb-status hidden"></div>
          </div>

          <!-- Annotation form (hidden until replay loaded) -->
          <div class="fb-panel" id="annotation-form-panel" style="display:none">
            <div class="panel-header"><span>Annotate Turn <span id="annotate-turn-num">—</span></span></div>

            <div class="turn-nav">
              <button id="ann-prev-btn" class="btn small">Prev</button>
              <input type="range" id="ann-turn-slider" min="0" max="0" value="0" class="turn-slider">
              <button id="ann-next-btn" class="btn small">Next</button>
            </div>

            <div class="tag-section">
              <label class="form-label">Tag this moment:</label>
              <div class="tag-buttons" id="tag-buttons">
                ${tagButtons}
              </div>
            </div>

            <div class="comment-section">
              <label class="form-label" for="ann-comment">Comment (optional)</label>
              <textarea id="ann-comment" class="ann-textarea" rows="3" placeholder="Describe what's happening here…" maxlength="280"></textarea>
              <span id="ann-comment-len" class="char-count">0 / 280</span>
            </div>

            <button id="submit-annotation-btn" class="btn primary" disabled>Submit Annotation</button>
            <div id="submit-status" class="fb-status hidden"></div>
          </div>

          <!-- Submitted annotations log -->
          <div class="fb-panel" id="annotations-log-panel" style="display:none">
            <div class="panel-header"><span>Your Annotations</span></div>
            <div id="annotations-log" class="annotations-log"></div>
          </div>
        </div>

        <!-- Right: replay viewer -->
        <div class="feedback-right" id="fb-viewer-col" style="display:none">
          <div class="fb-panel">
            <div class="panel-header">
              <span id="fb-replay-title">Replay</span>
              <span id="fb-replay-info" class="replay-info"></span>
            </div>
            <canvas id="fb-canvas" class="fb-canvas"></canvas>
            <div class="viewer-controls">
              <button id="fb-play-btn" class="btn small">Play</button>
              <button id="fb-reset-btn" class="btn small secondary">Reset</button>
              <span id="fb-turn-label" class="turn-label">Turn 0 / 0</span>
            </div>
            <!-- Annotation markers overlaid on replay -->
            <div id="fb-annotation-markers" class="annotation-markers"></div>
          </div>
        </div>
      </div>
    </div>
    ${FEEDBACK_STYLES}
  `;
}

// ─── Initialisation ───────────────────────────────────────────────────────────

function initFeedback(): void {
  let replay: Replay | null = null;
  let viewer: ReplayViewer | null = null;
  let selectedTag: FeedbackType | null = null;
  const localAnnotations: Annotation[] = [];

  const loadStatus = document.getElementById('fb-load-status')!;
  const formPanel  = document.getElementById('annotation-form-panel')!;
  const logPanel   = document.getElementById('annotations-log-panel')!;
  const viewerCol  = document.getElementById('fb-viewer-col')!;
  const turnNum    = document.getElementById('annotate-turn-num')!;
  const turnSlider = document.getElementById('ann-turn-slider') as HTMLInputElement;
  const canvas     = document.getElementById('fb-canvas') as HTMLCanvasElement;
  const replayTitle = document.getElementById('fb-replay-title')!;
  const replayInfo = document.getElementById('fb-replay-info')!;
  const turnLabel  = document.getElementById('fb-turn-label')!;
  const submitBtn  = document.getElementById('submit-annotation-btn') as HTMLButtonElement;
  const commentTa  = document.getElementById('ann-comment') as HTMLTextAreaElement;
  const commentLen = document.getElementById('ann-comment-len')!;
  const submitStatus = document.getElementById('submit-status')!;

  // ── Tab switching ──────────────────────────────────────────────────────────
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const tab = (btn as HTMLElement).dataset.tab!;
      document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      document.querySelectorAll('.tab-content').forEach(c => c.classList.add('hidden'));
      document.getElementById(`tab-${tab}`)?.classList.remove('hidden');
    });
  });

  // ── Load recent matches ────────────────────────────────────────────────────
  fetchMatchIndex().then(idx => {
    const listEl = document.getElementById('recent-matches-list')!;
    const recent = idx.matches.slice(0, 20);
    if (recent.length === 0) {
      listEl.innerHTML = '<div class="empty-state-sm">No matches recorded yet.</div>';
      return;
    }
    listEl.innerHTML = recent.map(m => `
      <div class="recent-match-row" data-match-id="${m.id}">
        <div class="recent-match-bots">${m.participants.map(p => escapeHtml(p.name)).join(' vs ')}</div>
        <div class="recent-match-meta">
          <span>${m.turns ?? '?'} turns</span>
          <span>${formatDate(m.completed_at)}</span>
        </div>
      </div>
    `).join('');

    listEl.querySelectorAll('.recent-match-row').forEach(row => {
      row.addEventListener('click', async () => {
        const mid = (row as HTMLElement).dataset.matchId!;
        const match = recent.find(m => m.id === mid)!;
        await loadReplayFromUrl(replayUrlForMatch(match));
      });
    });
  }).catch(() => {
    const listEl = document.getElementById('recent-matches-list')!;
    listEl.innerHTML = '<div class="empty-state-sm">Could not load match list.</div>';
  });

  // ── File upload ────────────────────────────────────────────────────────────
  document.getElementById('fb-file-input')!.addEventListener('change', async (e) => {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      loadReplayData(JSON.parse(text) as Replay);
    } catch (err) {
      showLoadError('Parse error: ' + err);
    }
  });

  // ── URL load ───────────────────────────────────────────────────────────────
  document.getElementById('fb-load-url-btn')!.addEventListener('click', () => {
    const url = (document.getElementById('fb-url-input') as HTMLInputElement).value.trim();
    if (url) loadReplayFromUrl(url);
  });

  async function loadReplayFromUrl(url: string): Promise<void> {
    loadStatus.textContent = 'Loading…';
    loadStatus.className = 'fb-status';
    try {
      const resp = await fetch(url);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      loadReplayData((await resp.json()) as Replay);
    } catch (err) {
      showLoadError('Failed to load: ' + err);
    }
  }

  function showLoadError(msg: string): void {
    loadStatus.textContent = msg;
    loadStatus.className = 'fb-status error';
  }

  function loadReplayData(data: Replay): void {
    replay = data;
    const total = data.turns.length - 1;

    loadStatus.textContent = `Loaded: ${data.match_id}`;
    loadStatus.className = 'fb-status ok';

    // Setup viewer
    viewerCol.style.display = '';
    viewer = new ReplayViewer(canvas, { cellSize: 10, showGrid: false });
    viewer.loadReplay(data);
    viewer.onTurnChange = () => updateTurnUI(viewer!.getTurn(), total);

    replayTitle.textContent = 'Replay';
    replayInfo.textContent = `${data.match_id.slice(0, 8)}… · ${total + 1} turns`;

    // Setup annotation form
    turnSlider.max = String(total);
    turnSlider.value = '0';
    updateTurnUI(0, total);

    formPanel.style.display = '';
    updateAnnotationMarkers();

    document.getElementById('fb-play-btn')!.addEventListener('click', () => viewer?.togglePlay(), { once: false });
    document.getElementById('fb-reset-btn')!.addEventListener('click', () => { viewer?.pause(); viewer?.setTurn(0); });
  }

  function updateTurnUI(turn: number, total: number): void {
    turnNum.textContent = String(turn);
    turnSlider.value = String(turn);
    turnLabel.textContent = `Turn ${turn} / ${total}`;
  }

  // ── Playback controls ──────────────────────────────────────────────────────
  turnSlider.addEventListener('input', () => {
    const t = Number(turnSlider.value);
    viewer?.setTurn(t);
    updateTurnUI(t, Number(turnSlider.max));
  });

  document.getElementById('ann-prev-btn')!.addEventListener('click', () => {
    if (!viewer) return;
    const t = Math.max(0, viewer.getTurn() - 1);
    viewer.setTurn(t);
    updateTurnUI(t, Number(turnSlider.max));
  });

  document.getElementById('ann-next-btn')!.addEventListener('click', () => {
    if (!viewer) return;
    const t = Math.min(Number(turnSlider.max), viewer.getTurn() + 1);
    viewer.setTurn(t);
    updateTurnUI(t, Number(turnSlider.max));
  });

  // ── Tag selection ──────────────────────────────────────────────────────────
  document.getElementById('tag-buttons')!.querySelectorAll('.tag-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.tag-btn').forEach(b => b.classList.remove('selected'));
      btn.classList.add('selected');
      selectedTag = (btn as HTMLElement).dataset.tag! as FeedbackType;
      updateSubmitButton();
    });
  });

  commentTa.addEventListener('input', () => {
    const len = commentTa.value.length;
    commentLen.textContent = `${len} / 280`;
    updateSubmitButton();
  });

  function updateSubmitButton(): void {
    submitBtn.disabled = !selectedTag || !replay;
  }

  // ── Submit annotation ──────────────────────────────────────────────────────
  submitBtn.addEventListener('click', async () => {
    if (!replay || !selectedTag) return;

    const annotation: Annotation = {
      id: `ann_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`,
      match_id: replay.match_id,
      turn: Number(turnSlider.value),
      type: selectedTag,
      body: commentTa.value.trim(),
      author: 'Anonymous',
      upvotes: 0,
      created_at: new Date().toISOString(),
    };

    submitBtn.disabled = true;
    submitStatus.textContent = 'Submitting…';
    submitStatus.className = 'fb-status';

    try {
      await submitAnnotation(annotation);

      submitStatus.textContent = 'Annotation submitted! Thank you.';
      submitStatus.className = 'fb-status ok';

      localAnnotations.push(annotation);

      // Reset form
      document.querySelectorAll('.tag-btn').forEach(b => b.classList.remove('selected'));
      selectedTag = null;
      commentTa.value = '';
      commentLen.textContent = '0 / 280';
      updateSubmitButton();

      logPanel.style.display = '';
      renderAnnotationsLog(localAnnotations);
      updateAnnotationMarkers();
    } catch (err) {
      submitStatus.textContent = 'Error: ' + err;
      submitStatus.className = 'fb-status error';
    } finally {
      submitBtn.disabled = !selectedTag;
    }
  });

  // Load any previously stored annotations for any match
  const stored = loadLocalAnnotations();
  if (stored.length > 0) {
    localAnnotations.push(...stored);
    logPanel.style.display = '';
    renderAnnotationsLog(localAnnotations);
  }

  function updateAnnotationMarkers(): void {
    if (!replay) return;
    const total = replay.turns.length;
    const markersEl = document.getElementById('fb-annotation-markers')!;
    const relevant = localAnnotations.filter(a => a.match_id === replay!.match_id);

    if (relevant.length === 0) {
      markersEl.innerHTML = '';
      return;
    }

    markersEl.innerHTML = relevant.map(a => {
      const pct = (a.turn / Math.max(1, total - 1)) * 100;
      const tagInfo = FEEDBACK_TYPES.find(t => t.type === a.type);
      const color = tagInfo?.color ?? '#94a3b8';
      return `<div class="ann-marker" style="left:${pct.toFixed(1)}%;background:${color}"
        title="Turn ${a.turn}: ${escapeHtml(tagInfo?.label ?? a.type)}${a.body ? ' — ' + escapeHtml(a.body) : ''}"></div>`;
    }).join('');
  }

  function renderAnnotationsLog(anns: Annotation[]): void {
    const logEl = document.getElementById('annotations-log')!;
    const sorted = [...anns].sort((a, b) => a.turn - b.turn);
    logEl.innerHTML = sorted.map(a => {
      const tagInfo = FEEDBACK_TYPES.find(t => t.type === a.type);
      return `
        <div class="ann-log-row">
          <span class="ann-tag-pill" style="background:${tagInfo?.color ?? '#94a3b8'}22;color:${tagInfo?.color ?? '#94a3b8'}">${tagInfo?.icon ?? ''} ${escapeHtml(tagInfo?.label ?? a.type)}</span>
          <span class="ann-turn">Turn ${a.turn}</span>
          ${a.body ? `<span class="ann-comment-text">${escapeHtml(a.body)}</span>` : ''}
          <span class="ann-match-id">${a.match_id.slice(0, 8)}…</span>
        </div>
      `;
    }).join('');
  }
}

// ─── Utilities ────────────────────────────────────────────────────────────────

// ─── Utilities ────────────────────────────────────────────────────────────────

function replayUrlForMatch(m: MatchSummary): string {
  return `/replays/${m.id}.json.gz`;
}

function formatDate(s: string | null): string {
  if (!s) return '–';
  return new Date(s).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// ─── Styles ───────────────────────────────────────────────────────────────────

const FEEDBACK_STYLES = `
<style>
.feedback-intro { color: var(--text-muted); margin-bottom: 24px; max-width: 700px; }
.feedback-layout { display: flex; gap: 20px; align-items: flex-start; }
.feedback-left { width: 360px; flex-shrink: 0; display: flex; flex-direction: column; gap: 16px; }
.feedback-right { flex: 1; min-width: 0; }
.fb-panel { background: var(--bg-secondary); border-radius: 8px; padding: 16px; }
.panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; color: var(--text-primary); }
.replay-info { font-size: 0.75rem; color: var(--text-muted); font-weight: 400; }
.load-tabs { display: flex; gap: 4px; margin-bottom: 12px; }
.tab-btn { background: var(--bg-tertiary); border: none; color: var(--text-muted); padding: 6px 12px; border-radius: 6px; cursor: pointer; font-size: 0.8rem; }
.tab-btn.active { background: var(--accent); color: #fff; }
.tab-content.hidden { display: none; }
.tab-content { padding: 4px 0; }
.url-row { display: flex; gap: 8px; }
.url-input { flex: 1; background: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 7px 10px; border-radius: 6px; font-size: 0.875rem; }
.fb-status { font-size: 0.8rem; padding: 8px; border-radius: 4px; margin-top: 8px; }
.fb-status.hidden { display: none; }
.fb-status.ok { background: rgba(34,197,94,0.15); color: var(--success); }
.fb-status.error { background: rgba(239,68,68,0.15); color: var(--error); }
.recent-list { max-height: 260px; overflow-y: auto; display: flex; flex-direction: column; gap: 4px; }
.recent-match-row { background: var(--bg-primary); border-radius: 6px; padding: 8px 12px; cursor: pointer; transition: background 0.15s; }
.recent-match-row:hover { background: var(--bg-tertiary); }
.recent-match-bots { font-size: 0.875rem; color: var(--text-primary); margin-bottom: 2px; }
.recent-match-meta { display: flex; gap: 12px; font-size: 0.75rem; color: var(--text-muted); }
.empty-state-sm { color: var(--text-muted); font-size: 0.875rem; padding: 12px 0; }
.turn-nav { display: flex; gap: 8px; align-items: center; margin-bottom: 14px; }
.turn-slider { flex: 1; }
.tag-section { margin-bottom: 14px; }
.form-label { display: block; font-size: 0.8rem; color: var(--text-muted); margin-bottom: 6px; }
.tag-buttons { display: flex; flex-wrap: wrap; gap: 6px; }
.tag-btn { background: var(--bg-primary); border: 2px solid var(--tag-color, #475569); color: var(--tag-color, #94a3b8); padding: 5px 10px; border-radius: 20px; cursor: pointer; font-size: 0.8rem; transition: all 0.15s; }
.tag-btn:hover { background: color-mix(in srgb, var(--tag-color, #475569) 15%, transparent); }
.tag-btn.selected { background: color-mix(in srgb, var(--tag-color, #475569) 20%, transparent); font-weight: 600; }
.comment-section { margin-bottom: 14px; }
.ann-textarea { width: 100%; background: var(--bg-primary); border: 1px solid var(--border); color: var(--text-primary); padding: 8px; border-radius: 6px; font-size: 0.875rem; resize: vertical; font-family: inherit; }
.char-count { font-size: 0.7rem; color: var(--text-muted); float: right; }
.fb-canvas { display: block; width: 100%; border-radius: 6px; background: var(--bg-primary); }
.viewer-controls { display: flex; gap: 8px; align-items: center; margin-top: 10px; }
.turn-label { color: var(--text-muted); font-size: 0.875rem; margin-left: auto; }
.annotation-markers { position: relative; height: 16px; background: var(--bg-tertiary); border-radius: 4px; margin-top: 8px; }
.ann-marker { position: absolute; top: 2px; bottom: 2px; width: 4px; transform: translateX(-50%); border-radius: 2px; cursor: pointer; }
.annotations-log { display: flex; flex-direction: column; gap: 6px; max-height: 200px; overflow-y: auto; }
.ann-log-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 0.8rem; padding: 4px 0; border-bottom: 1px solid var(--bg-tertiary); }
.ann-tag-pill { padding: 2px 8px; border-radius: 10px; font-size: 0.75rem; font-weight: 600; }
.ann-turn { color: var(--text-muted); }
.ann-comment-text { color: var(--text-secondary); flex: 1; }
.ann-match-id { color: var(--text-muted); font-family: monospace; font-size: 0.7rem; margin-left: auto; }
@media (max-width: 900px) {
  .feedback-layout { flex-direction: column; }
  .feedback-left { width: 100%; }
}
</style>
`;
