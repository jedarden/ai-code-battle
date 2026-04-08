import { ReplayViewer } from './replay-viewer';
import type { Replay } from './types';

// DOM elements
const canvas = document.getElementById('replay-canvas') as HTMLCanvasElement;
const noReplayDiv = document.getElementById('no-replay') as HTMLDivElement;
const fileInput = document.getElementById('file-input') as HTMLInputElement;
const urlInput = document.getElementById('url-input') as HTMLInputElement;
const loadUrlBtn = document.getElementById('load-url-btn') as HTMLButtonElement;
const playBtn = document.getElementById('play-btn') as HTMLButtonElement;
const prevBtn = document.getElementById('prev-btn') as HTMLButtonElement;
const nextBtn = document.getElementById('next-btn') as HTMLButtonElement;
const resetBtn = document.getElementById('reset-btn') as HTMLButtonElement;
const turnDisplay = document.getElementById('turn-display') as HTMLSpanElement;
const totalTurnsSpan = document.getElementById('total-turns') as HTMLSpanElement;
const turnSlider = document.getElementById('turn-slider') as HTMLInputElement;
const speedDisplay = document.getElementById('speed-display') as HTMLSpanElement;
const speedSlider = document.getElementById('speed-slider') as HTMLInputElement;
const fogSelect = document.getElementById('fog-select') as HTMLSelectElement;
const cellSizeSelect = document.getElementById('cell-size-select') as HTMLSelectElement;
const eventLogDiv = document.getElementById('event-log') as HTMLDivElement;

// Info elements
const infoMatchId = document.getElementById('info-match-id') as HTMLElement;
const infoWinner = document.getElementById('info-winner') as HTMLElement;
const infoTurns = document.getElementById('info-turns') as HTMLElement;
const infoReason = document.getElementById('info-reason') as HTMLElement;

// Initialize viewer
let viewer = new ReplayViewer(canvas, { cellSize: 16 });

// Enable controls when replay is loaded
function enableControls(): void {
  playBtn.disabled = false;
  prevBtn.disabled = false;
  nextBtn.disabled = false;
  resetBtn.disabled = false;
  turnSlider.disabled = false;
  noReplayDiv.style.display = 'none';
}

// Update UI state
function updateUI(): void {
  turnDisplay.textContent = String(viewer.getTurn());
  totalTurnsSpan.textContent = String(viewer.getTotalTurns());
  turnSlider.value = String(viewer.getTurn());

  // Update play button text
  playBtn.textContent = 'Pause';
  if (!viewer.getReplay() || viewer.isAtEnd()) {
    playBtn.textContent = 'Play';
  }
}

// Update event log
function updateEventLog(): void {
  const events = viewer.getTurnEvents();
  if (events.length === 0) {
    eventLogDiv.innerHTML = '<div class="no-replay">No events</div>';
    return;
  }

  eventLogDiv.innerHTML = events.map(e => {
    const type = e.type.replace(/_/g, ' ');
    return `<div class="event"><span class="event-type">${type}</span></div>`;
  }).join('');
}

// Update match info panel
function updateMatchInfo(replay: Replay): void {
  infoMatchId.textContent = replay.match_id;
  infoTurns.textContent = String(replay.result.turns);
  infoReason.textContent = replay.result.reason;

  if (replay.result.winner >= 0 && replay.result.winner < replay.players.length) {
    infoWinner.textContent = replay.players[replay.result.winner].name;
  } else if (replay.result.winner === -1) {
    infoWinner.textContent = 'Draw';
  } else {
    infoWinner.textContent = 'Player ' + replay.result.winner;
  }

  // Update fog of war options
  fogSelect.innerHTML = '<option value="">Disabled (full view)</option>';
  replay.players.forEach((player, idx) => {
    const option = document.createElement('option');
    option.value = String(idx);
    option.textContent = player.name;
    fogSelect.appendChild(option);
  });
}

