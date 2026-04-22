/**
 * PacifistBot strategy: pure evasion, never attacks.
 *
 * - Each bot moves to maximize distance from the nearest visible enemy.
 * - If cornered (enemy within attack radius), retreat toward own core.
 * - Never initiates combat; no moves toward enemies.
 * - Avoids self-collision (two friendly bots on same tile).
 * - Spawning is automatic (handled by the engine), so we conserve energy
 *   by not rushing into contested energy nodes.
 */

const { distance2, manhattan, moveDir, posKey } = require("./grid");

const DIRECTIONS = ["N", "E", "S", "W"];

function computeMoves(state) {
  const { rows, cols, attack_radius2 } = state.config;
  const myId = state.you.id;

  // Partition bots
  const myBots = [];
  const enemyBots = [];
  for (const bot of state.bots) {
    if (bot.owner === myId) myBots.push(bot);
    else enemyBots.push(bot);
  }
  if (myBots.length === 0) return [];

  // Build wall set
  const walls = new Set(state.walls.map((w) => posKey(w.row, w.col)));

  // Own active cores — safe zones to retreat to
  const myCores = state.cores.filter(
    (c) => c.owner === myId && c.active
  );

  // Enemy position list for distance lookups
  const enemyPos = enemyBots.map((b) => b.position);

  // Track committed positions to avoid self-collision
  const committed = new Set();

  const moves = [];

  // Sort bots: those closest to enemies get priority (they need to flee first)
  myBots.sort((a, b) => {
    const distA = nearestEnemyDist(a.position, enemyPos, rows, cols);
    const distB = nearestEnemyDist(b.position, enemyPos, rows, cols);
    return distA - distB;
  });

  for (const bot of myBots) {
    const br = bot.position.row;
    const bc = bot.position.col;

    // Check if cornered — enemy within attack radius
    const cornered = isInDanger(br, bc, enemyPos, rows, cols, attack_radius2);

    let bestDir = null;
    let bestScore = -Infinity;

    for (const dir of DIRECTIONS) {
      const [nr, nc] = moveDir(br, bc, dir, rows, cols);
      const nk = posKey(nr, nc);

      // Can't move into walls
      if (walls.has(nk)) continue;

      // Can't move onto a tile occupied by an enemy (would cause combat)
      if (enemyPos.some((e) => e.row === nr && e.col === nc)) continue;

      // Avoid self-collision with already-committed moves
      if (committed.has(nk)) continue;

      let score = 0;

      if (enemyPos.length > 0) {
        // Primary: maximize minimum distance to any enemy
        const minDist = nearestEnemyDist({ row: nr, col: nc }, enemyPos, rows, cols);
        score += minDist * 10;

        // Bonus: also increase total distance to all enemies
        let totalDist = 0;
        for (const e of enemyPos) {
          totalDist += distance2(nr, nc, e.row, e.col, rows, cols);
        }
        score += totalDist * 0.5;

        // Penalty: moving closer to enemies
        const currentMinDist = nearestEnemyDist(bot.position, enemyPos, rows, cols);
        if (minDist < currentMinDist) {
          score -= 20;
        }
      }

      if (cornered && myCores.length > 0) {
        // When cornered, strong preference for moving toward own core
        const coreDist = nearestCoreDist(nr, nc, myCores, rows, cols);
        const currentCoreDist = nearestCoreDist(br, bc, myCores, rows, cols);
        // Big bonus for moving closer to core
        score += (currentCoreDist - coreDist) * 15;
      } else if (enemyPos.length === 0 && myCores.length > 0) {
        // No enemies visible — drift toward own core area for safety
        const coreDist = nearestCoreDist(nr, nc, myCores, rows, cols);
        score -= coreDist * 2;
      }

      if (score > bestScore) {
        bestScore = score;
        bestDir = dir;
      }
    }

    // If no direction is safe, hold position (don't move)
    const targetKey = bestDir
      ? posKey(...moveDir(br, bc, bestDir, rows, cols))
      : posKey(br, bc);

    if (!committed.has(targetKey)) {
      committed.add(targetKey);
      if (bestDir) {
        moves.push({
          position: { row: br, col: bc },
          direction: bestDir,
        });
      }
    }
    // If target is already committed by another bot, this bot holds position
    // (intentionally skip to avoid self-collision)
  }

  return moves;
}

function nearestEnemyDist(pos, enemyPos, rows, cols) {
  let minD = Infinity;
  for (const e of enemyPos) {
    const d = distance2(pos.row, pos.col, e.row, e.col, rows, cols);
    if (d < minD) minD = d;
  }
  return minD;
}

function isInDanger(r, c, enemyPos, rows, cols, attackRadius2) {
  for (const e of enemyPos) {
    if (distance2(r, c, e.row, e.col, rows, cols) <= attackRadius2) {
      return true;
    }
  }
  return false;
}

function nearestCoreDist(r, c, cores, rows, cols) {
  let minD = Infinity;
  for (const core of cores) {
    const d = manhattan(r, c, core.position.row, core.position.col, rows, cols);
    if (d < minD) minD = d;
  }
  return minD;
}

module.exports = { computeMoves };
