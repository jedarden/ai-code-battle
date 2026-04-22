//! Assassin strategy: decapitation archetype — all units rush the enemy core.
//!
//! Ignores economy, ignores enemy units (unless directly blocking), and commits
//! fully to core destruction. No perimeter defense. Relies on speed and mass.

use crate::game::{Direction, GameConfig, GameState, Move, Position};
use std::collections::{HashMap, HashSet, VecDeque};

pub struct AssassinStrategy {
    /// Enemy cores discovered so far (persisted across turns)
    known_targets: HashMap<Position, bool>,
}

impl AssassinStrategy {
    pub fn new() -> Self {
        Self {
            known_targets: HashMap::new(),
        }
    }

    pub fn compute_moves(&mut self, state: &GameState) -> Vec<Move> {
        let my_id = state.you.id;
        let config = &state.config;
        let rows = config.rows as i32;
        let cols = config.cols as i32;

        self.update_targets(state, my_id);

        let my_bots: Vec<_> = state.bots.iter().filter(|b| b.owner == my_id).collect();
        if my_bots.is_empty() {
            return vec![];
        }

        let walls: HashSet<Position> = state.walls.iter().copied().collect();

        // Active enemy core targets, sorted by distance from our center of mass
        let targets = self.active_targets();
        let center = center_of_mass(&my_bots);

        let mut sorted_targets: Vec<Position> = targets
            .iter()
            .filter(|(_, active)| **active)
            .map(|(pos, _)| *pos)
            .collect();

        sorted_targets.sort_by_key(|t| center.distance2(t, rows, cols));

        // If no known targets, explore outward
        if sorted_targets.is_empty() {
            return self.explore_moves(&my_bots, &walls, config);
        }

        // Primary target: nearest active enemy core to our center of mass
        let primary = sorted_targets[0];

        // BFS from each bot to the primary target, walking through enemies
        let mut moves = Vec::with_capacity(my_bots.len());
        let mut destinations: HashSet<Position> = HashSet::new();

        for bot in &my_bots {
            if let Some(dir) = self.bfs_toward(bot.position, primary, &walls, &destinations, rows, cols) {
                let dest = bot.position.move_toward(dir, rows, cols);
                destinations.insert(dest);
                moves.push(Move {
                    position: bot.position,
                    direction: dir,
                });
            }
        }

        moves
    }

    /// Update known targets from visible cores
    fn update_targets(&mut self, state: &GameState, my_id: u32) {
        for core in &state.cores {
            if core.owner != my_id {
                self.known_targets
                    .entry(core.position)
                    .and_modify(|a| *a = core.active)
                    .or_insert(core.active);
            }
        }
    }

    fn active_targets(&self) -> &HashMap<Position, bool> {
        &self.known_targets
    }

    /// BFS toward a target position. Unlike rusher, does NOT avoid enemy bots —
    /// we walk straight through them. Only walls block movement.
    /// Avoids self-collision by checking already-claimed destinations.
    fn bfs_toward(
        &self,
        start: Position,
        goal: Position,
        walls: &HashSet<Position>,
        claimed: &HashSet<Position>,
        rows: i32,
        cols: i32,
    ) -> Option<Direction> {
        if start == goal {
            return None;
        }

        let mut visited: HashSet<Position> = HashSet::new();
        let mut queue: VecDeque<(Position, Option<Direction>)> = VecDeque::new();
        visited.insert(start);
        queue.push_back((start, None));

        while let Some((pos, first_dir)) = queue.pop_front() {
            if pos == goal {
                return first_dir;
            }

            for dir in Direction::all() {
                let next = pos.move_toward(dir, rows, cols);
                if visited.contains(&next) || walls.contains(&next) {
                    continue;
                }
                visited.insert(next);
                queue.push_back((next, first_dir.or(Some(dir))));
            }
        }

        // No path to goal — pick the direction that gets us closest
        let mut best_dir = None;
        let mut best_dist = i32::MAX;
        for dir in Direction::all() {
            let next = start.move_toward(dir, rows, cols);
            if walls.contains(&next) || claimed.contains(&next) {
                continue;
            }
            let dr = (next.row - goal.row).abs();
            let dc = (next.col - goal.col).abs();
            let dist = dr.min(rows - dr) + dc.min(cols - dc);
            if dist < best_dist {
                best_dist = dist;
                best_dir = Some(dir);
            }
        }
        best_dir
    }

    /// When no targets are known, spread bots outward to find enemy cores
    fn explore_moves(
        &self,
        my_bots: &[&crate::game::VisibleBot],
        walls: &HashSet<Position>,
        config: &GameConfig,
    ) -> Vec<Move> {
        let rows = config.rows as i32;
        let cols = config.cols as i32;
        let mut moves = Vec::with_capacity(my_bots.len());

        // Spread in a line toward the opposite side of the map
        for (i, bot) in my_bots.iter().enumerate() {
            let target_col = if i % 2 == 0 { cols - 1 } else { 0 };
            let target_row = if i % 3 == 0 { rows / 2 } else { rows - 1 };
            let target = Position { row: target_row, col: target_col };

            let mut best_dir = None;
            let mut best_dist = i32::MAX;
            for dir in Direction::all() {
                let next = bot.position.move_toward(dir, rows, cols);
                if walls.contains(&next) {
                    continue;
                }
                let dr = (next.row - target.row).abs();
                let dc = (next.col - target.col).abs();
                let dist = dr.min(rows - dr) + dc.min(cols - dc);
                if dist < best_dist {
                    best_dist = dist;
                    best_dir = Some(dir);
                }
            }
            if let Some(dir) = best_dir {
                moves.push(Move {
                    position: bot.position,
                    direction: dir,
                });
            }
        }

        moves
    }
}

impl Default for AssassinStrategy {
    fn default() -> Self {
        Self::new()
    }
}

/// Compute center of mass of our bots
fn center_of_mass(bots: &[&crate::game::VisibleBot]) -> Position {
    if bots.is_empty() {
        return Position { row: 0, col: 0 };
    }
    let sum_r: i32 = bots.iter().map(|b| b.position.row).sum();
    let sum_c: i32 = bots.iter().map(|b| b.position.col).sum();
    Position {
        row: sum_r / bots.len() as i32,
        col: sum_c / bots.len() as i32,
    }
}
