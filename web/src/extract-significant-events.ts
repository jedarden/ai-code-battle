// Significant Event Extraction for Replay Event Ribbon
// Client-side analysis of replay data to surface key moments

import type { Replay, ReplayTurn, GameEvent, Position } from './types';

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

export type SignificantEventType =
  | 'combat'          // ⚔️ Bot damage/knockout events
  | 'core_capture'    // 🏰 Core ownership changes
  | 'energy_milestone' // 💎 Energy thresholds crossed (50, 100, 150)
  | 'mass_death'      // 💀 ≥3 bots dying within 3 turns
  | 'momentum_shift'   // 📈 Score lead changes
  | 'critical_moment'  // 🌟 Game state phase changes
  | 'spawn_wave'       // 🐣 New bot spawns;

export interface SignificantEvent {
  type: SignificantEventType;
  turn: number;
  description: string;
  botId?: number;
  playerId?: number;
  position?: Position;
  emoji?: string; // Visual marker for the event ribbon
}

// Event detection configuration
interface EventDetectionConfig {
  energyMilestones: number[];  // Energy thresholds to detect (default: [50, 100, 150])
  massDeathThreshold: number;   // Minimum deaths for "mass death" event (default: 3)
  massDeathWindow: number;      // Turn window to check for mass deaths (default: 3)
  trackLeadChanges: boolean;    // Whether to track score lead changes (default: true)
}

const DEFAULT_CONFIG: EventDetectionConfig = {
  energyMilestones: [50, 100, 150],
  massDeathThreshold: 3,
  massDeathWindow: 3,
  trackLeadChanges: true,
};

// ─────────────────────────────────────────────────────────────────────────────
// Main Extraction Function
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Extract significant events from replay data for the event ribbon.
 * Returns a structured array of events sorted by turn number.
 */
export function extractSignificantEvents(
  replay: Replay,
  config: Partial<EventDetectionConfig> = {}
): SignificantEvent[] {
  const fullConfig = { ...DEFAULT_CONFIG, ...config };
  const events: SignificantEvent[] = [];

  if (!replay || !replay.turns || replay.turns.length === 0) {
    return [];
  }

  // Track state across turns
  let lastLeader: number | null = null;
  const deathWindow: number[] = []; // Rolling window of death counts

  for (let i = 0; i < replay.turns.length; i++) {
    const turn = replay.turns[i];
    if (!turn) continue;

    // Track deaths for mass death detection
    const deathsThisTurn = countDeathsInTurn(turn);
    deathWindow.push(deathsThisTurn);
    if (deathWindow.length > fullConfig.massDeathWindow) {
      deathWindow.shift();
    }

    // Detect mass death (≥3 deaths in 3 turns)
    const deathsInWindow = deathWindow.reduce((sum, count) => sum + count, 0);
    if (deathsInWindow >= fullConfig.massDeathThreshold) {
      // Find the turn in this window with the most deaths
      const maxDeathsInWindow = Math.max(...deathWindow);
      const windowStart = Math.max(0, i - fullConfig.massDeathWindow + 1);
      const peakTurnIndex = deathWindow.findIndex((count, idx) =>
        count === maxDeathsInWindow && (windowStart + idx) <= i
      );

      // Only add event if we haven't already for this death spike
      const peakTurn = windowStart + (peakTurnIndex >= 0 ? peakTurnIndex : 0);
      if (!events.some(e => e.type === 'mass_death' && e.turn === peakTurn)) {
        events.push(createMassDeathEvent(peakTurn, maxDeathsInWindow, replay));
      }
    }

    // Process turn events
    if (turn.events && turn.events.length > 0) {
      for (const event of turn.events) {
        const significantEvents = processGameEvent(event, turn, replay, fullConfig);
        events.push(...significantEvents);
      }
    }

    // Track lead changes
    if (fullConfig.trackLeadChanges) {
      const currentLeader = getCurrentLeader(turn, replay.players.length);
      if (currentLeader !== null && lastLeader !== null && currentLeader !== lastLeader) {
        events.push(createMomentumShiftEvent(turn.turn, currentLeader, lastLeader, replay));
      }
      lastLeader = currentLeader;
    }

    // Check for energy milestones
    if (turn.energy_held) {
      for (const milestone of fullConfig.energyMilestones) {
        for (let playerId = 0; playerId < turn.energy_held.length; playerId++) {
          const energy = turn.energy_held[playerId];
          if (energy >= milestone) {
            // Check if we haven't already marked this milestone for this player
            const alreadyMarked = events.some(e =>
              e.type === 'energy_milestone' &&
              e.turn === turn.turn &&
              e.playerId === playerId
            );
            if (!alreadyMarked) {
              events.push(createEnergyMilestoneEvent(turn.turn, milestone, playerId, replay));
            }
          }
        }
      }
    }
  }

  // Add critical moment for game end
  if (replay.result && replay.turns.length > 0) {
    const finalTurn = replay.turns.length - 1;
    events.push(createGameEndEvent(finalTurn, replay.result, replay));
  }

  // Sort events by turn
  events.sort((a, b) => a.turn - b.turn);

  return events;
}

