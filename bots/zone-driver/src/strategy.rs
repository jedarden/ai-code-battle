//! ZoneDriverStrategy - Weaponize the shrinking zone to force enemy eliminations.
//!
//! This bot exploits the shrinking zone as its primary weapon:
//! 1. Computes the "kill band" - tiles where enemies will die next turn
//! 2. Positions units to block escape routes inward
//! 3. Herds enemies toward zone edge for passive eliminations
//! 4. Prioritizes survival for own bots near zone boundary

use crate::game::{Direction, GameConfig, GameState, Move, Position, VisibleBot};
use std::collections::{HashSet, VecDeque};

/// ZoneDriver implements zone-aware herding strategy
pub struct ZoneDriverStrategy {
    /// Track which bots we've assigned this turn
    assigned_bots: HashSet<Position>,
}

impl ZoneDriverStrategy {
    pub fn new() -> Self {
        Self {
            assigned_bots: HashSet::new(),
        }
    }

    /// Compute moves for all owned bots
    pub fn compute_moves(&mut self, state: &GameState) -> Vec<Move> {
        let my_id = state.you.id;
        let config = &state.config;

        // Reset assigned bots for this turn
        self.assigned_bots.clear();

        // Separate my bots from enemies
        let (my_bots, enemy_bots): (Vec<_>, Vec<_>) =
            state.bots.iter().partition(|b| b.owner == my_id);

        if my_bots.is_empty() {
            return vec![];
        }

        // Build position lookups
        let enemy_positions: HashSet<Position> =
            enemy_bots.iter().map(|b| b.position).collect();
        let walls: HashSet<Position> = state.walls.iter().copied().collect();

        // If no active zone, fall back to defensive positioning
        let zone = match &state.zone {
            Some(z) if z.active => z,
            _ => {
                // No active zone - play conservatively: spread out and defend
                return self.defensive_fallback(&my_bots, &enemy_positions, &walls, config);
            }
        };

        let mut moves = Vec::with_capacity(my_bots.len());

        // PRIORITY 1: Save own bots that are outside or dangerously close to zone edge
        for bot in &my_bots {
            let dist_to_zone = self.distance_to_zone_edge(bot.position, zone, config);
            if dist_to_zone <= 0 {
                // Bot is outside or on zone edge - immediate retreat inward
                if let Some(mv) = self.retreat_to_zone(bot.position, zone, &walls, config) {
                    moves.push(mv);
                    self.assigned_bots.insert(bot.position);
                }
            }
        }

        // PRIORITY 2: Block enemy escape routes from kill band
        // The kill band is the ring of tiles just inside the zone edge
        let kill_band_inner = (zone.radius.saturating_sub(2) as i32).max(0);
        let kill_band_outer = zone.radius as i32;

        for bot in &my_bots {
            if self.assigned_bots.contains(&bot.position) {
                continue; // Already assigned to save itself
            }

            // Find nearest enemy in kill band
            if let Some(target) = self.find_enemy_in_kill_band(
                bot.position,
                &enemy_bots,
                zone,
                kill_band_inner,
                kill_band_outer,
                config,
            ) {
                // Move to block escape routes (position adjacent to enemy on the inward side)
                if let Some(mv) = self.block_escape_route(bot.position, target, zone, &walls, config) {
                    moves.push(mv);
                    self.assigned_bots.insert(bot.position);
                }
            }
        }

        // PRIORITY 3: Form sweeping line to push remaining enemies toward zone
        for bot in &my_bots {
            if self.assigned_bots.contains(&bot.position) {
                continue;
            }

            // Advance toward nearest enemy or zone edge to apply pressure
            if let Some(mv) = self.advance_pressure(bot.position, &enemy_positions, zone, &walls, config) {
                moves.push(mv);
                self.assigned_bots.insert(bot.position);
            }
        }

        // Any remaining bots hold position
        for bot in &my_bots {
            if !self.assigned_bots.contains(&bot.position) {
                moves.push(Move {
                    position: bot.position,
                    direction: Direction::N, // Will be overridden to "stay"
                });
            }
        }

        moves
    }

    /// Calculate distance from position to zone edge (positive = inside, 0 = on edge, negative = outside)
    fn distance_to_zone_edge(&self, pos: Position, zone: &crate::game::ZoneBounds, config: &GameConfig) -> i32 {
        let dist_to_center = pos.distance2(&zone.center, config.rows as i32, config.cols as i32) as f64;
        let dist_from_center = dist_to_center.sqrt();
        let zone_radius = zone.radius as f64;
        (zone_radius - dist_from_center) as i32
    }

    /// Retreat toward zone center when outside or near edge
    fn retreat_to_zone(
        &self,
        pos: Position,
        zone: &crate::game::ZoneBounds,
        walls: &HashSet<Position>,
        config: &GameConfig,
    ) -> Option<Move> {
        let rows = config.rows as i32;
        let cols = config.cols as i32;

        // Find direction that moves us toward zone center
        let mut best_dir = None;
        let mut best_reduction = i32::MIN;

        for dir in Direction::all() {
            let next = pos.move_toward(dir, rows, cols);

            if walls.contains(&next) {
                continue;
            }

            let current_dist = pos.distance2(&zone.center, rows, cols);
            let new_dist = next.distance2(&zone.center, rows, cols);

            if new_dist < current_dist {
                let reduction = (current_dist - new_dist) as i32;
                if reduction > best_reduction {
                    best_reduction = reduction;
                    best_dir = Some(dir);
                }
            }
        }

        best_dir.map(|dir| Move {
            position: pos,
            direction: dir,
        })
    }

