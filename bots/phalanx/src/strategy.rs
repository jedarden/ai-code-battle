//! Phalanx strategy: tight formation combat.
//!
//! All units move as a coordinated group, maximizing local firepower.
//! - Computes group centroid each tick using circular mean (toroidal-aware)
//! - Each unit maintains a fixed offset from centroid (hexagonal packing)
//! - Group advances toward nearest enemy concentration
//! - If formation breaks (units >3 cells from centroid), rally before advancing
//! - Spawned units join the back of the formation

use crate::game::{Direction, GameConfig, GameState, Move, Position};
use std::collections::{HashMap, HashSet};

/// Maximum allowed mean squared distance from centroid before rally mode
const FORMATION_RADIUS2: u32 = 9; // 3 cells squared
/// Bonus weight for advancing toward enemies
const ADVANCE_WEIGHT: f64 = 10.0;
/// Penalty per unit distance from formation slot
const FORMATION_WEIGHT: f64 = 8.0;
/// Bonus for being within attack range of an enemy
const ATTACK_RANGE_BONUS: f64 = 50.0;

pub struct PhalanxStrategy {
    /// Persistent centroid estimate (smoothed across turns for stability)
    centroid: Option<Position>,
    /// Known enemy positions from last turn (for tracking movement)
    last_enemy_positions: HashSet<Position>,
}

impl PhalanxStrategy {
    pub fn new() -> Self {
        Self {
            centroid: None,
            last_enemy_positions: HashSet::new(),
        }
    }

    pub fn compute_moves(&mut self, state: &GameState) -> Vec<Move> {
        let my_id = state.you.id;
        let config = &state.config;
        let rows = config.rows as i32;
        let cols = config.cols as i32;

        // Separate my bots from enemies
        let (my_bots, enemy_bots): (Vec<_>, Vec<_>) =
            state.bots.iter().partition(|b| b.owner == my_id);

        if my_bots.is_empty() {
            return vec![];
        }

        let my_positions: Vec<Position> = my_bots.iter().map(|b| b.position).collect();
        let enemy_positions: Vec<Position> = enemy_bots.iter().map(|b| b.position).collect();

        // Build wall and enemy lookups
        let walls: HashSet<Position> = state.walls.iter().copied().collect();
        let enemy_set: HashSet<Position> = enemy_positions.iter().copied().collect();

        // Compute group centroid using circular mean
        let centroid = circular_mean(&my_positions, rows, cols);

        // Smooth centroid with previous value for stability
        let centroid = if let Some(prev) = self.centroid {
            smooth_centroid(&prev, &centroid, rows, cols)
        } else {
            centroid
        };
        self.centroid = Some(centroid);

        // Check formation cohesion — are units within formation radius?
        let mean_dist = mean_distance2_from(&my_positions, &centroid, rows, cols);
        let rallying = mean_dist > FORMATION_RADIUS2;

        // Compute enemy centroid for advance target
        let enemy_centroid = if !enemy_positions.is_empty() {
            Some(circular_mean(&enemy_positions, rows, cols))
        } else {
            // No enemies visible — advance toward map center
            Some(Position {
                row: rows / 2,
                col: cols / 2,
            })
        };

        // Generate hexagonal formation slots around centroid
        let slots = generate_formation_slots(&centroid, my_positions.len(), rows, cols);

        // Assign bots to slots (greedy nearest-neighbor matching)
        let assignments = assign_slots(&my_positions, &slots, rows, cols);

        // Track claimed destinations to avoid self-collision
        let mut claimed: HashSet<Position> = HashSet::new();

        let mut moves = Vec::with_capacity(my_bots.len());

        for (_bot, bot_pos) in my_bots.iter().zip(my_positions.iter()) {
            let target_slot = assignments.get(bot_pos).copied();

            let advance_target = if rallying {
                // Rally mode: move toward centroid, not enemy
                centroid
            } else {
                // Advance mode: move toward enemy concentration
                enemy_centroid.unwrap_or(centroid)
            };

            if let Some(dir) = self.compute_bot_move(
                *bot_pos,
                &target_slot,
                &advance_target,
                &centroid,
                &enemy_set,
                &walls,
                &claimed,
                rallying,
                config,
            ) {
                let dest = bot_pos.move_toward(dir, rows, cols);
                claimed.insert(dest);
                moves.push(Move {
                    position: *bot_pos,
                    direction: dir,
                });
            } else {
                // Hold position
                claimed.insert(*bot_pos);
            }
        }

        // Update enemy tracking
        self.last_enemy_positions = enemy_set;

        moves
    }

