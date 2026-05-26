use crate::types::{Move, VisibleState};

/// Compute moves for all visible bots.
/// Implement your strategy here.
pub fn compute_moves(state: &VisibleState) -> Vec<Move> {
    state.bots
        .iter()
        .filter(|bot| bot.owner == state.you.id)
        .map(|bot| Move {
            position: bot.position,
            direction: crate::types::Direction::None, // Hold position
        })
        .collect()
}