    /// Find enemy in kill band (danger zone near zone edge)
    fn find_enemy_in_kill_band(
        &self,
        my_pos: Position,
        enemy_bots: &[&VisibleBot],
        zone: &crate::game::ZoneBounds,
        inner: i32,
        outer: i32,
        config: &GameConfig,
    ) -> Option<Position> {
        let rows = config.rows as i32;
        let cols = config.cols as i32;

        enemy_bots
            .iter()
            .filter(|bot| {
                let dist = bot.position.distance2(&zone.center, rows, cols) as f64;
                let dist_from_center = dist.sqrt();
                dist_from_center >= (inner as f64) && dist_from_center <= (outer as f64)
            })
            .min_by_key(|bot| my_pos.distance2(&bot.position, rows, cols))
            .map(|bot| bot.position)
    }

    /// Block escape route by positioning between enemy and zone center
    fn block_escape_route(
        &self,
        my_pos: Position,
        enemy_pos: Position,
        zone: &crate::game::ZoneBounds,
        walls: &HashSet<Position>,
        config: &GameConfig,
    ) -> Option<Move> {
        let rows = config.rows as i32;
        let cols = config.cols as i32;

        // Calculate ideal blocking position: between enemy and zone center
        // This blocks the enemy's escape route inward
        let dr = zone.center.row as f64 - enemy_pos.row as f64;
        let dc = zone.center.col as f64 - enemy_pos.col as f64;
        let len = (dr * dr + dc * dc).sqrt();

        if len < 0.1 {
            return None; // Already at center
        }

        // Unit vector toward center
        let ur = dr / len;
        let uc = dc / len;

        // Ideal blocking position is one step toward center from enemy
        let ideal_row = enemy_pos.row as f64 + ur;
        let ideal_col = enemy_pos.col as f64 + uc;

        // Find best move toward ideal position
        let mut best_dir = None;
        let mut best_dist = f64::MAX;

        for dir in Direction::all() {
            let next = my_pos.move_toward(dir, rows, cols);

            if walls.contains(&next) {
                continue;
            }

            let dr = ideal_row - next.row as f64;
            let dc = ideal_col - next.col as f64;
            let dist = (dr * dr + dc * dc).sqrt();

            if dist < best_dist {
                best_dist = dist;
                best_dir = Some(dir);
            }
        }

        best_dir.map(|dir| Move {
            position: my_pos,
            direction: dir,
        })
    }

    /// Apply pressure by advancing toward enemies or zone edge
    fn advance_pressure(
        &self,
        pos: Position,
        enemy_positions: &HashSet<Position>,
        zone: &crate::game::ZoneBounds,
        walls: &HashSet<Position>,
        config: &GameConfig,
    ) -> Option<Move> {
        let rows = config.rows as i32;
        let cols = config.cols as i32;

        // If enemies visible, advance toward nearest one
        if let Some(&nearest_enemy) = enemy_positions
            .iter()
            .min_by_key(|e| pos.distance2(e, rows, cols))
        {
            let mut best_dir = None;
            let mut best_dist = u32::MAX;

            for dir in Direction::all() {
                let next = pos.move_toward(dir, rows, cols);

                if walls.contains(&next) {
                    continue;
                }

                let dist = next.distance2(&nearest_enemy, rows, cols);
                if dist < best_dist {
                    best_dist = dist;
                    best_dir = Some(dir);
                }
            }

            if let Some(dir) = best_dir {
                return Some(Move {
                    position: pos,
                    direction: dir,
                });
            }
        }

        // No enemies visible - move toward zone edge to apply pressure to anything hiding there
        let target_radius = (zone.radius.saturating_sub(3) as i32).max(0);
        let target_dist2 = (target_radius * target_radius) as u32;

        let current_dist2 = pos.distance2(&zone.center, rows, cols);

        if current_dist2 < target_dist2 {
            // We're too close to center - move outward
            let mut best_dir = None;
            let mut best_increase = i32::MIN;

            for dir in Direction::all() {
                let next = pos.move_toward(dir, rows, cols);

                if walls.contains(&next) {
                    continue;
                }

                let new_dist2 = next.distance2(&zone.center, rows, cols);
                let increase = (new_dist2 as i32 - current_dist2 as i32);

                if increase > best_increase {
                    best_increase = increase;
                    best_dir = Some(dir);
                }
            }

            best_dir.map(|dir| Move {
                position: pos,
                direction: dir,
            })
        } else {
            // Hold position near optimal pressure point
            None
        }
    }

    /// Fallback strategy when no zone is active
    fn defensive_fallback(
        &self,
        my_bots: &[&VisibleBot],
        enemy_positions: &HashSet<Position>,
        walls: &HashSet<Position>,
        config: &GameConfig,
    ) -> Vec<Move> {
        let rows = config.rows as i32;
        let cols = config.cols as i32;
        let mut moves = Vec::new();

        // Spread out and defend - move toward enemies if visible, otherwise spread
        for bot in my_bots {
            if let Some(&nearest_enemy) = enemy_positions
                .iter()
                .min_by_key(|e| bot.position.distance2(e, rows, cols))
            {
                let mut best_dir = None;
                let mut best_dist = u32::MAX;

                for dir in Direction::all() {
                    let next = bot.position.move_toward(dir, rows, cols);

                    if walls.contains(&next) || enemy_positions.contains(&next) {
                        continue;
                    }

                    let dist = next.distance2(&nearest_enemy, rows, cols);
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
        }

        moves
    }
}

impl Default for ZoneDriverStrategy {
    fn default() -> Self {
        Self::new()
    }
}