    fn compute_bot_move(
        &self,
        bot_pos: Position,
        slot: &Option<Position>,
        advance_target: &Position,
        centroid: &Position,
        enemies: &HashSet<Position>,
        walls: &HashSet<Position>,
        claimed: &HashSet<Position>,
        rallying: bool,
        config: &GameConfig,
    ) -> Option<Direction> {
        let rows = config.rows as i32;
        let cols = config.cols as i32;

        let mut best_dir: Option<Direction> = None;
        let mut best_score = f64::NEG_INFINITY;

        for dir in Direction::all() {
            let dest = bot_pos.move_toward(dir, rows, cols);

            // Hard constraints: can't move into walls or enemies
            if walls.contains(&dest) || enemies.contains(&dest) {
                continue;
            }

            // Avoid self-collision
            if claimed.contains(&dest) {
                continue;
            }

            let mut score = 0.0;

            // Formation cohesion: move toward assigned slot
            if let Some(slot_pos) = slot {
                let dist_to_slot = dest.distance2(slot_pos, rows, cols) as f64;
                let current_dist_to_slot = bot_pos.distance2(slot_pos, rows, cols) as f64;
                score += (current_dist_to_slot - dist_to_slot) * FORMATION_WEIGHT;
            }

            // Stay close to centroid
            let dist_to_centroid = dest.distance2(centroid, rows, cols) as f64;
            let current_dist_centroid = bot_pos.distance2(centroid, rows, cols) as f64;
            score += (current_dist_centroid - dist_to_centroid) * (FORMATION_WEIGHT * 0.3);

            // Advance toward target (enemy or rally point)
            let dist_to_target = dest.distance2(advance_target, rows, cols) as f64;
            let current_dist_target = bot_pos.distance2(advance_target, rows, cols) as f64;

            if rallying {
                // During rally, heavily weight closing distance to centroid
                score += (current_dist_target - dist_to_target) * ADVANCE_WEIGHT * 2.0;
            } else {
                score += (current_dist_target - dist_to_target) * ADVANCE_WEIGHT;
            }

            // Bonus for being in attack range of enemies (only when not rallying)
            if !rallying {
                for enemy_pos in enemies.iter() {
                    let dist = dest.distance2(enemy_pos, rows, cols);
                    if dist <= config.attack_radius2 {
                        score += ATTACK_RANGE_BONUS;
                    }
                }
            }

            if score > best_score {
                best_score = score;
                best_dir = Some(dir);
            }
        }

        best_dir
    }
}

impl Default for PhalanxStrategy {
    fn default() -> Self {
        Self::new()
    }
}

/// Circular mean for toroidal coordinates — mathematically correct
/// center-of-mass on a wrapping grid.
fn circular_mean(positions: &[Position], rows: i32, cols: i32) -> Position {
    if positions.is_empty() {
        return Position {
            row: rows / 2,
            col: cols / 2,
        };
    }

    let row_scale = 2.0 * std::f64::consts::PI / rows as f64;
    let col_scale = 2.0 * std::f64::consts::PI / cols as f64;
    let n = positions.len() as f64;

    let mut sum_sin_row = 0.0_f64;
    let mut sum_cos_row = 0.0_f64;
    let mut sum_sin_col = 0.0_f64;
    let mut sum_cos_col = 0.0_f64;

    for pos in positions {
        sum_sin_row += (pos.row as f64 * row_scale).sin();
        sum_cos_row += (pos.row as f64 * row_scale).cos();
        sum_sin_col += (pos.col as f64 * col_scale).sin();
        sum_cos_col += (pos.col as f64 * col_scale).cos();
    }

    let avg_row = (sum_sin_row / n).atan2(sum_cos_row / n) / row_scale;
    let avg_col = (sum_sin_col / n).atan2(sum_cos_col / n) / col_scale;

    Position {
        row: ((avg_row % rows as f64 + rows as f64) % rows as f64).round() as i32,
        col: ((avg_col % cols as f64 + cols as f64) % cols as f64).round() as i32,
    }
}

/// Smooth centroid by blending with previous value (70% new, 30% old).
fn smooth_centroid(prev: &Position, current: &Position, rows: i32, cols: i32) -> Position {
    let (dr, dc) = prev.delta_to(current, rows, cols);
    let blend_dr = dr as f64 * 0.7;
    let blend_dc = dc as f64 * 0.7;
    Position {
        row: ((prev.row as f64 + blend_dr).round() as i32).rem_euclid(rows),
        col: ((prev.col as f64 + blend_dc).round() as i32).rem_euclid(cols),
    }
}

/// Mean squared distance from a set of positions to a reference point.
fn mean_distance2_from(positions: &[Position], center: &Position, rows: i32, cols: i32) -> u32 {
    if positions.is_empty() {
        return 0;
    }
    let total: u32 = positions
        .iter()
        .map(|p| p.distance2(center, rows, cols))
        .sum();
    total / positions.len() as u32
}

/// Generate hexagonal packing formation slots around a centroid.
/// Produces `count` positions in a tight hex pattern.
fn generate_formation_slots(centroid: &Position, count: usize, rows: i32, cols: i32) -> Vec<Position> {
    if count == 0 {
        return vec![];
    }

    let mut slots = vec![*centroid];

    if count == 1 {
        return slots;
    }

    // Hex ring expansion: generate slots in concentric hex rings
    // Ring 0: center (1 slot)
    // Ring 1: 6 slots at distance ~1.4
    // Ring 2: 12 slots at distance ~2.8
    // etc.
    let mut ring = 1;
    while slots.len() < count {
        let ring_slots = hex_ring(ring);
        for (dr, dc) in ring_slots {
            if slots.len() >= count {
                break;
            }
            let r = (centroid.row + dr).rem_euclid(rows);
            let c = (centroid.col + dc).rem_euclid(cols);
            slots.push(Position { row: r, col: c });
        }
        ring += 1;
        if ring > 20 {
            break;
        }
    }

    slots
}

