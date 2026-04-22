import { ReplayViewer } from './replay-viewer';
import type { Replay, TranscriptEntry } from './types';

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

// Transcript panel elements (§15.3)
const transcriptPanel = document.getElementById('transcript-panel') as HTMLDivElement;
const transcriptToggleBtn = document.getElementById('transcript-toggle') as HTMLButtonElement;
const transcriptCloseBtn = document.getElementById('transcript-close') as HTMLButtonElement;
const transcriptEntriesDiv = document.getElementById('transcript-entries') as HTMLDivElement;
const transcriptViewMode = document.getElementById('transcript-view-mode') as HTMLSelectElement;

// Transcript state
let transcriptEntries: TranscriptEntry[] = [];
let transcriptViewModeValue: 'all' | 'window' | 'recent' = 'all';

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
  renderTranscript(); // Generate and render transcript (§15.3)
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
    viewer.onTurnChange = () => { updateUI(); updateEventLog(); updateTranscriptHighlight(); };
    viewer.onPlayStateChange = (playing) => { playBtn.textContent = playing ? 'Pause' : 'Play'; };
    loadReplay(replay);
  }
});

// Viewer callbacks
viewer.onTurnChange = () => {
  updateUI();
  updateEventLog();
  updateTranscriptHighlight();
};

viewer.onPlayStateChange = (playing) => {
  playBtn.textContent = playing ? 'Pause' : 'Play';
};

// ── Transcript Panel Functions (§15.3) ────────────────────────────────────────

function toggleTranscriptPanel(): void {
  transcriptPanel.classList.toggle('open');
  // Update button visibility based on panel state
  transcriptToggleBtn.style.display = transcriptPanel.classList.contains('open') ? 'none' : 'block';
}

function closeTranscriptPanel(): void {
  transcriptPanel.classList.remove('open');
  transcriptToggleBtn.style.display = 'block';
}

function renderTranscript(): void {
  if (!viewer.getReplay()) {
    transcriptEntriesDiv.innerHTML = '<p style="color: #64748b; text-align: center; padding: 20px;">Load a replay to view the transcript.</p>';
    return;
  }

  // Generate transcript from viewer
  transcriptEntries = viewer.generateTranscript();

  // Filter entries based on view mode
  const filteredEntries = filterTranscriptEntries(transcriptEntries);

  if (filteredEntries.length === 0) {
    transcriptEntriesDiv.innerHTML = '<p style="color: #64748b; text-align: center; padding: 20px;">No transcript entries available.</p>';
    return;
  }

  // Render entries
  transcriptEntriesDiv.innerHTML = filteredEntries.map(entry => {
    const isCurrent = entry.turn === viewer.getTurn();
    return `
      <div class="transcript-entry${isCurrent ? ' current' : ''}" data-turn="${entry.turn}">
        <div class="turn-number">Turn ${entry.turn}</div>
        <div class="text">${entry.text}</div>
      </div>
    `;
  }).join('');

  // Add click handlers for jump-to-turn
  transcriptEntriesDiv.querySelectorAll('.transcript-entry').forEach(el => {
    el.addEventListener('click', () => {
      const turn = parseInt(el.getAttribute('data-turn') || '0', 10);
      viewer.setTurn(turn);
      updateUI();
      updateEventLog();
      updateTranscriptHighlight();
    });
  });

  updateTranscriptHighlight();
}

function filterTranscriptEntries(entries: TranscriptEntry[]): TranscriptEntry[] {
  const currentTurn = viewer.getTurn();
  const totalTurns = viewer.getTotalTurns();

  switch (transcriptViewModeValue) {
    case 'window':
      // Show ±10 turns from current turn
      return entries.filter(e => e.turn >= currentTurn - 10 && e.turn <= currentTurn + 10);

    case 'recent':
      // Show last 20 turns
      return entries.filter(e => e.turn >= totalTurns - 20);

    case 'all':
    default:
      // Show all turns
      return entries;
  }
}

function updateTranscriptHighlight(): void {
  // Update current turn highlighting in transcript
  const currentTurn = viewer.getTurn();
  transcriptEntriesDiv.querySelectorAll('.transcript-entry').forEach(el => {
    const turn = parseInt(el.getAttribute('data-turn') || '-1', 10);
    if (turn === currentTurn) {
      el.classList.add('current');
      // Scroll the current entry into view if panel is open
      if (transcriptPanel.classList.contains('open')) {
        el.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
    } else {
      el.classList.remove('current');
    }
  });
}

// Transcript panel event listeners
transcriptToggleBtn.addEventListener('click', toggleTranscriptPanel);
transcriptCloseBtn.addEventListener('click', closeTranscriptPanel);

transcriptViewMode.addEventListener('change', () => {
  const value = transcriptViewMode.value;
  if (value === 'all' || value === 'window' || value === 'recent') {
    transcriptViewModeValue = value;
    renderTranscript();
  }
});

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
    case 'KeyT':
      // Toggle transcript panel (§15.3)
      e.preventDefault();
      toggleTranscriptPanel();
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
