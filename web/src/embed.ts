// Embeddable replay viewer - minimal, auto-playing widget
import { ReplayViewer } from './replay-viewer';
import type { Replay } from './types';
import { fetchCommentary } from './api-types';

// Player colors matching replay-viewer.ts
const PLAYER_COLORS = [
  '#3b82f6', // Blue (player 0)
  '#ef4444', // Red (player 1)
  '#22c55e', // Green (player 2)
  '#f59e0b', // Amber (player 3)
  '#8b5cf6', // Purple (player 4)
  '#06b6d4', // Cyan (player 5)
];

// Configuration
const R2_BASE = 'https://r2.aicodebattle.com';
const B2_BASE = 'https://b2.aicodebattle.com';
const PAGES_BASE = 'https://ai-code-battle.pages.dev';

interface EmbedConfig {
  matchId: string;
  autoPlay: boolean;
  speed: number;
  loop: boolean;
  viewMode: 'standard' | 'dots' | 'voronoi' | 'influence';
}

class EmbedViewer {
  private canvas: HTMLCanvasElement;
  private viewer: ReplayViewer | null = null;
  private replay: Replay | null = null;
  private config: EmbedConfig;

  // UI elements
  private playBtn: HTMLButtonElement;
  private resetBtn: HTMLButtonElement;
  private turnDisplay: HTMLElement;
  private progressBar: HTMLElement;
  private progressFill: HTMLElement;
  private speedSelect: HTMLSelectElement;
  private loadingOverlay: HTMLElement;
  private errorOverlay: HTMLElement;
  private errorMessage: HTMLElement;
  private retryBtn: HTMLElement;
  private endOverlay: HTMLElement;
  private endTitle: HTMLElement;
  private endSubtitle: HTMLElement;
  private scoreOverlay: HTMLElement;
  private commentaryBar: HTMLElement;
  private commentaryText: HTMLElement;

  constructor() {
    this.canvas = document.getElementById('replay-canvas') as HTMLCanvasElement;
    this.playBtn = document.getElementById('play-btn') as HTMLButtonElement;
    this.resetBtn = document.getElementById('reset-btn') as HTMLButtonElement;
    this.turnDisplay = document.getElementById('turn-display') as HTMLElement;
    this.progressBar = document.getElementById('progress-bar') as HTMLElement;
    this.progressFill = document.getElementById('progress-fill') as HTMLElement;
    this.speedSelect = document.getElementById('speed-select') as HTMLSelectElement;
    this.loadingOverlay = document.getElementById('loading-overlay') as HTMLElement;
    this.errorOverlay = document.getElementById('error-overlay') as HTMLElement;
    this.errorMessage = document.getElementById('error-message') as HTMLElement;
    this.retryBtn = document.getElementById('retry-btn') as HTMLElement;
    this.endOverlay = document.getElementById('end-overlay') as HTMLElement;
    this.endTitle = document.getElementById('end-title') as HTMLElement;
    this.endSubtitle = document.getElementById('end-subtitle') as HTMLElement;
    this.scoreOverlay = document.getElementById('score-overlay') as HTMLElement;
    this.commentaryBar = document.getElementById('commentary-bar') as HTMLElement;
    this.commentaryText = document.getElementById('commentary-text') as HTMLElement;

    // Parse config from URL
    this.config = this.parseConfig();

    this.init();
  }

  private parseConfig(): EmbedConfig {
    const path = window.location.pathname;
    const params = new URLSearchParams(window.location.search);

    // Extract match_id from path: /embed/{match_id}
    const matchIdMatch = path.match(/\/embed\/([^/]+)/);
    const matchId = matchIdMatch ? matchIdMatch[1] : params.get('match_id') || '';

    // Parse view mode - default to 'influence' (territory view) for homepage embeds
    const viewModeParam = params.get('view');
    const viewMode: 'standard' | 'dots' | 'voronoi' | 'influence' =
      viewModeParam === 'standard' || viewModeParam === 'dots' || viewModeParam === 'voronoi' || viewModeParam === 'influence'
        ? viewModeParam
        : 'influence'; // Default to territory view for homepage

    return {
      matchId,
      autoPlay: params.get('autoplay') !== 'false',
      speed: parseInt(params.get('speed') || '100', 10),
      loop: params.get('loop') === 'true',
      viewMode,
    };
  }

