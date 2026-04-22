//! Grid utility functions for AI Code Battle.
//!
//! Provides toroidal distance calculations, neighbor enumeration,
//! and BFS pathfinding on a wrapping grid.

use std::collections::{HashMap, VecDeque};

/// Manhattan distance with wrap-around on a toroidal grid.
pub fn toroidal_manhattan(a: &Position, b: &Position, rows: u32, cols: u32) -> u32 {
    let dr = (a.row as i32 - b.row as i32).unsigned_abs();
    let dc = (a.col as i32 - b.col as i32).unsigned_abs();
    dr.min(rows - dr) + dc.min(cols - dc)
}

/// Chebyshev distance with wrap-around on a toroidal grid.
pub fn toroidal_chebyshev(a: &Position, b: &Position, rows: u32, cols: u32) -> u32 {
    let dr = (a.row as i32 - b.row as i32).unsigned_abs();
    let dc = (a.col as i32 - b.col as i32).unsigned_abs();
    dr.min(rows - dr).max(dc.min(cols - dc))
}

/// 8-directional neighbors with wrap-around.
pub fn neighbors(pos: &Position, rows: u32, cols: u32) -> Vec<Position> {
    const OFFSETS: [(i32, i32); 8] = [
        (-1, -1), (-1, 0), (-1, 1),
        (0, -1),           (0, 1),
        (1, -1),  (1, 0),  (1, 1),
    ];
    OFFSETS
        .iter()
        .map(|(dr, dc)| Position {
            row: (pos.row as i32 + dr).rem_euclid(rows as i32) as u32,
            col: (pos.col as i32 + dc).rem_euclid(cols as i32) as u32,
        })
        .collect()
}

/// BFS pathfinding on a toroidal grid.
///
/// `passable` returns true if a cell can be entered.
/// Returns the path (excluding start) or None if unreachable.
pub fn bfs(
    start: &Position,
    goal: &Position,
    passable: impl Fn(&Position) -> bool,
    rows: u32,
    cols: u32,
) -> Option<Vec<Position>> {
    if start.row == goal.row && start.col == goal.col {
        return Some(vec![]);
    }

    let mut visited: HashMap<(u32, u32), bool> = HashMap::new();
    visited.insert((start.row, start.col), true);

    let mut queue: VecDeque<(Position, Vec<Position>)> = VecDeque::new();
    queue.push_back((start.clone(), vec![]));

    while let Some((cur, path)) = queue.pop_front() {
        for n in neighbors(&cur, rows, cols) {
            let mut new_path = path.clone();
            new_path.push(n.clone());
            if n.row == goal.row && n.col == goal.col {
                return Some(new_path);
            }
            let key = (n.row, n.col);
            if !visited.contains_key(&key) && passable(&n) {
                visited.insert(key, true);
                queue.push_back((n, new_path));
            }
        }
    }
    None
}