// Load replay from JSON
function loadReplay(replay: Replay): void {
  viewer.loadReplay(replay);
  enableControls();
  updateMatchInfo(replay);

  // Update slider max
  turnSlider.max = String(viewer.getTotalTurns() - 1);

  updateUI();
  updateEventLog();
}

// File input handler
fileInput.addEventListener('change', async (e) => {
  const file = (e.target as HTMLInputElement).files?.[0];
  if (!file) return;

  try {
    const text = await file.text();
    const replay = JSON.parse(text) as Replay;
    loadReplay(replay);
  } catch (err) {
    alert('Failed to load replay: ' + err);
  }
});

// URL load handler
loadUrlBtn.addEventListener('click', async () => {
  const url = urlInput.value.trim();
  if (!url) return;

  try {
    const response = await fetch(url);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const replay = await response.json() as Replay;
    loadReplay(replay);
  } catch (err) {
    alert('Failed to load replay from URL: ' + err);
  }
});

// Playback controls
playBtn.addEventListener('click', () => {
  viewer.togglePlay();
});

prevBtn.addEventListener('click', () => {
  viewer.setTurn(viewer.getTurn() - 1);
  updateUI();
  updateEventLog();
});

nextBtn.addEventListener('click', () => {
  viewer.setTurn(viewer.getTurn() + 1);
  updateUI();
  updateEventLog();
});

resetBtn.addEventListener('click', () => {
  viewer.pause();
  viewer.setTurn(0);
  updateUI();
  updateEventLog();
});

// Turn slider
turnSlider.addEventListener('input', () => {
  viewer.setTurn(parseInt(turnSlider.value, 10));
  updateUI();
  updateEventLog();
});

// Speed slider
speedSlider.addEventListener('input', () => {
  const speed = parseInt(speedSlider.value, 10);
  viewer.setSpeed(speed);
  speedDisplay.textContent = String(speed);
});

// Fog of war select
fogSelect.addEventListener('change', () => {
  const value = fogSelect.value;
  viewer.setFogOfWar(value === '' ? null : parseInt(value, 10));
});

// Cell size select
cellSizeSelect.addEventListener('change', () => {
  const size = parseInt(cellSizeSelect.value, 10);
  const replay = viewer.getReplay();
  if (replay) {
    viewer = new ReplayViewer(canvas, { cellSize: size });
    viewer.onTurnChange = () => { updateUI(); updateEventLog(); };
    viewer.onPlayStateChange = (playing) => { playBtn.textContent = playing ? 'Pause' : 'Play'; };
    loadReplay(replay);
  }
});

// Viewer callbacks
viewer.onTurnChange = () => {
  updateUI();
  updateEventLog();
};

viewer.onPlayStateChange = (playing) => {
  playBtn.textContent = playing ? 'Pause' : 'Play';
};

// Keyboard shortcuts
document.addEventListener('keydown', (e) => {
  if (!viewer.getReplay()) return;

  switch (e.code) {
    case 'Space':
      e.preventDefault();
      viewer.togglePlay();
      break;
    case 'ArrowLeft':
      e.preventDefault();
      viewer.setTurn(viewer.getTurn() - 1);
      updateUI();
      updateEventLog();
      break;
    case 'ArrowRight':
      e.preventDefault();
      viewer.setTurn(viewer.getTurn() + 1);
      updateUI();
      updateEventLog();
      break;
    case 'Home':
      e.preventDefault();
      viewer.setTurn(0);
      updateUI();
      updateEventLog();
      break;
    case 'End':
      e.preventDefault();
      viewer.setTurn(viewer.getTotalTurns() - 1);
      updateUI();
      updateEventLog();
      break;
  }
});

console.log('AI Code Battle Replay Viewer initialized');

// Auto-load demo replay
(async () => {
  try {
    const response = await fetch('/data/demo-replay-v2.json');
    if (response.ok) {
      const replay = await response.json() as Replay;
      loadReplay(replay);
      urlInput.value = '/data/demo-replay-v2.json';
    }
  } catch (e) {
    // silently fail - user can load manually
  }
})();