  private init(): void {
    // Wire up event handlers
    this.playBtn.addEventListener('click', () => this.togglePlay());
    this.resetBtn.addEventListener('click', () => this.reset());
    this.retryBtn.addEventListener('click', () => this.loadReplay());
    this.speedSelect.addEventListener('change', () => this.updateSpeed());
    this.progressBar.addEventListener('click', (e) => this.seekTo(e));

    // Keyboard controls
    document.addEventListener('keydown', (e) => this.handleKeydown(e));

    if (!this.config.matchId) {
      this.showError('No match ID specified');
      return;
    }

    this.loadReplay();
  }

  private async loadReplay(): Promise<void> {
    this.showLoading();
    this.hideError();
    this.hideEndOverlay();

    try {
      // Try R2 first (warm cache), fall back to B2 (cold archive)
      const replay = await this.fetchReplay(this.config.matchId);
      this.replay = replay;

      // Update page metadata
      this.updateMetadata(replay);

      // Initialize viewer
      this.viewer = new ReplayViewer(this.canvas, {
        cellSize: 10,
        animationSpeed: this.config.speed,
        viewMode: this.config.viewMode,
      });

      this.viewer.loadReplay(replay);

      // Wire viewer callbacks
      this.viewer.onTurnChange = (turn) => this.onTurnChange(turn);
      this.viewer.onPlayStateChange = (playing) => this.onPlayStateChange(playing);
      this.viewer.onCommentaryChange = (entry) => this.onCommentaryChange(entry);

      // Load AI commentary if available (non-blocking)
      this.loadCommentary(this.config.matchId);

      // Hide loading, enable controls
      this.hideLoading();
      this.enableControls();
      this.updateUI();

      // Auto-play if configured
      if (this.config.autoPlay) {
        this.viewer.play();
      }
    } catch (err) {
      console.error('Failed to load replay:', err);
      this.showError(err instanceof Error ? err.message : 'Failed to load replay');
    }
  }

  private async fetchReplay(matchId: string): Promise<Replay> {
    // Try R2 warm cache first
    const r2Url = `${R2_BASE}/replays/${matchId}.json.gz`;
    try {
      const response = await fetch(r2Url);
      if (response.ok) {
        // Note: For gzipped content, browser handles decompression automatically
        // if Content-Encoding: gzip is set, or we can use DecompressionStream
        const replay = await response.json();
        return replay as Replay;
      }
    } catch (e) {
      console.warn('R2 fetch failed, trying B2:', e);
    }

    // Fall back to B2 cold archive
    const b2Url = `${B2_BASE}/replays/${matchId}.json.gz`;
    const response = await fetch(b2Url);
    if (!response.ok) {
      throw new Error(`Replay not found: ${matchId}`);
    }
    const replay = await response.json();
    return replay as Replay;
  }

  private updateMetadata(replay: Replay): void {
    // Update page title
    const winnerName = replay.result.winner >= 0 && replay.result.winner < replay.players.length
      ? replay.players[replay.result.winner].name
      : 'Draw';
    document.title = `${winnerName} wins - AI Code Battle Replay`;

    // Update OG tags
    const ogUrl = document.querySelector('meta[property="og:url"]') as HTMLMetaElement;
    const ogTitle = document.querySelector('meta[property="og:title"]') as HTMLMetaElement;
    const ogDescription = document.querySelector('meta[property="og:description"]') as HTMLMetaElement;
    const twitterPlayer = document.querySelector('meta[name="twitter:player"]') as HTMLMetaElement;

    const embedUrl = `${PAGES_BASE}/embed/${replay.match_id}`;

    if (ogUrl) ogUrl.content = embedUrl;
    if (ogTitle) ogTitle.content = `${winnerName} wins - AI Code Battle`;
    if (ogDescription) {
      const players = replay.players.map(p => p.name).join(' vs ');
      ogDescription.content = `${players} - ${replay.result.turns} turns. Winner: ${winnerName}`;
    }
    if (twitterPlayer) twitterPlayer.content = embedUrl;
  }

  private togglePlay(): void {
    if (!this.viewer) return;
    this.viewer.togglePlay();
  }

  private reset(): void {
    if (!this.viewer) return;
    this.viewer.pause();
    this.viewer.setTurn(0);
    this.updateUI();
    this.hideEndOverlay();
  }

  private updateSpeed(): void {
    if (!this.viewer) return;
    const speed = parseInt(this.speedSelect.value, 10);
    this.viewer.setSpeed(speed);
    this.config.speed = speed;
  }

  private seekTo(e: MouseEvent): void {
    if (!this.viewer || !this.replay) return;
    const rect = this.progressBar.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const percent = x / rect.width;
    const turn = Math.floor(percent * this.viewer.getTotalTurns());
    this.viewer.setTurn(turn);
    this.updateUI();
  }

