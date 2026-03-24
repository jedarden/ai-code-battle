/**
 * SwarmBot strategy: formation-based combat with tight cohesion.
 *
 * Strategy: Keep units in tight formations, advance as a group toward enemies.
 * - All bots maintain cohesion — no bot moves if it would be >3 tiles from the
 *   nearest friendly bot
 * - The swarm moves as a unit toward the nearest enemy presence
 * - BFS-based center-of-mass steering
 * - Energy collection is incidental (pass over it during advance)
 * - New spawns rally to the swarm before advancing
 */

import {
  GameState,
  VisibleBot,
  Position,
  Move,
  Direction,
  GameConfig,
  posKey,
  posEquals,
  moveToward,
  distance2,
  manhattanDistance,
  ALL_DIRECTIONS,
  buildPositionSet,
} from './game.js';

const COHESION_RADIUS = 3; // Maximum distance from nearest friendly
const COHESION_RADIUS2 = COHESION_RADIUS * COHESION_RADIUS;

export class SwarmStrategy {
  /**
   * Compute moves for all owned bots
   */
  computeMoves(state: GameState): Move[] {
    const myId = state.you.id;
    const config = state.config;

    // Separate my bots from enemies
    const myBots: VisibleBot[] = [];
    const enemyBots: VisibleBot[] = [];
    for (const bot of state.bots) {
      if (bot.owner === myId) {
        myBots.push(bot);
      } else {
        enemyBots.push(bot);
      }
    }

    if (myBots.length === 0) {
      return [];
    }

    // Build wall lookup
    const walls = buildPositionSet(state.walls);

    // Build enemy position lookup
    const enemyPositions = new Map<string, VisibleBot>();
    for (const bot of enemyBots) {
      enemyPositions.set(posKey(bot.position), bot);
    }

    // Calculate swarm center (center of mass of my bots)
    const swarmCenter = this.calculateCenter(myBots.map(b => b.position), config);

    // Calculate enemy center if any enemies visible
    const enemyCenter = enemyBots.length > 0
      ? this.calculateCenter(enemyBots.map(b => b.position), config)
      : null;

    // My bot positions for cohesion checks
    const myBotPositions = new Set(myBots.map(b => posKey(b.position)));

    const moves: Move[] = [];

    for (const bot of myBots) {
      const move = this.computeBotMove(
        bot,
        myBotPositions,
        enemyPositions,
        swarmCenter,
        enemyCenter,
        walls,
        config
      );
      if (move) {
        moves.push(move);
      }
    }

    return moves;
  }

  /**
   * Calculate center of mass of positions
   */
  private calculateCenter(positions: Position[], config: GameConfig): Position {
    if (positions.length === 0) {
      return { row: config.rows / 2, col: config.cols / 2 };
    }

    // Use circular mean for toroidal coordinates
    let sumSinRow = 0, sumCosRow = 0;
    let sumSinCol = 0, sumCosCol = 0;

    const rowScale = (2 * Math.PI) / config.rows;
    const colScale = (2 * Math.PI) / config.cols;

    for (const pos of positions) {
      sumSinRow += Math.sin(pos.row * rowScale);
      sumCosRow += Math.cos(pos.row * rowScale);
      sumSinCol += Math.sin(pos.col * colScale);
      sumCosCol += Math.cos(pos.col * colScale);
    }

    const avgRow = Math.atan2(sumSinRow / positions.length, sumCosRow / positions.length) / rowScale;
    const avgCol = Math.atan2(sumSinCol / positions.length, sumCosCol / positions.length) / colScale;

    return {
      row: ((avgRow % config.rows) + config.rows) % config.rows,
      col: ((avgCol % config.cols) + config.cols) % config.cols,
    };
  }

  /**
   * Compute move for a single bot
   */
  private computeBotMove(
    bot: VisibleBot,
    myBotPositions: Set<string>,
    enemyPositions: Map<string, VisibleBot>,
    swarmCenter: Position,
    enemyCenter: Position | null,
    walls: Set<string>,
    config: GameConfig
  ): Move | null {
    const rows = config.rows;
    const cols = config.cols;

    // Find direction that maintains cohesion while advancing toward enemy
    let bestDir: Direction | null = null;
    let bestScore = -Infinity;

    // Target is enemy center if visible, otherwise explore
    const target = enemyCenter ?? { row: rows / 2, col: cols / 2 };

    for (const dir of ALL_DIRECTIONS) {
      const newPos = moveToward(bot.position, dir, rows, cols);
      const newPosKey = posKey(newPos);

      // Can't move into walls or enemies
      if (walls.has(newPosKey) || enemyPositions.has(newPosKey)) {
        continue;
      }

      // Check cohesion: must stay within COHESION_RADIUS of at least one friendly bot
      if (!this.maintainsCohesion(newPos, bot.position, myBotPositions, rows, cols)) {
        continue;
      }

      // Score this move
      let score = 0;

      // Prefer moving toward enemy center (or target)
      const distToTarget = distance2(newPos, target, rows, cols);
      const currentDistToTarget = distance2(bot.position, target, rows, cols);
      score += (currentDistToTarget - distToTarget) * 10; // Reward getting closer

      // Prefer staying near swarm center
      const distToSwarmCenter = distance2(newPos, swarmCenter, rows, cols);
      score -= distToSwarmCenter * 0.5; // Penalize being far from swarm

      // Bonus for moving toward nearby enemies (engagement)
      let nearestEnemyDist = Infinity;
      for (const enemy of enemyPositions.values()) {
        const dist = distance2(newPos, enemy.position, rows, cols);
        nearestEnemyDist = Math.min(nearestEnemyDist, dist);
      }
      if (nearestEnemyDist < Infinity) {
        // Bonus for being in attack range
        if (nearestEnemyDist <= config.attack_radius2) {
          score += 50;
        }
      }

      if (score > bestScore) {
        bestScore = score;
        bestDir = dir;
      }
    }

    if (bestDir) {
      return { position: bot.position, direction: bestDir };
    }

    // If no good move found, try to stay put or move toward swarm
    return null;
  }

  /**
   * Check if moving to newPos maintains cohesion with friendly bots
   */
  private maintainsCohesion(
    newPos: Position,
    oldPos: Position,
    myBotPositions: Set<string>,
    rows: number,
    cols: number
  ): boolean {
    // Temporarily remove old position and add new position
    const oldKey = posKey(oldPos);

    for (const botPosKey of myBotPositions) {
      if (botPosKey === oldKey) continue;

      const [row, col] = botPosKey.split(',').map(Number);
      const botPos = { row, col };

      const dist2 = distance2(newPos, botPos, rows, cols);
      if (dist2 <= COHESION_RADIUS2) {
        return true;
      }
    }

    return false;
  }
}
