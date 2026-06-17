/**
 * Coordinator strategy - dynamic role allocation for multi-role bot coordination.
 *
 * Strategy: Each turn, assign each bot exactly one role (Attacker/Harvester/Defender/Scout)
 * and execute role-appropriate logic. Role allocation adapts to game state:
 * - High threat → more defenders
 * - Low energy → more harvesters
 * - Winning → more attackers
 */

import {
  GameState,
  VisibleBot,
  VisibleCore,
  Position,
  Move,
  Direction,
  GameConfig,
  PlayerInfo,
  posKey,
  posEquals,
  moveToward,
  distance2,
  manhattanDistance,
  ALL_DIRECTIONS,
  buildPositionSet,
} from './game.js';

export type BotRole = 'ATTACKER' | 'HARVESTER' | 'DEFENDER' | 'SCOUT';

interface RoleAssignment {
  bot: VisibleBot;
  role: BotRole;
  target: Position | null;
}

/**
 * Coordinator strategy implementation
 */
export class CoordinatorStrategy {
  // Track last seen positions for scouting
  private seenPositions = new Set<string>();

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

    // Build lookups
    const walls = buildPositionSet(state.walls);
    const energySet = buildPositionSet(state.energy);
    const enemyPositions = new Map<string, VisibleBot>();
    for (const bot of enemyBots) {
      enemyPositions.set(posKey(bot.position), bot);
    }

    // Get my cores for defenders
    const myCores = state.cores.filter(c => c.owner === myId && c.active);

    // Assign roles to each bot
    const assignments = this.assignRoles(
      myBots,
      enemyBots,
      myCores,
      state.energy,
      state.you,
      config,
      state
    );

    // Compute moves based on role assignments
    const moves: Move[] = [];
    const assignedTargets = new Set<string>();

    for (const assignment of assignments) {
      const move = this.computeMoveForRole(
        assignment,
        walls,
        energySet,
        enemyPositions,
        myCores,
        state,
        assignedTargets
      );
      if (move) {
        moves.push(move);
      }

      // Track seen positions for scouting
      this.seenPositions.add(posKey(assignment.bot.position));
    }

