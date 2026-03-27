//! RusherBot strategy: aggressive core-rushing behavior.
//!
//! This strategy identifies and rushes the nearest enemy core as fast as possible.
//! Bots use BFS to find paths to enemy cores, ignoring energy and enemy bots
//! unless they block the path.

use crate::game::{Direction, GameConfig, GameState, Move, Position, VisibleBot, VisibleCore};
use std::collections::{HashMap, HashSet, VecDeque};

/// RusherStrategy implements aggressive core-rushing behavior.
pub struct RusherStrategy {
    /// Known enemy core positions (discovered during gameplay)
    known_enemy_cores: HashSet<Position>,
}

impl RusherStrategy {
    pub fn new() -> Self {
        Self {
            known_enemy_cores: HashSet::new(),
        }
    }

    /// Compute moves for all owned bots
    pub fn compute_moves(&mut self, state: &GameState) -> Vec<Move> {
        let my_id = state.you.id;
        let config = &state.config;

        // Update known enemy cores
        self.update_known_cores(state, my_id);

        // Separate my bots from enemies
        let (my_bots, enemy_bots): (Vec<_>, Vec<_>) =
            state.bots.iter().partition(|b| b.owner == my_id);

        if my_bots.is_empty() {
            return vec![];
        }

        // Build position lookup for enemies
        let enemy_positions: HashSet<Position> =
            enemy_bots.iter().map(|b| b.position).collect();

        // Build wall lookup
        let walls: HashSet<Position> = state.walls.iter().copied().collect();

        // Find target cores to rush
        let targets = self.get_rush_targets(state, my_id);

        // Assign each bot to the nearest target
        let mut moves = Vec::with_capacity(my_bots.len());
        let mut assigned_targets: HashSet<Position> = HashSet::new();

        for bot in &my_bots {
            if let Some((dir, _)) = self.find_best_move(
                bot.position,
                &targets,
                &enemy_positions,
                &walls,
                &assigned_targets,
                config,
            ) {
                // Mark target as assigned to avoid duplicates
                if let Some(target) = self.find_target_for_bot(bot.position, &targets, config) {
                    assigned_targets.insert(target);
                }
                moves.push(Move {
                    position: bot.position,
                    direction: dir,
                });
            }
        }

        moves
    }

    /// Update known enemy cores from visible state
    fn update_known_cores(&mut self, state: &GameState, my_id: u32) {
        for core in &state.cores {
            if core.owner != my_id {
                self.known_enemy_cores.insert(core.position);
            }
        }
    }

    /// Get list of cores to rush (enemy cores first, then explore)
    fn get_rush_targets(&self, state: &GameState, my_id: u32) -> Vec<Position> {
        let mut targets: Vec<Position> = state
            .cores
            .iter()
            .filter(|c| c.owner != my_id && c.active)
            .map(|c| c.position)
            .collect();

        // If we know about enemy cores from previous turns, include them
        for pos in &self.known_enemy_cores {
            if !targets.contains(pos) {
                targets.push(*pos);
            }
        }

        // If no enemy cores known, explore the map
        if targets.is_empty() {
            // Add exploration targets at grid edges
            let rows = state.config.rows as i32;
            let cols = state.config.cols as i32;
            targets.push(Position { row: rows / 2, col: cols / 2 });
            targets.push(Position { row: 0, col: 0 });
            targets.push(Position { row: rows - 1, col: cols - 1 });
        }

        targets
    }

    /// Find the best move for a bot using BFS toward targets
    fn find_best_move(
        &self,
        start: Position,
        targets: &[Position],
        enemy_positions: &HashSet<Position>,
        walls: &HashSet<Position>,
        _assigned_targets: &HashSet<Position>,
        config: &GameConfig,
    ) -> Option<(Direction, Position)> {
        let rows = config.rows as i32;
        let cols = config.cols as i32;

        // BFS to find shortest path to any target
        let mut visited: HashSet<Position> = HashSet::new();
        let mut queue: VecDeque<(Position, Option<Direction>)> = VecDeque::new();

        visited.insert(start);
        queue.push_back((start, None));

        while let Some((pos, first_dir)) = queue.pop_front() {
            // Check if we've reached a target
            if targets.contains(&pos) {
                if let Some(dir) = first_dir {
                    return Some((dir, pos));
                }
            }

            // Explore neighbors
            for dir in Direction::all() {
                let next = pos.move_toward(dir, rows, cols);

                if visited.contains(&next) || walls.contains(&next) {
                    continue;
                }

                // Don't walk into enemy bots (but allow pathing near them)
                if enemy_positions.contains(&next) {
                    continue;
                }

                visited.insert(next);
                queue.push_back((next, first_dir.or(Some(dir))));
            }
        }

        // No path found - pick a random direction
        for dir in Direction::all() {
            let next = start.move_toward(dir, rows, cols);
            if !walls.contains(&next) && !enemy_positions.contains(&next) {
                return Some((dir, next));
            }
        }

        None
    }

    /// Find the nearest target for a bot
    fn find_target_for_bot(
        &self,
        bot_pos: Position,
        targets: &[Position],
        config: &GameConfig,
    ) -> Option<Position> {
        let rows = config.rows as i32;
        let cols = config.cols as i32;

        targets
            .iter()
            .min_by_key(|t| bot_pos.distance2(t, rows, cols))
            .copied()
    }
}

impl Default for RusherStrategy {
    fn default() -> Self {
        Self::new()
    }
}
