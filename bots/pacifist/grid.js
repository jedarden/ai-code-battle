/**
 * Grid utility functions for AI Code Battle.
 * Toroidal distance calculations, neighbor enumeration, and BFS.
 */

function toroidalDelta(a, b, size) {
  const d = Math.abs(a - b);
  return Math.min(d, size - d);
}

function distance2(r1, c1, r2, c2, rows, cols) {
  const dr = toroidalDelta(r1, r2, rows);
  const dc = toroidalDelta(c1, c2, cols);
  return dr * dr + dc;
}

function manhattan(r1, c1, r2, c2, rows, cols) {
  return toroidalDelta(r1, r2, rows) + toroidalDelta(c1, c2, cols);
}

function moveDir(row, col, dir, rows, cols) {
  switch (dir) {
    case "N": return [(row - 1 + rows) % rows, col];
    case "E": return [row, (col + 1) % cols];
    case "S": return [(row + 1) % rows, col];
    case "W": return [row, (col - 1 + cols) % cols];
  }
}

function posKey(r, c) {
  return `${r},${c}`;
}

module.exports = { distance2, manhattan, moveDir, posKey };