// ─────────────────────────────────────────────────────────────────────────────
// Event Detection Helpers
// ─────────────────────────────────────────────────────────────────────────────

function countDeathsInTurn(turn: ReplayTurn): number {
  if (!turn.events) return 0;
  return turn.events.filter(e =>
    e.type === 'bot_died' ||
    e.type === 'combat_death' ||
    e.type === 'zone_death'
  ).length;
}

function getCurrentLeader(turn: ReplayTurn, numPlayers: number): number | null {
  if (!turn.scores || turn.scores.length < 2) return null;

  let maxScore = -1;
  let leader = null;

  for (let i = 0; i < Math.min(numPlayers, turn.scores.length); i++) {
    if (turn.scores[i] > maxScore) {
      maxScore = turn.scores[i];
      leader = i;
    }
  }

  return leader;
}

function processGameEvent(
  event: GameEvent,
  turn: ReplayTurn,
  replay: Replay,
  _config: EventDetectionConfig
): SignificantEvent[] {
  const events: SignificantEvent[] = [];
  const details = event.details as Record<string, unknown>;

  switch (event.type) {
    case 'bot_died':
    case 'combat_death':
      events.push(createCombatEvent(turn.turn, event, details, replay));
      break;

    case 'zone_death':
      events.push(createZoneDeathEvent(turn.turn, details, replay));
      break;

    case 'core_captured':
      events.push(createCoreCaptureEvent(turn.turn, details, replay));
      break;

    case 'core_destroyed':
      events.push(createCoreDestroyedEvent(turn.turn, details, replay));
      break;

    case 'bot_spawned':
      events.push(createSpawnWaveEvent(turn.turn, details, replay));
      break;

    case 'energy_collected':
      // Energy collection is frequent, only mark if it's significant
      // (e.g., multiple energy nodes collected in one turn)
      if (turn.events && turn.events.filter(e => e.type === 'energy_collected').length >= 3) {
        // Only add one event per turn for energy collection
        if (!events.some(e => e.type === 'energy_milestone' && e.turn === turn.turn)) {
          events.push(createEnergyCollectionEvent(turn.turn, details, replay));
        }
      }
      break;
  }

  return events;
}

// ─────────────────────────────────────────────────────────────────────────────
// Event Creation Functions
// ─────────────────────────────────────────────────────────────────────────────

function createCombatEvent(
  turn: number,
  _event: GameEvent,
  details: Record<string, unknown>,
  replay: Replay
): SignificantEvent {
  const botId = details.bot_id as number | undefined;
  const owner = details.owner as number | undefined;
  const position = details.position as Position | undefined;
  const killers = details.killers as Array<{ bot_id: number; owner: number; position: Position }> | undefined;

  const playerName = owner !== undefined ? getPlayerName(owner, replay) : 'Unknown';

  let description = `${playerName}'s bot destroyed`;
  if (killers && killers.length > 0) {
    const killerNames = killers.map(k => getPlayerName(k.owner, replay));
    description += ` by ${killerNames.join(' & ')}`;
  } else {
    description += ' in combat';
  }

  return {
    type: 'combat',
    turn,
    description,
    botId,
    playerId: owner,
    position,
    emoji: '⚔️',
  };
}

function createZoneDeathEvent(
  turn: number,
  details: Record<string, unknown>,
  replay: Replay
): SignificantEvent {
  const botId = details.bot_id as number | undefined;
  const owner = details.owner as number | undefined;
  const position = details.position as Position | undefined;
  const playerName = owner !== undefined ? getPlayerName(owner, replay) : 'Unknown';

  return {
    type: 'combat', // Zone deaths are a form of combat
    turn,
    description: `${playerName}'s bot eliminated by zone`,
    botId,
    playerId: owner,
    position,
    emoji: '⚔️',
  };
}

function createCoreCaptureEvent(
  turn: number,
  details: Record<string, unknown>,
  replay: Replay
): SignificantEvent {
  const newOwner = details.new_owner as number | undefined;
  const oldOwner = details.old_owner as number | undefined;
  const position = details.position as Position | undefined;

  const capturerName = newOwner !== undefined ? getPlayerName(newOwner, replay) : 'Unknown';
  const previousOwnerName = oldOwner !== undefined ? getPlayerName(oldOwner, replay) : 'Unknown';

  return {
    type: 'core_capture',
    turn,
    description: `${capturerName} captured ${previousOwnerName}'s core`,
    playerId: newOwner,
    position,
    emoji: '🏰',
  };
}

