/**
 * Accessibility utilities for AI Code Battle
 * Implements plan §15.3: Accessibility Suite
 */

// Color palettes for player colors
export interface PlayerPalette {
  name: string;
  colors: string[];
}

export const DEFAULT_PALETTE: PlayerPalette = {
  name: 'default',
  colors: [
    '#2196F3', // Player 1: Blue
    '#F44336', // Player 2: Red
    '#4CAF50', // Player 3: Green
    '#FFEB3B', // Player 4: Yellow
    '#9C27B0', // Player 5: Purple
    '#009688', // Player 6: Teal
  ],
};

export const COLORBLIND_PALETTE: PlayerPalette = {
  name: 'tol',
  colors: [
    '#0077BB', // Player 1: Blue (Tol)
    '#EE7733', // Player 2: Orange (Tol)
    '#009988', // Player 3: Cyan (Tol)
    '#EE3377', // Player 4: Magenta (Tol)
    '#BBBBBB', // Player 5: Grey (Tol)
    '#000000', // Player 6: Black (Tol)
  ],
};

export const PALETTES: Record<string, PlayerPalette> = {
  default: DEFAULT_PALETTE,
  tol: COLORBLIND_PALETTE,
};

// Get palette from localStorage or default
export function getPalette(): PlayerPalette {
  const stored = localStorage.getItem('color-palette');
  if (stored && PALETTES[stored]) {
    return PALETTES[stored];
  }
  return DEFAULT_PALETTE;
}

// Set palette and persist to localStorage
export function setPalette(paletteName: string): void {
  if (PALETTES[paletteName]) {
    localStorage.setItem('color-palette', paletteName);
    applyPaletteStyles(PALETTES[paletteName]);
  }
}

// Apply palette colors as CSS custom properties
export function applyPaletteStyles(palette: PlayerPalette): void {
  const root = document.documentElement;
  palette.colors.forEach((color, i) => {
    root.style.setProperty(`--player-${i + 1}-color`, color);
  });
}

// High contrast mode
const HIGH_CONTRAST_KEY = 'high-contrast-mode';

export function isHighContrastMode(): boolean {
  return localStorage.getItem(HIGH_CONTRAST_KEY) === 'true';
}

export function setHighContrastMode(enabled: boolean): void {
  localStorage.setItem(HIGH_CONTRAST_KEY, enabled.toString());
  if (enabled) {
    document.body.classList.add('high-contrast');
  } else {
    document.body.classList.remove('high-contrast');
  }
}

export function toggleHighContrastMode(): void {
  setHighContrastMode(!isHighContrastMode());
}

// Reduced motion detection
export function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

// Keyboard shortcuts
export interface KeyboardShortcut {
  key: string;
  description: string;
  action: () => void;
}

export const KEYBOARD_SHORTCUTS: KeyboardShortcut[] = [
  { key: 'Space', description: 'Play / Pause', action: () => {} },
  { key: '← / →', description: 'Step back / forward one turn', action: () => {} },
  { key: 'Shift+← / Shift+→', description: 'Jump 10 turns', action: () => {} },
  { key: '[ / ]', description: 'Previous / Next critical moment', action: () => {} },
  { key: '1-5', description: 'Speed preset (1×, 2×, 4×, 8×, 16×)', action: () => {} },
  { key: 'V', description: 'Cycle view mode (dots → territory → influence)', action: () => {} },
  { key: 'F', description: 'Cycle fog of war perspective', action: () => {} },
  { key: 'T', description: 'Toggle debug telemetry panel', action: () => {} },
  { key: 'E', description: 'Toggle event timeline', action: () => {} },
  { key: 'C', description: 'Toggle commentary subtitles', action: () => {} },
  { key: 'H', description: 'Toggle high contrast mode', action: () => toggleHighContrastMode() },
  { key: '?', description: 'Show keyboard shortcuts overlay', action: () => {} },
];

// Register keyboard shortcuts with a callback function
export function registerKeyboardShortcuts(callbacks: Map<string, () => void>): () => void {
  const handleKeyDown = (e: KeyboardEvent) => {
    const key = e.key;
    const shift = e.shiftKey;
    const ctrl = e.ctrlKey;
    const meta = e.metaKey;

    // Don't trigger if typing in an input
    if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
      return;
    }

    let shortcutKey = key;
    if (shift) shortcutKey = 'Shift+' + key;
    if (ctrl) shortcutKey = 'Ctrl+' + key;
    if (meta) shortcutKey = 'Meta+' + key;

    const callback = callbacks.get(shortcutKey);
    if (callback) {
      e.preventDefault();
      callback();
    }
  };

  document.addEventListener('keydown', handleKeyDown);
  return () => document.removeEventListener('keydown', handleKeyDown);
}

// ARIA live region helpers
export function announceToScreenReader(message: string, priority: 'polite' | 'assertive' = 'polite'): void {
  const region = document.createElement('div');
  region.setAttribute('aria-live', priority);
  region.setAttribute('aria-atomic', 'true');
  region.className = 'sr-only';
  region.textContent = message;

  document.body.appendChild(region);

  // Remove after announcement
  setTimeout(() => {
    document.body.removeChild(region);
  }, 1000);
}

