/**
 * Grid utility functions for AI Code Battle.
 *
 * Provides toroidal distance calculations, neighbor enumeration,
 * and BFS pathfinding on a wrapping grid.
 */

function toroidalManhattan(r1, c1, r2, c2, cols, rows) {
  const dr = Math.min(Math.abs(r1 - r2), rows - Math.abs(r1 - r2));
  const dc = Math.min(Math.abs(c1 - c2), cols - Math.abs(c1 - c2));
  return dr + dc;
}

function toroidalChebyshev(r1, c1, r2, c2, cols, rows) {
  const dr = Math.min(Math.abs(r1 - r2), rows - Math.abs(r1 - r2));
  const dc = Math.min(Math.abs(c1 - c2), cols - Math.abs(c1 - c2));
  return Math.max(dr, dc);
}

function neighbors(row, col, rows, cols) {
  const offsets = [
    [-1, -1], [-1, 0], [-1, 1],
    [0, -1],           [0, 1],
    [1, -1],  [1, 0],  [1, 1],
  ];
  return offsets.map(([dr, dc]) => [
    (row + dr + rows) % rows,
    (col + dc + cols) % cols,
  ]);
}

/**
 * BFS pathfinding on a toroidal grid.
 *
 * @param {[number,number]} start - [row, col]
 * @param {[number,number]} goal - [row, col]
 * @param {function(number,number): boolean} passable - returns true if walkable
 * @param {number} rows
 * @param {number} cols
 * @returns {[number,number][]|null} path from start to goal (excl. start), or null
 */
function bfs(start, goal, passable, rows, cols) {
  const [sr, sc] = start;
  const [gr, gc] = goal;
  if (sr === gr && sc === gc) return [];

  const key = (r, c) => `${r},${c}`;
  const visited = new Set([key(sr, sc)]);
  const queue = [{ r: sr, c: sc, path: [] }];

  while (queue.length > 0) {
    const { r, c, path } = queue.shift();
    for (const [nr, nc] of neighbors(r, c, rows, cols)) {
      const newPath = [...path, [nr, nc]];
      if (nr === gr && nc === gc) return newPath;
      const k = key(nr, nc);
      if (!visited.has(k) && passable(nr, nc)) {
        visited.add(k);
        queue.push({ r: nr, c: nc, path: newPath });
      }
    }
  }
  return null;
}

module.exports = {
  toroidalManhattan,
  toroidalChebyshev,
  neighbors,
  bfs,
};