function createCoreDestroyedEvent(
  turn: number,
  details: Record<string, unknown>,
  _replay: Replay
): SignificantEvent {
  const position = details.position as Position | undefined;

  return {
    type: 'core_capture',
    turn,
    description: 'Core destroyed',
    position,
    emoji: '💥',
  };
}

function createMassDeathEvent(
  turn: number,
  deathCount: number,
  _replay: Replay
): SignificantEvent {
  return {
    type: 'mass_death',
    turn,
    description: `${deathCount} bots eliminated`,
    emoji: '💀',
  };
}

function createMomentumShiftEvent(
  turn: number,
  newLeader: number,
  oldLeader: number,
  replay: Replay
): SignificantEvent {
  const newLeaderName = getPlayerName(newLeader, replay);
  const oldLeaderName = getPlayerName(oldLeader, replay);

  return {
    type: 'momentum_shift',
    turn,
    description: `${newLeaderName} takes lead from ${oldLeaderName}`,
    playerId: newLeader,
    emoji: '📈',
  };
}

function createEnergyMilestoneEvent(
  turn: number,
  milestone: number,
  playerId: number,
  replay: Replay
): SignificantEvent {
  const playerName = getPlayerName(playerId, replay);

  return {
    type: 'energy_milestone',
    turn,
    description: `${playerName} reaches ${milestone} energy`,
    playerId,
    emoji: '💎',
  };
}

function createEnergyCollectionEvent(
  turn: number,
  details: Record<string, unknown>,
  replay: Replay
): SignificantEvent {
  const owner = details.owner as number | undefined;
  const playerName = owner !== undefined ? getPlayerName(owner, replay) : 'Unknown';

  return {
    type: 'energy_milestone',
    turn,
    description: `${playerName} collects energy surge`,
    playerId: owner,
    emoji: '💎',
  };
}

function createSpawnWaveEvent(
  turn: number,
  details: Record<string, unknown>,
  replay: Replay
): SignificantEvent {
  const botId = details.bot_id as number | undefined;
  const owner = details.owner as number | undefined;
  const position = details.position as Position | undefined;
  const playerName = owner !== undefined ? getPlayerName(owner, replay) : 'Unknown';

  return {
    type: 'spawn_wave',
    turn,
    description: `${playerName} spawns new bot`,
    botId,
    playerId: owner,
    position,
    emoji: '🐣',
  };
}

function createGameEndEvent(
  turn: number,
  result: { winner: number; reason: string },
  replay: Replay
): SignificantEvent {
  const winnerName = result.winner >= 0 ? getPlayerName(result.winner, replay) : 'Unknown';

  return {
    type: 'critical_moment',
    turn,
    description: `Game ends: ${winnerName} wins by ${result.reason}`,
    playerId: result.winner >= 0 ? result.winner : undefined,
    emoji: '🏆',
  };
}

// ─────────────────────────────────────────────────────────────────────────────
// Utility Functions
// ─────────────────────────────────────────────────────────────────────────────

function getPlayerName(playerId: number, replay: Replay): string {
  if (playerId >= 0 && playerId < replay.players.length) {
    return replay.players[playerId].name;
  }
  return `Player ${playerId}`;
}

/**
 * Get significant events for a specific turn range.
 * Useful for filtering the event ribbon to show only relevant events.
 */
export function getEventsInRange(
  events: SignificantEvent[],
  startTurn: number,
  endTurn: number
): SignificantEvent[] {
  return events.filter(e => e.turn >= startTurn && e.turn <= endTurn);
}

/**
 * Group events by turn for efficient rendering in the event ribbon.
 */
export function groupEventsByTurn(events: SignificantEvent[]): Map<number, SignificantEvent[]> {
  const grouped = new Map<number, SignificantEvent[]>();

  for (const event of events) {
    if (!grouped.has(event.turn)) {
      grouped.set(event.turn, []);
    }
    grouped.get(event.turn)!.push(event);
  }

  return grouped;
}

/**
 * Get a summary of event types and their counts.
 * Useful for showing event statistics or filters.
 */
export function getEventSummary(events: SignificantEvent[]): Record<SignificantEventType, number> {
  const summary: Record<string, number> = {
    combat: 0,
    core_capture: 0,
    energy_milestone: 0,
    mass_death: 0,
    momentum_shift: 0,
    critical_moment: 0,
    spawn_wave: 0,
  };

  for (const event of events) {
    summary[event.type] = (summary[event.type] || 0) + 1;
  }

  return summary as Record<SignificantEventType, number>;
}
