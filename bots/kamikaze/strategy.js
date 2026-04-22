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

  // Reserve all starting positions so no two bots end up on the same tile.
  // When a bot moves away, its old cell is freed for others.
  const committed = new Set(
    myBots.map((b) => posKey(b.position.row, b.position.col))
  );
  const moves = [];

  // Bots closest to enemies decide first — they get priority on attack positions
  myBots.sort((a, b) => {
    const da = nearestEnemyDist2(a.position, enemyBots, rows, cols);
    const db = nearestEnemyDist2(b.position, enemyBots, rows, cols);
    return da - db;
  });

  for (const bot of myBots) {
    const br = bot.position.row;
    const bc = bot.position.col;

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
        const distToTarget = distance2(
          nr, nc, target.position.row, target.position.col, rows, cols
        );

        // Close distance to nearest enemy as fast as possible
        score -= distToTarget * 100;

        // Heavy bonus for staying in attack range — press the engagement
        if (distToTarget <= attack_radius2) {
          score += 200;
        }

        // Also prefer closing distance to other enemies (don't tunnel-vision)
        let totalEnemyDist = 0;
        for (const e of enemyBots) {
          totalEnemyDist += distance2(
            nr, nc, e.position.row, e.position.col, rows, cols
          );
        }
        score -= totalEnemyDist;

        // Grab energy only when it's directly along the attack path
        if (energySet.has(nk)) {
          score += 5;
        }
      } else {
        // No enemies visible — rush enemy core to raze it
        if (enemyCores.length > 0) {
          const coreDist = nearestCoreDist(
            nr, nc, enemyCores, rows, cols
          );
          score -= coreDist * 100;
        } else {
          // No targets at all — spread outward to explore
          // Prefer moving away from other friendly bots
          let friendProximity = 0;
          for (const other of myBots) {
            if (other === bot) continue;
            friendProximity += distance2(
              nr, nc, other.position.row, other.position.col, rows, cols
            );
          }
          score += friendProximity * 0.5;
        }

        // Collect energy while roaming (we need it to keep spawning)
        if (energySet.has(nk)) {
          score += 10;
        }
      }

      if (score > bestScore) {
        bestScore = score;
        bestDir = dir;
      }
    }

    if (bestDir) {
      const [nr, nc] = moveDir(br, bc, bestDir, rows, cols);
      committed.delete(posKey(br, bc));
      committed.add(posKey(nr, nc));
      moves.push({
        position: { row: br, col: bc },
        direction: bestDir,
      });
    }
    // If no direction is viable the bot holds; its starting cell stays in committed
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
