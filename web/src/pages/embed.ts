// Embeddable replay viewer - minimal chrome for iframe embedding
// §13.4: /embed/{id} - auto-play, ~50KB, Open Graph tags

const loadReplayViewer = () => import('../replay-viewer');

export function renderEmbedPage(params: Record<string, string>): void {
  const app = document.getElementById('app');
  if (!app) return;

  // Parse query params
  const startTurn = params.start ? parseInt(params.start, 10) : 0;
  const speed = params.speed ? parseInt(params.speed, 10) : 4; // Default 4x speed
  const mode = (params.mode as 'dots' | 'voronoi' | 'influence' | 'standard') || 'dots';

  // Minimal embed layout - no chrome, just the canvas
  app.innerHTML = `
    <div class="embed-page">
      <div class="canvas-wrapper" style="position:relative;width:100%;height:100vh;background:#0f172a">
        <canvas id="replay-canvas" style="touch-action:none;display:block"></canvas>
        <div id="no-replay" class="no-replay-message" style="position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);color:#94a3b8;font:16px system-ui">Loading replay…</div>
        <!-- Minimal controls overlay (hover to show) -->
        <div class="embed-controls" style="position:absolute;bottom:0;left:0;right:0;padding:8px;background:linear-gradient(transparent,rgba(15,23,42,0.9));opacity:0;transition:opacity 0.2s;display:flex;align-items:center;gap:8px">
          <button id="reset-btn" class="btn small" aria-label="Reset to start" style="padding:4px 8px;font-size:12px" disabled>&#9204;</button>
          <button id="play-btn" class="btn small primary" aria-label="Play or pause" style="padding:4px 8px;font-size:12px" disabled>&#9654;</button>
          <span id="turn-info" style="color:#e5e7eb;font:12px monospace;white-space:nowrap">T: 0/0</span>
        </div>
      </div>
    </div>
  `;

  // Show controls on hover
  const wrapper = app.querySelector('.canvas-wrapper') as HTMLElement;
  const controls = app.querySelector('.embed-controls') as HTMLElement;
  if (wrapper && controls) {
    wrapper.addEventListener('mouseenter', () => controls.style.opacity = '1');
    wrapper.addEventListener('mouseleave', () => controls.style.opacity = '0');
  }

  loadReplayViewer().then(({ ReplayViewer }) => {
    const replayUrl = params.id ? `/r2/replays/${params.id}.json.gz` : undefined;
    if (!replayUrl) {
      const noReplay = document.getElementById('no-replay');
      if (noReplay) noReplay.textContent = 'No replay specified';
      return;
    }

    // Initialize viewer
    const canvas = document.getElementById('replay-canvas') as HTMLCanvasElement;
    if (!canvas) return;

    const viewer = new ReplayViewer(canvas, {
      cellSize: 16,
      showGrid: true,
      fogOfWarPlayer: null,
      animationSpeed: 100 / speed, // Convert speed multiplier to ms per turn
      viewMode: mode,
      showDebug: false,
    });

    // Track play state for button toggle
    let isPlaying = false;

    // Hook into play state changes
    viewer.onPlayStateChange = (playing: boolean) => {
      isPlaying = playing;
      const playBtn = document.getElementById('play-btn') as HTMLButtonElement;
      if (playBtn) {
        playBtn.textContent = playing ? '⏸' : '▶';
      }
    };

    // Load replay
    fetch(replayUrl)
      .then(resp => {
        if (!resp.ok) throw new Error(`Failed to load replay: ${resp.status}`);
        return resp.json();
      })
      .then((replay: any) => {
        viewer.loadReplay(replay);
        viewer.setTurn(startTurn);
        viewer.play();

        // Update controls
        const resetBtn = document.getElementById('reset-btn') as HTMLButtonElement;
        const playBtn = document.getElementById('play-btn') as HTMLButtonElement;
        const turnInfo = document.getElementById('turn-info') as HTMLSpanElement;

        if (resetBtn) {
          resetBtn.disabled = false;
          resetBtn.onclick = () => {
            viewer.setTurn(0);
            if (!isPlaying) viewer.play();
            updateTurnInfo();
          };
        }

        if (playBtn) {
          playBtn.disabled = false;
          playBtn.onclick = () => {
            if (isPlaying) {
              // No pause method available - just stop by setting turn
              viewer.setTurn(viewer.getTurn());
            } else {
              viewer.play();
            }
          };
        }

        function updateTurnInfo() {
          if (turnInfo) {
            turnInfo.textContent = `T: ${viewer.getTurn()}/${viewer.getTotalTurns()}`;
          }
        }

        // Update turn info periodically
        setInterval(updateTurnInfo, 200);
      })
      .catch(err => {
        const noReplay = document.getElementById('no-replay');
        if (noReplay) noReplay.textContent = `Failed to load replay: ${err.message}`;
      });
  });
}