/// Generate the 6*ring offsets for a hex ring at distance `ring`.
/// Uses axial hex coordinates converted to offset coordinates.
fn hex_ring(ring: i32) -> Vec<(i32, i32)> {
    if ring == 0 {
        return vec![(0, 0)];
    }

    // Six hex directions as (dq, dr) in axial coordinates
    let hex_dirs: [(i32, i32); 6] = [
        (1, 0),
        (0, 1),
        (-1, 1),
        (-1, 0),
        (0, -1),
        (1, -1),
    ];

    // Convert axial to offset: offset_row = dr, offset_col = dq + dr/2
    let mut result = Vec::with_capacity(6 * ring as usize);

    // Start at one corner of the ring
    let mut q = ring as i32;
    let mut r: i32 = 0;

    for &(dq, dr) in &hex_dirs {
        for _ in 0..ring {
            let offset_row = r;
            let offset_col = q + r / 2;
            result.push((offset_row, offset_col));
            q += dq;
            r += dr;
        }
    }

    result
}

/// Greedy nearest-neighbor assignment of bots to formation slots.
fn assign_slots(
    bots: &[Position],
    slots: &[Position],
    rows: i32,
    cols: i32,
) -> HashMap<Position, Position> {
    let mut assignments = HashMap::with_capacity(bots.len());
    let mut used_slots: HashSet<usize> = HashSet::new();

    // Sort bots by distance to their nearest unused slot (greedy priority)
    let bot_indices: Vec<usize> = (0..bots.len()).collect();
    // Simple greedy: assign each bot to nearest unused slot
    for &bi in &bot_indices {
        let bot = bots[bi];
        let mut best_slot_idx = 0;
        let mut best_dist = u32::MAX;

        for (si, slot) in slots.iter().enumerate() {
            if used_slots.contains(&si) {
                continue;
            }
            let d = bot.distance2(slot, rows, cols);
            if d < best_dist {
                best_dist = d;
                best_slot_idx = si;
            }
        }

        used_slots.insert(best_slot_idx);
        if best_slot_idx < slots.len() {
            assignments.insert(bot, slots[best_slot_idx]);
        }
    }

    assignments
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_circular_mean_single() {
        let pos = Position { row: 10, col: 20 };
        let center = circular_mean(&[pos], 60, 60);
        assert_eq!(center.row, 10);
        assert_eq!(center.col, 20);
    }

    #[test]
    fn test_circular_mean_wrapping() {
        // Two positions near the wrap boundary should average near the boundary
        let positions = vec![
            Position { row: 2, col: 30 },
            Position { row: 58, col: 30 },
        ];
        let center = circular_mean(&positions, 60, 60);
        // Should be near row 0 (wrap), not row 30
        assert!(center.row <= 4 || center.row >= 56);
    }

    #[test]
    fn test_formation_slots_count() {
        let centroid = Position { row: 30, col: 30 };
        let slots = generate_formation_slots(&centroid, 10, 60, 60);
        assert_eq!(slots.len(), 10);
    }

    #[test]
    fn test_formation_slots_single() {
        let centroid = Position { row: 30, col: 30 };
        let slots = generate_formation_slots(&centroid, 1, 60, 60);
        assert_eq!(slots.len(), 1);
        assert_eq!(slots[0], centroid);
    }

    #[test]
    fn test_mean_distance_empty() {
        let center = Position { row: 30, col: 30 };
        assert_eq!(mean_distance2_from(&[], &center, 60, 60), 0);
    }

    #[test]
    fn test_mean_distance_coherent() {
        let center = Position { row: 30, col: 30 };
        let positions = vec![
            Position { row: 30, col: 30 },
            Position { row: 31, col: 30 },
        ];
        let mean = mean_distance2_from(&positions, &center, 60, 60);
        assert!(mean < FORMATION_RADIUS2);
    }

    #[test]
    fn test_mean_distance_broken() {
        let center = Position { row: 30, col: 30 };
        let positions = vec![
            Position { row: 30, col: 30 },
            Position { row: 40, col: 40 },
        ];
        let mean = mean_distance2_from(&positions, &center, 60, 60);
        assert!(mean > FORMATION_RADIUS2);
    }

    #[test]
    fn test_hex_ring_1() {
        let ring = hex_ring(1);
        assert_eq!(ring.len(), 6);
    }

    #[test]
    fn test_hex_ring_2() {
        let ring = hex_ring(2);
        assert_eq!(ring.len(), 12);
    }

    #[test]
    fn test_smooth_centroid_stability() {
        let prev = Position { row: 30, col: 30 };
        let current = Position { row: 32, col: 31 };
        let smoothed = smooth_centroid(&prev, &current, 60, 60);
        // Smoothed should be between prev and current but closer to current
        assert!(smoothed.row > prev.row);
        assert!(smoothed.col > prev.col);
    }
}