// Create ARIA live region element
export function createLiveRegion(id: string, priority: 'polite' | 'assertive' = 'polite'): HTMLElement {
  const region = document.createElement('div');
  region.id = id;
  region.setAttribute('aria-live', priority);
  region.setAttribute('aria-atomic', 'true');
  region.className = 'sr-only';
  return region;
}

// Focus management
export function setFocus(element: HTMLElement): void {
  element.focus();
}

export function makeFocusable(element: HTMLElement): void {
  if (!element.getAttribute('tabindex')) {
    element.setAttribute('tabindex', '0');
  }
}

export function addFocusIndicator(element: HTMLElement): void {
  element.style.outline = '2px solid #2196F3';
  element.style.outlineOffset = '2px';
}

// Skip-to-content link
export function createSkipLink(targetId: string, label: string = 'Skip to content'): HTMLElement {
  const link = document.createElement('a');
  link.href = `#${targetId}`;
  link.textContent = label;
  link.className = 'skip-link';
  link.setAttribute('aria-label', label);

  // Add styles for skip link
  const style = document.createElement('style');
  style.textContent = `
    .skip-link {
      position: absolute;
      top: -40px;
      left: 0;
      background: #2196F3;
      color: white;
      padding: 8px;
      text-decoration: none;
      z-index: 10000;
      border-radius: 0 0 4px 0;
    }
    .skip-link:focus {
      top: 0;
    }
  `;
  document.head.appendChild(style);

  return link;
}

// Screen reader transcript generator
export interface TurnEvent {
  type: string;
  player?: number;
  botName?: string;
  position?: [number, number];
  count?: number;
}

export interface TurnSummary {
  turn: number;
  playerMoves: Map<number, number>; // player -> move count
  combatEvents: TurnEvent[];
  energyCollected: TurnEvent[];
  spawns: TurnEvent[];
  captures: TurnEvent[];
  winProb: number[];
}

export function generateTurnSummary(summary: TurnSummary): string {
  const parts: string[] = [];

  // Player movements
  const moveDescriptions: string[] = [];
  summary.playerMoves.forEach((count, player) => {
    moveDescriptions.push(`Player ${player + 1} moved ${count} bot${count !== 1 ? 's' : ''}`);
  });
  if (moveDescriptions.length > 0) {
    parts.push(moveDescriptions.join('. '));
  }

  // Combat events
  if (summary.combatEvents.length > 0) {
    const combatDesc = summary.combatEvents.map(e => {
      const loc = e.position ? `at (${e.position[0]},${e.position[1]})` : '';
      const count = e.count || 1;
      return `${count} ${e.botName || 'bot'} unit${count !== 1 ? 's' : ''} killed ${loc}`;
    }).join(', ');
    parts.push(`Combat: ${combatDesc}`);
  }

  // Energy collection
  if (summary.energyCollected.length > 0) {
    const energyDesc = summary.energyCollected.map(e => {
      const loc = e.position ? `at (${e.position[0]},${e.position[1]})` : '';
      return `energy ${loc}`;
    }).join(', ');
    parts.push(`Collected ${energyDesc}`);
  }

  // Spawns
  if (summary.spawns.length > 0) {
    const spawnCount = summary.spawns.reduce((sum, e) => sum + (e.count || 1), 0);
    parts.push(`${spawnCount} bot${spawnCount !== 1 ? 's' : ''} spawned`);
  }

  // Core captures
  if (summary.captures.length > 0) {
    const captureDesc = summary.captures.map(e => {
      const player = e.player !== undefined ? `Player ${e.player + 1}` : 'A player';
      return `${player}'s core captured`;
    }).join(', ');
    parts.push(captureDesc);
  }

  // Win probability
  if (summary.winProb.length >= 2) {
    const probDesc = summary.winProb.map((p, i) => `Player ${i + 1}: ${Math.round(p * 100)}%`).join(', ');
    parts.push(`Win probability: ${probDesc}`);
  }

  return `Turn ${summary.turn}: ${parts.join('. ')}.`;
}

// Generate full transcript for screen reader
export function generateTranscript(turns: TurnSummary[]): string[] {
  return turns.map(generateTurnSummary);
}

// Initialize accessibility features
export function initAccessibility(): void {
  // Apply saved palette
  const palette = getPalette();
  applyPaletteStyles(palette);

  // Apply high contrast mode if saved
  if (isHighContrastMode()) {
    document.body.classList.add('high-contrast');
  }

  // Add skip-to-content link
  const skipLink = createSkipLink('main-content');
  document.body.insertBefore(skipLink, document.body.firstChild);

  // Add screen reader only styles
  const srStyle = document.createElement('style');
  srStyle.textContent = `
    .sr-only {
      position: absolute;
      width: 1px;
      height: 1px;
      padding: 0;
      margin: -1px;
      overflow: hidden;
      clip: rect(0, 0, 0, 0);
      white-space: nowrap;
      border-width: 0;
    }
  `;
  document.head.appendChild(srStyle);
}

// Shape per player (redundant encoding for color blindness)
export const PLAYER_SHAPES = [
  'circle',      // Player 1
  'square',      // Player 2
  'triangle',    // Player 3
  'diamond',     // Player 4
  'pentagon',    // Player 5
  'hexagon',     // Player 6
] as const;

export type PlayerShape = typeof PLAYER_SHAPES[number];

export function getPlayerShape(playerIndex: number): PlayerShape {
  return PLAYER_SHAPES[playerIndex % PLAYER_SHAPES.length];
}