  private handleKeydown(e: KeyboardEvent): void {
    if (!this.viewer || !this.replay) return;

    switch (e.code) {
      case 'Space':
        e.preventDefault();
        this.viewer.togglePlay();
        break;
      case 'ArrowLeft':
        e.preventDefault();
        this.viewer.setTurn(this.viewer.getTurn() - 1);
        this.updateUI();
        break;
      case 'ArrowRight':
        e.preventDefault();
        this.viewer.setTurn(this.viewer.getTurn() + 1);
        this.updateUI();
        break;
      case 'Home':
        e.preventDefault();
        this.reset();
        break;
      case 'End':
        e.preventDefault();
        this.viewer.setTurn(this.viewer.getTotalTurns() - 1);
        this.updateUI();
        break;
    }
  }

  private onTurnChange(_turn: number): void {
    this.updateUI();

    // Check if at end
    if (this.viewer && this.viewer.isAtEnd()) {
      if (this.config.loop) {
        this.viewer.setTurn(0);
        this.viewer.play();
      } else {
        this.showEndOverlay();
      }
    }
  }

  private onPlayStateChange(playing: boolean): void {
    this.playBtn.textContent = playing ? 'Pause' : 'Play';
    if (!playing && this.viewer?.isAtEnd()) {
      this.showEndOverlay();
    }
  }

  private updateUI(): void {
    if (!this.viewer || !this.replay) return;

    const turn = this.viewer.getTurn();
    const total = this.viewer.getTotalTurns();

    this.turnDisplay.textContent = `${turn} / ${total}`;

    const percent = total > 0 ? (turn / (total - 1)) * 100 : 0;
    this.progressFill.style.width = `${percent}%`;

    this.playBtn.textContent = this.viewer.getIsPlaying() ? 'Pause' : 'Play';

    // Update score overlay
    this.updateScoreOverlay(turn);
  }

  private updateScoreOverlay(turn: number): void {
    if (!this.replay) return;

    const turnData = this.replay.turns[turn];
    if (!turnData) return;

    let html = '';
    this.replay.players.forEach((player, idx) => {
      const score = turnData.scores[idx] ?? 0;
      const energy = turnData.energy_held[idx] ?? 0;
      const color = PLAYER_COLORS[idx];

      html += `
        <div class="player-score">
          <div class="color-dot" style="background-color: ${color}"></div>
          <span>${player.name}: ${score} <small>(E:${energy})</small></span>
        </div>
      `;
    });

    this.scoreOverlay.innerHTML = html;
  }

  private showLoading(): void {
    this.loadingOverlay.style.display = 'flex';
  }

  private hideLoading(): void {
    this.loadingOverlay.style.display = 'none';
  }

  private showError(message: string): void {
    this.hideLoading();
    this.errorMessage.textContent = message;
    this.errorOverlay.style.display = 'flex';
  }

  private hideError(): void {
    this.errorOverlay.style.display = 'none';
  }

  private enableControls(): void {
    this.playBtn.disabled = false;
    this.resetBtn.disabled = false;
  }

  private showEndOverlay(): void {
    if (!this.replay) return;

    const winnerName = this.replay.result.winner >= 0 && this.replay.result.winner < this.replay.players.length
      ? this.replay.players[this.replay.result.winner].name
      : 'Draw';

    this.endTitle.textContent = this.replay.result.winner >= 0 ? `${winnerName} Wins!` : 'Draw';
    this.endSubtitle.textContent = `${this.replay.result.reason} after ${this.replay.result.turns} turns`;
    this.endOverlay.classList.add('visible');

    // Click to replay
    this.endOverlay.onclick = () => {
      this.reset();
      this.viewer?.play();
    };
  }

  private hideEndOverlay(): void {
    this.endOverlay.classList.remove('visible');
    this.endOverlay.onclick = null;
  }

  private async loadCommentary(matchId: string): Promise<void> {
    const commentary = await fetchCommentary(matchId);
    if (commentary && commentary.entries.length > 0) {
      this.viewer?.setCommentary(commentary);
      this.commentaryBar.classList.add('visible');
    }
  }

  private onCommentaryChange(entry: { turn: number; text: string; type: string } | null): void {
    if (!entry) {
      this.commentaryText.textContent = '';
      return;
    }
    this.commentaryText.textContent = entry.text;
    this.commentaryText.className = `commentary-text type-${entry.type}`;
  }
}

// Initialize on load
document.addEventListener('DOMContentLoaded', () => {
  new EmbedViewer();
});
