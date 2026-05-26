package main

// ComputeMoves calculates moves for all visible bots.
// Implement your strategy here.
func ComputeMoves(state *VisibleState) []Move {
	moves := make([]Move, 0)

	// TODO: Implement your strategy here
	// This stub holds all bots in place
	for _, bot := range state.Bots {
		if bot.Owner == state.You.ID {
			moves = append(moves, Move{
				Position:  bot.Position,
				Direction: DirNone, // Hold position
			})
		}
	}

	return moves
}
