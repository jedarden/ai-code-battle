const { distance2, manhattan, moveDir, posKey } = require("./grid");

const DIRECTIONS = ["N", "E", "S", "W"];

function computeMoves(state) {
  const { rows, cols, attack_radius2 } = state.config;
  const myId = state.you.id;

  const myBots = [];
  const enemyBots = [];
  for (const bot of state.bots) {
    if (bot.owner === myId) myBots.push(bot);
    else enemyBots.push(bot);
  }
  if (myBots.length === 0) return [];

  const walls = new Set(state.walls.map((w) => posKey(w.row, w.col)));
  const enemyCores = state.cores.filter(
    (c) => c.owner !== myId && c.active
  );
  const energySet = new Set(
    (state.energy || []).map((e) => posKey(e.row, e.col))
  );

  const committed = new Set();
  const moves = [];

  // Bots already adjacent to enemies get priority so they press the attack
  myBots.sort((a, b) => {
    const da = nearestEnemyDist2(a.position, enemyBots, rows, cols);
    const db = nearestEnemyDist2(b.position, enemyBots, rows, cols);
    return da - db;
  });

  for (const bot of myBots) {
    const br = bot.position.row;
    const bc = bot.position.col;

    // Find the nearest enemy
    const target = findNearestEnemy(br, bc, enemyBots, rows, cols);

    let bestDir = null;
    let bestScore = -Infinity;

    for (const dir of DIRECTIONS) {
      const [nr, nc] = moveDir(br, bc, dir, rows, cols);
      const nk = posKey(nr, nc);

      if (walls.has(nk)) continue;
      if (committed.has(nk)) continue;

      let score = 0;

      if (target) {
        // Primary: move toward nearest enemy — minimize squared distance
        const distToTarget = distance2(
          nr, nc, target.position.row, target.position.col, rows, cols
        );
        score -= distToTarget * 10;

        // Bonus for being within attack range (press the engagement)
        if (distToTarget <= attack_radius2) {
          score += 50;
        }

        // Secondary: prefer directions that also close distance to other enemies
        let totalEnemyDist = 0;
        for (const e of enemyBots) {
          totalEnemyDist += distance2(
            nr, nc, e.position.row, e.position.col, rows, cols
          );
        }
        score -= totalEnemyDist * 0.1;

        // Small bonus for collecting energy if it's along the way
        if (energySet.has(nk)) {
          score += 5;
        }
      } else {
        // No enemies visible — march toward enemy core
        if (enemyCores.length > 0) {
          const coreDist = nearestCoreDist(
            nr, nc, enemyCores, rows, cols
          );
          score -= coreDist * 10;
        }

        // Collect energy opportunistically
        if (energySet.has(nk)) {
          score += 3;
        }
      }

      if (score > bestScore) {
        bestScore = score;
        bestDir = dir;
      }
    }

    // Commit the destination to prevent self-collision
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
  }

  return moves;
}

function findNearestEnemy(r, c, enemyBots, rows, cols) {
  let best = null;
  let bestDist = Infinity;
  for (const e of enemyBots) {
    const d = distance2(r, c, e.position.row, e.position.col, rows, cols);
    if (d < bestDist) {
      bestDist = d;
      best = e;
    }
  }
  return best;
}

function nearestEnemyDist2(pos, enemyBots, rows, cols) {
  let minD = Infinity;
  for (const e of enemyBots) {
    const d = distance2(
      pos.row, pos.col, e.position.row, e.position.col, rows, cols
    );
    if (d < minD) minD = d;
  }
  return minD;
}

function nearestCoreDist(r, c, cores, rows, cols) {
  let minD = Infinity;
  for (const core of cores) {
    const d = manhattan(
      r, c, core.position.row, core.position.col, rows, cols
    );
    if (d < minD) minD = d;
  }
  return minD;
}

module.exports = { computeMoves };
