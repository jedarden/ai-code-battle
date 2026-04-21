// Director Mode: adaptive auto-speed playback per §16.10
import type { Replay, ReplayTurn } from '../types';

// Speed multiplier to ms-per-turn mapping
// "x" means that many turns per second at the base rate of 2 turns/sec at 1x
export const SPEED_MULTIPLIERS = [1, 2, 4, 8, 16] as const;
export type SpeedMultiplier = typeof SPEED_MULTIPLIERS[number];

// Base rate: 1x = 500ms/turn (2 turns/sec)
const BASE_MS_PER_TURN = 500;

export function multiplierToMs(mult: SpeedMultiplier): number {
  return BASE_MS_PER_TURN / mult;
}

export function msToMultiplier(ms: number): SpeedMultiplier {
  const ratio = BASE_MS_PER_TURN / ms;
  let best: SpeedMultiplier = 16;
  for (const m of SPEED_MULTIPLIERS) {
    if (Math.abs(m - ratio) < Math.abs(best - ratio)) best = m;
  }
  return best;
}

// Target duration presets (in seconds)
export const DURATION_PRESETS = [30, 60, 120, 300] as const;
export type DurationPreset = typeof DURATION_PRESETS[number];

export const DURATION_LABELS: Record<DurationPreset, string> = {
  30: '30s',
  60: '1min',
  120: '2min',
  300: '5min',
};

export interface DirectorConfig {
  targetDuration: DurationPreset;
}

export const DEFAULT_DIRECTOR_CONFIG: DirectorConfig = {
  targetDuration: 60,
};

export function loadDirectorConfig(): DirectorConfig {
  try {
    const raw = localStorage.getItem('acb-director-config');
    if (raw) return { ...DEFAULT_DIRECTOR_CONFIG, ...JSON.parse(raw) };
  } catch {}
  return { ...DEFAULT_DIRECTOR_CONFIG };
}

export function saveDirectorConfig(config: DirectorConfig): void {
  try {
    localStorage.setItem('acb-director-config', JSON.stringify(config));
  } catch {}
}

// ── Action density computation ──────────────────────────────────────────────

export interface ActionDensity {
  density: number;
  deaths: number;
  captures: number;
  energyCollected: number;
  spawns: number;
  deltaWinProb: number;
}

/**
 * Compute action density for a single turn using the formula from §16.10:
 *   action_density(turn) = deaths × 3.0 + captures × 5.0 +
 *                          energy_collected × 0.5 + spawns × 1.0 +
 *                          abs(delta_win_prob) × 10.0
 */
export function computeActionDensity(
  turn: ReplayTurn,
  _prevTurn: ReplayTurn | null,
  winProb?: number[][],
  turnIndex?: number,
): ActionDensity {
  const events = turn.events ?? [];

  let deaths = 0;
  let captures = 0;
  let energyCollected = 0;
  let spawns = 0;

  for (const event of events) {
    switch (event.type) {
      case 'bot_died':
      case 'combat_death':
      case 'collision_death':
        deaths++;
        break;
      case 'core_captured':
      case 'core_destroyed':
        captures++;
        break;
      case 'energy_collected':
        energyCollected++;
        break;
      case 'bot_spawned':
        spawns++;
        break;
    }
  }

  let deltaWinProb = 0;
  if (winProb && turnIndex != null && turnIndex > 0) {
    const prev = winProb[turnIndex - 1];
    const curr = winProb[turnIndex];
    if (prev && curr) {
      // Sum absolute delta across all players
      for (let i = 0; i < prev.length; i++) {
        deltaWinProb += Math.abs((curr[i] ?? 0) - (prev[i] ?? 0));
      }
    }
  }

  const density =
    deaths * 3.0 +
    captures * 5.0 +
    energyCollected * 0.5 +
    spawns * 1.0 +
    deltaWinProb * 10.0;

  return { density, deaths, captures, energyCollected, spawns, deltaWinProb };
}

/**
 * Pre-compute action density for all turns in a replay.
 * Returns an array indexed by turn index (0-based).
 */
export function computeAllDensities(replay: Replay): ActionDensity[] {
  const turns = replay.turns;
  const winProb = replay.win_prob;
  const densities: ActionDensity[] = [];

  for (let i = 0; i < turns.length; i++) {
    const prev = i > 0 ? turns[i - 1] : null;
    densities.push(computeActionDensity(turns[i], prev, winProb, i));
  }

  return densities;
}