    return moves;
  }

  /**
   * Assign roles to each bot based on game state
   */
  private assignRoles(
    myBots: VisibleBot[],
    enemyBots: VisibleBot[],
    myCores: VisibleCore[],
    energyNodes: Position[],
    playerInfo: PlayerInfo,
    config: GameConfig,
    state: GameState
  ): RoleAssignment[] {
    const n = myBots.length;
    if (n === 0) return [];

    // Compute threat level: enemies near own cores / total bots
    let threatLevel = 0;
    if (myCores.length > 0) {
      const enemiesNearCores = enemyBots.filter(bot =>
        myCores.some(core => distance2(bot.position, core.position, config.rows, config.cols) <= 16) // 4 tiles radius
      ).length;
      threatLevel = enemiesNearCores / n;
    }

    // Compute economic pressure: spawn_cost - energy
    const spawnThreshold = config.spawn_cost;
    const economicPressure = Math.max(0, spawnThreshold - playerInfo.energy) / spawnThreshold;

    // Base role split
    let attackerPct = 0.50;
    let harvesterPct = 0.25;
    let defenderPct = 0.15;
    let scoutPct = 0.10;

    // Adjust based on threat level
    if (threatLevel > 0.3) {
      defenderPct += 0.10;
      attackerPct -= 0.10;
    }

    // Adjust based on economic pressure
    if (economicPressure > 0.5) {
      harvesterPct += 0.10;
      attackerPct -= 0.10;
    }

    // Adjust based on score (winning → more attackers)
    const maxScore = Math.max(playerInfo.score, 1);
    const avgEnemyScore = 1; // Simplified - could track actual enemy scores
    if (playerInfo.score > avgEnemyScore + 5) {
      attackerPct += 0.20;
      harvesterPct -= 0.10;
      defenderPct -= 0.10;
    }

    // Calculate counts
    const numAttackers = Math.max(1, Math.round(n * attackerPct));
    const numHarvesters = Math.max(1, Math.round(n * harvesterPct));
    const numDefenders = Math.max(0, Math.round(n * defenderPct));
    const numScouts = Math.max(0, n - numAttackers - numHarvesters - numDefenders);

    // Assign bots to roles by proximity to role targets (greedy Hungarian-style)
    const roles: BotRole[] = [];
    for (let i = 0; i < numAttackers; i++) roles.push('ATTACKER');
    for (let i = 0; i < numHarvesters; i++) roles.push('HARVESTER');
    for (let i = 0; i < numDefenders; i++) roles.push('DEFENDER');
    for (let i = 0; i < numScouts; i++) roles.push('SCOUT');

    // Calculate targets for each role type
    const attackerTarget = this.getAttackerTarget(enemyBots, state.cores, config, playerInfo.id);
    const harvesterTargets = this.getHarvesterTargets(energyNodes, myBots, enemyBots, config);
    const defenderTargets = myCores.map(c => c.position);
    const scoutTarget = this.getScoutTarget(state, config);

    // Greedy assignment: match each bot to nearest available role target
    const assignments: RoleAssignment[] = [];
    const usedBots = new Set<number>();
    const usedRoles = new Set<number>();

    // Sort bots by distance to their preferred role targets
    const botsWithScores = myBots.map((bot, idx) => {
      let bestRole: BotRole = 'ATTACKER';
      let bestDist = Infinity;

      // Score each role type for this bot
      if (roles.includes('ATTACKER') && attackerTarget) {
        const dist = distance2(bot.position, attackerTarget, config.rows, config.cols);
        if (dist < bestDist) { bestDist = dist; bestRole = 'ATTACKER'; }
      }
      if (roles.includes('HARVESTER') && harvesterTargets.length > 0) {
        const nearestHarvester = harvesterTargets.reduce((min, t) =>
          distance2(bot.position, t, config.rows, config.cols) < distance2(bot.position, min, config.rows, config.cols) ? t : min
        );
        const dist = distance2(bot.position, nearestHarvester, config.rows, config.cols);
        if (dist < bestDist) { bestDist = dist; bestRole = 'HARVESTER'; }
      }
      if (roles.includes('DEFENDER') && defenderTargets.length > 0) {
        const nearestDefender = defenderTargets.reduce((min, t) =>
          distance2(bot.position, t, config.rows, config.cols) < distance2(bot.position, min, config.rows, config.cols) ? t : min
        );
        const dist = distance2(bot.position, nearestDefender, config.rows, config.cols);
        if (dist < bestDist) { bestDist = dist; bestRole = 'DEFENDER'; }
      }
      if (roles.includes('SCOUT') && scoutTarget) {
        const dist = distance2(bot.position, scoutTarget, config.rows, config.cols);
        if (dist < bestDist) { bestDist = dist; bestRole = 'SCOUT'; }
      }

      return { bot, role: bestRole, dist: bestDist, idx };
    });

    // Assign bots greedily by closeness to their preferred role
    botsWithScores.sort((a, b) => a.dist - b.dist);

    for (const { bot, role, idx } of botsWithScores) {
      if (usedBots.has(idx)) continue;

      // Check if we still need this role
      const roleIndex = roles.indexOf(role);
      if (roleIndex === -1) continue;

      usedBots.add(idx);
      roles.splice(roleIndex, 1);

      const target = this.getTargetForRole(role, bot, attackerTarget, harvesterTargets, defenderTargets, scoutTarget, config);
      assignments.push({ bot, role, target });
    }

    return assignments;
  }

  /**
   * Get target for attacker role (weakest enemy cluster)
   */
  private getAttackerTarget(enemyBots: VisibleBot[], allCores: VisibleCore[], config: GameConfig, myId: number): Position | null {
    if (enemyBots.length === 0) {
      // No enemies visible - target nearest enemy core
      const enemyCores = allCores.filter(c => c.owner !== myId);
      if (enemyCores.length > 0) {
        return enemyCores[0].position;
      }
      return { row: Math.floor(config.rows / 2), col: Math.floor(config.cols / 2) };
    }

    // Find weakest cluster (fewest bots, nearest)
    const clusters = this.clusterBots(enemyBots, config);
    if (clusters.length === 0) {
      return enemyBots[0].position;
    }

    // Sort by size (ascending) then distance
    const myBotPos = { row: 0, col: 0 }; // Placeholder - will use actual bot position in assignment
    clusters.sort((a, b) => {
      if (a.length !== b.length) return a.length - b.length;
      const distA = distance2(myBotPos, a[0].position, config.rows, config.cols);
      const distB = distance2(myBotPos, b[0].position, config.rows, config.cols);
      return distA - distB;
    });

    return this.calculateCenter(clusters[0].map(b => b.position), config);
  }

  /**
   * Get targets for harvesters (nearest uncontested energy nodes)
   */
  private getHarvesterTargets(energyNodes: Position[], myBots: VisibleBot[], enemyBots: VisibleBot[], config: GameConfig): Position[] {
    if (energyNodes.length === 0) return [];

    // Filter out contested energy (near enemies)
    const uncontested = energyNodes.filter(energy => {
      return !enemyBots.some(bot =>
        distance2(energy, bot.position, config.rows, config.cols) <= config.vision_radius2
      );
    });

    return uncontested.length > 0 ? uncontested : energyNodes;
  }

  /**
   * Get target for scout role (unexplored region)
   */
  private getScoutTarget(state: GameState, config: GameConfig): Position {
    // Find a position that hasn't been seen recently
    const candidates: Position[] = [];

    for (let r = 0; r < config.rows; r += 3) {
      for (let c = 0; c < config.cols; c += 3) {
        const key = posKey({ row: r, col: c });
        if (!this.seenPositions.has(key)) {
          candidates.push({ row: r, col: c });
        }
      }
    }

    if (candidates.length > 0) {
      // Return random unseen position
      return candidates[Math.floor(Math.random() * candidates.length)];
    }

    // All seen - return center
    return { row: Math.floor(config.rows / 2), col: Math.floor(config.cols / 2) };
  }

  /**
   * Get target position for a specific role
   */
  private getTargetForRole(
    role: BotRole,
    bot: VisibleBot,
    attackerTarget: Position | null,
    harvesterTargets: Position[],
    defenderTargets: Position[],
    scoutTarget: Position | null,
    config: GameConfig
  ): Position | null {
    switch (role) {
      case 'ATTACKER':
        return attackerTarget;
      case 'HARVESTER':
        if (harvesterTargets.length === 0) return null;
        return harvesterTargets.reduce((min, t) =>
          distance2(bot.position, t, config.rows, config.cols) < distance2(bot.position, min, config.rows, config.cols) ? t : min
        );
      case 'DEFENDER':
        if (defenderTargets.length === 0) return null;
        return defenderTargets.reduce((min, t) =>
          distance2(bot.position, t, config.rows, config.cols) < distance2(bot.position, min, config.rows, config.cols) ? t : min
        );
      case 'SCOUT':
        return scoutTarget;
    }
  }

  /**
   * Compute move for a bot based on its assigned role
   */
  private computeMoveForRole(
    assignment: RoleAssignment,
    walls: Set<string>,
    energySet: Set<string>,
    enemyPositions: Map<string, VisibleBot>,
    myCores: VisibleCore[],
    state: GameState,
    assignedTargets: Set<string>
  ): Move | null {
    const { bot, role, target } = assignment;
    const config = state.config;
    const rows = config.rows;
    const cols = config.cols;

    // Zone awareness - survival priority
    if (state.zone && state.zone.active) {
      const distToZoneCenter2 = distance2(bot.position, state.zone.center, rows, cols);
      const safetyMargin2 = 4;
      if (distToZoneCenter2 >= state.zone.radius * state.zone.radius - safetyMargin2) {
        return this.moveTowardPosition(bot, state.zone.center, walls, rows, cols);
      }
    }

    switch (role) {
      case 'ATTACKER':
        return this.computeAttackerMove(bot, target, enemyPositions, walls, state);
      case 'HARVESTER':
        return this.computeHarvesterMove(bot, target, energySet, walls, state);
      case 'DEFENDER':
        return this.computeDefenderMove(bot, target, enemyPositions, walls, state);
      case 'SCOUT':
        return this.computeScoutMove(bot, target, walls, state);
    }
  }

  /**
   * Attacker: rush toward weakest enemy cluster
   */
  private computeAttackerMove(
    bot: VisibleBot,
    target: Position | null,
    enemyPositions: Map<string, VisibleBot>,
    walls: Set<string>,
    state: GameState
  ): Move | null {
    if (!target) return null;

    const config = state.config;
    const rows = config.rows;
    const cols = config.cols;

    let bestDir: Direction | null = null;
    let bestScore = -Infinity;

    for (const dir of ALL_DIRECTIONS) {
      const newPos = moveToward(bot.position, dir, rows, cols);
      const newPosKey = posKey(newPos);

      if (walls.has(newPosKey) || enemyPositions.has(newPosKey)) {
        continue;
      }

      let score = 0;

      // Primary: move toward target
      const distToTarget = distance2(newPos, target, rows, cols);
      const currentDistToTarget = distance2(bot.position, target, rows, cols);
      score += (currentDistToTarget - distToTarget) * 10;

      // Secondary: avoid getting surrounded
      const nearbyEnemies = this.countNearbyEnemies(newPos, enemyPositions, config);
      score -= nearbyEnemies * 5;

      // Tertiary: prefer attack range to target
      if (distToTarget <= config.attack_radius2 && distToTarget > 0) {
        score += 20;
      }

      if (score > bestScore) {
        bestScore = score;
        bestDir = dir;
      }
    }

    if (bestDir) {
      return { position: bot.position, direction: bestDir };
    }

    return null;
  }

  /**
   * Harvester: BFS toward nearest uncontested energy node
   */
  private computeHarvesterMove(
    bot: VisibleBot,
    target: Position | null,
    energySet: Set<string>,
    walls: Set<string>,
    state: GameState
  ): Move | null {
    if (!target) return null;

    const config = state.config;
    const rows = config.rows;
    const cols = config.cols;

    // Check if already on energy
    if (energySet.has(posKey(bot.position))) {
      // Stay put to collect
      return null;
    }

    return this.moveTowardPosition(bot, target, walls, rows, cols);
  }

  /**
   * Defender: stay near own core, intercept enemies
   */
  private computeDefenderMove(
    bot: VisibleBot,
    target: Position | null,
    enemyPositions: Map<string, VisibleBot>,
    walls: Set<string>,
    state: GameState
  ): Move | null {
    if (!target) return null;

    const config = state.config;
    const rows = config.rows;
    const cols = config.cols;
    const DEFEND_RADIUS2 = 16; // 4 tiles

    let bestDir: Direction | null = null;
    let bestScore = -Infinity;

    for (const dir of ALL_DIRECTIONS) {
      const newPos = moveToward(bot.position, dir, rows, cols);
      const newPosKey = posKey(newPos);

      if (walls.has(newPosKey) || enemyPositions.has(newPosKey)) {
        continue;
      }

      let score = 0;

      // Primary: stay within defend radius of core
      const distToCore2 = distance2(newPos, target, rows, cols);
      if (distToCore2 <= DEFEND_RADIUS2) {
        score += 50;
      } else {
        score -= (distToCore2 - DEFEND_RADIUS2) * 10;
      }

      // Secondary: move toward nearby enemies to intercept
      let nearestEnemyDist = Infinity;
      for (const enemy of enemyPositions.values()) {
        const dist = distance2(newPos, enemy.position, rows, cols);
        if (dist < nearestEnemyDist) {
          nearestEnemyDist = dist;
        }
      }

      if (nearestEnemyDist < Infinity) {
        // Bonus for positioning between core and enemy
        const currentNearestDist = this.minDistanceToEnemies(bot.position, enemyPositions, rows, cols);
        if (nearestEnemyDist <= config.attack_radius2 && nearestEnemyDist > 0) {
          score += 30; // In attack range
        }
        score += (currentNearestDist - nearestEnemyDist) * 2; // Move toward enemies
      }

      if (score > bestScore) {
        bestScore = score;
        bestDir = dir;
      }
    }

    if (bestDir) {
      return { position: bot.position, direction: bestDir };
    }

    return null;
  }

  /**
   * Scout: move toward unexplored region
   */
  private computeScoutMove(
    bot: VisibleBot,
    target: Position | null,
    walls: Set<string>,
    state: GameState
  ): Move | null {
    if (!target) return null;

    const config = state.config;
    const rows = config.rows;
    const cols = config.cols;

    return this.moveTowardPosition(bot, target, walls, rows, cols);
  }

  /**
   * Generic move-toward-position logic
   */
  private moveTowardPosition(
    bot: VisibleBot,
    target: Position,
    walls: Set<string>,
    rows: number,
    cols: number
  ): Move | null {
    let bestDir: Direction | null = null;
    let bestDist2 = Infinity;

    for (const dir of ALL_DIRECTIONS) {
      const newPos = moveToward(bot.position, dir, rows, cols);
      const newPosKey = posKey(newPos);

      if (walls.has(newPosKey)) {
        continue;
      }

      const dist2 = distance2(newPos, target, rows, cols);
      if (dist2 < bestDist2) {
        bestDist2 = dist2;
        bestDir = dir;
      }
    }

    if (bestDir) {
      return { position: bot.position, direction: bestDir };
    }

    return null;
  }

  /**
   * Cluster nearby bots into groups
   */
  private clusterBots(bots: VisibleBot[], config: GameConfig): VisibleBot[][] {
    const clusters: VisibleBot[][] = [];
    const used = new Set<string>();

    for (const bot of bots) {
      const key = posKey(bot.position);
      if (used.has(key)) continue;

      const cluster: VisibleBot[] = [bot];
      used.add(key);

      // Find nearby bots
      for (const other of bots) {
        if (posKey(other.position) === key) continue;
        if (used.has(posKey(other.position))) continue;

        const dist2 = distance2(bot.position, other.position, config.rows, config.cols);
        if (dist2 <= 9) { // 3 tiles radius
          cluster.push(other);
          used.add(posKey(other.position));
        }
      }

      clusters.push(cluster);
    }

    return clusters;
  }

  /**
   * Calculate center of mass of positions
   */
  private calculateCenter(positions: Position[], config: GameConfig): Position {
    if (positions.length === 0) {
      return { row: Math.floor(config.rows / 2), col: Math.floor(config.cols / 2) };
    }

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
      row: ((Math.floor(avgRow) % config.rows) + config.rows) % config.rows,
      col: ((Math.floor(avgCol) % config.cols) + config.cols) % config.cols,
    };
  }

  /**
   * Count nearby enemies
   */
  private countNearbyEnemies(pos: Position, enemyPositions: Map<string, VisibleBot>, config: GameConfig): number {
    let count = 0;
    for (const enemy of enemyPositions.values()) {
      const dist2 = distance2(pos, enemy.position, config.rows, config.cols);
      if (dist2 <= config.vision_radius2) {
        count++;
      }
    }
    return count;
  }

  /**
   * Get minimum distance to any enemy
   */
  private minDistanceToEnemies(pos: Position, enemyPositions: Map<string, VisibleBot>, rows: number, cols: number): number {
    let minDist = Infinity;
    for (const enemy of enemyPositions.values()) {
      const dist = distance2(pos, enemy.position, rows, cols);
      if (dist < minDist) {
        minDist = dist;
      }
    }
    return minDist;
  }
}