// ── Speed mapping ───────────────────────────────────────────────────────────

/**
 * Map action density to a speed multiplier per §16.10:
 *   0       → 16x (nothing happening)
 *   0.1–1.0 → 8x  (minor activity)
 *   1.0–3.0 → 4x  (moderate)
 *   3.0–5.0 → 2x  (significant)
 *   5.0+    → 1x  (critical)
 */
export function densityToSpeed(density: number): SpeedMultiplier {
  if (density === 0) return 16;
  if (density < 1.0) return 8;
  if (density < 3.0) return 4;
  if (density < 5.0) return 2;
  return 1;
}

/**
 * Compute a raw speed schedule (one multiplier per turn) from densities,
 * scaled so the total approximate playback time matches the target duration.
 */
export function computeSpeedSchedule(
  densities: ActionDensity[],
  targetDurationSec: number,
): SpeedMultiplier[] {
  const totalTurns = densities.length;
  if (totalTurns === 0) return [];

  // First pass: raw speeds from density
  const rawSpeeds = densities.map(d => densityToSpeed(d.density));

  // Compute raw total duration: sum of (base_ms / speed) for each turn
  let rawTotalMs = 0;
  for (const speed of rawSpeeds) {
    rawTotalMs += BASE_MS_PER_TURN / speed;
  }

  const targetMs = targetDurationSec * 1000;

  // Scale factor: if raw duration is 2x target, we need 2x all speeds
  const scaleFactor = rawTotalMs > 0 ? targetMs / rawTotalMs : 1;

  // Apply scale factor to speeds, clamping to valid multipliers
  const schedule: SpeedMultiplier[] = rawSpeeds.map(raw => {
    const scaledMs = (BASE_MS_PER_TURN / raw) * scaleFactor;
    // Find closest valid multiplier
    let best: SpeedMultiplier = 1;
    let bestDiff = Infinity;
    for (const m of SPEED_MULTIPLIERS) {
      const ms = BASE_MS_PER_TURN / m;
      const diff = Math.abs(ms - scaledMs);
      if (diff < bestDiff) {
        bestDiff = diff;
        best = m;
      }
    }
    return best;
  });

  return schedule;
}

// ── Eased speed transition ──────────────────────────────────────────────────

const EASE_DURATION_MS = 500;

export interface DirectorState {
  enabled: boolean;
  currentMultiplier: SpeedMultiplier;
  targetMultiplier: SpeedMultiplier;
  easedMsPerTurn: number;
  easeStartTime: number;
  easeStartMs: number;
  pauseReason: 'none' | 'scrubbing';
}

export function createDirectorState(): DirectorState {
  return {
    enabled: false,
    currentMultiplier: 16,
    targetMultiplier: 16,
    easedMsPerTurn: multiplierToMs(16),
    easeStartTime: 0,
    easeStartMs: multiplierToMs(16),
    pauseReason: 'none',
  };
}

/**
 * Update the eased speed for a given turn.
 * Called each render frame to compute the current ms/turn.
 */
export function tickDirectorSpeed(
  state: DirectorState,
  schedule: SpeedMultiplier[],
  turnIndex: number,
  now: number,
): number {
  if (!state.enabled || state.pauseReason !== 'none') {
    return state.easedMsPerTurn;
  }

  const target = schedule[turnIndex] ?? 16;

  if (target !== state.targetMultiplier) {
    // Start a new ease transition
    state.easeStartTime = now;
    state.easeStartMs = state.easedMsPerTurn;
    state.targetMultiplier = target;
    state.currentMultiplier = target;
  }

  // Compute eased value
  const elapsed = now - state.easeStartTime;
  const t = Math.min(1, elapsed / EASE_DURATION_MS);
  // Ease-in-out cubic
  const eased = t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;

  const targetMs = multiplierToMs(state.targetMultiplier);
  state.easedMsPerTurn = state.easeStartMs + (targetMs - state.easeStartMs) * eased;

  return state.easedMsPerTurn;
}

/**
 * Format the director speed indicator string.
 * e.g., "Director 8x → 2x" or "Director 4x"
 */
export function formatDirectorLabel(
  current: SpeedMultiplier,
  target: SpeedMultiplier,
  transitioning: boolean,
): string {
  if (transitioning && current !== target) {
    return `Director ${current}x → ${target}x`;
  }
  return `Director ${current}x`;
}
