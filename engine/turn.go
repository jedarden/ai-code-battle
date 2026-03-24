package engine

// TurnPhase represents a phase of turn execution.
type TurnPhase int

const (
	PhaseMove TurnPhase = iota
	PhaseCombat
	PhaseCapture
	PhaseCollect
	PhaseSpawn
	PhaseEnergyTick
	PhaseEndgame
)

// ExecuteTurn executes a single turn of the game.
// It assumes moves have already been submitted via SubmitMove.
func (gs *GameState) ExecuteTurn() *MatchResult {
	gs.Turn++

	// Phase: MOVE - execute valid movement orders
	gs.executeMoves()

	// Phase: COMBAT - resolve focus-fire algorithm
	gs.executeCombat()

	// Phase: CAPTURE - enemy bots on undefended cores raze them
	gs.executeCaptures()

	// Phase: COLLECT - uncontested energy is collected
	gs.executeCollection()

	// Phase: SPAWN - players with enough energy spawn bots at cores
	gs.executeSpawns()

	// Phase: ENERGY_TICK - energy nodes on interval produce new energy
	gs.executeEnergyTick()

	// Phase: ENDGAME - check win conditions
	result := gs.checkWinConditions()

	return result
}

// executeMoves processes all submitted moves.
func (gs *GameState) executeMoves() {
	// First, compute intended destinations
	intended := make(map[int]Position) // bot ID -> intended position
	botsAtPos := make(map[Position][]*Bot) // position -> bots trying to move there

	for _, b := range gs.Bots {
		if !b.Alive {
			continue
		}

		move, hasMove := gs.Moves[b.ID]
		var dest Position
		if hasMove && move.Direction != DirNone {
			dest = gs.Grid.Move(b.Position, move.Direction)
			// Check if destination is passable
			if !gs.Grid.IsPassable(dest) {
				// Order ignored - stay in place
				dest = b.Position
			}
		} else {
			// No move - stay in place
			dest = b.Position
		}

		intended[b.ID] = dest
		botsAtPos[dest] = append(botsAtPos[dest], b)
	}

	// Process movements
	for _, b := range gs.Bots {
		if !b.Alive {
			continue
		}

		dest := intended[b.ID]

		// Check for collisions
		botsAtDest := botsAtPos[dest]
		if len(botsAtDest) > 1 {
			// Multiple bots trying to occupy same tile
			// Check if same owner (self-collision) or different owners (combat handled later)
			sameOwner := true
			for _, other := range botsAtDest {
				if other.Owner != b.Owner {
					sameOwner = false
					break
				}
			}

			if sameOwner {
				// Self-collision: all bots at this position die
				for _, other := range botsAtDest {
					gs.KillBot(other, "self_collision")
				}
				continue
			}
		}

		// Move to destination
		b.Position = dest
	}
}

// executeCombat resolves the focus-fire combat algorithm.
func (gs *GameState) executeCombat() {
	// For each bot, count enemies within attack radius
	enemyCounts := make(map[int]int) // bot ID -> enemy count
	botsInRadius := make(map[int][]*Bot) // bot ID -> enemies within radius

	for _, b := range gs.Bots {
		if !b.Alive {
			continue
		}

		var enemies []*Bot
		for _, e := range gs.Bots {
			if !e.Alive || e.ID == b.ID || e.Owner == b.Owner {
				continue
			}
			if gs.Grid.InRadius(b.Position, e.Position, gs.Config.AttackRadius2) {
				enemies = append(enemies, e)
			}
		}
		enemyCounts[b.ID] = len(enemies)
		botsInRadius[b.ID] = enemies
	}

	// Determine which bots die (simultaneous - use pre-computed counts)
	dead := make(map[int]bool)

	for _, b := range gs.Bots {
		if !b.Alive {
			continue
		}

		myEnemyCount := enemyCounts[b.ID]
		if myEnemyCount == 0 {
			continue // No enemies nearby, safe
		}

		// Check if any enemy has <= myEnemyCount enemies
		// Use the pre-computed enemy counts (not affected by simultaneous deaths)
		for _, e := range botsInRadius[b.ID] {
			theirEnemyCount := enemyCounts[e.ID]
			if myEnemyCount >= theirEnemyCount {
				// I die
				dead[b.ID] = true
				break
			}
		}
	}

	// Kill the dead bots
	for _, b := range gs.Bots {
		if dead[b.ID] {
			gs.KillBot(b, "combat")
		}
	}
}

// executeCaptures handles core capture mechanics.
func (gs *GameState) executeCaptures() {
	// Find bots on core tiles
	botsOnCores := make(map[int][]*Bot) // core index -> bots on it

	for ci, c := range gs.Cores {
		if !c.Active {
			continue
		}
		for _, b := range gs.Bots {
			if b.Alive && b.Position == c.Position {
				botsOnCores[ci] = append(botsOnCores[ci], b)
			}
		}
	}

	// Check each core for captures
	for ci, bots := range botsOnCores {
		c := gs.Cores[ci]
		if !c.Active {
			continue
		}

		// A core is defended if a bot of the owner is on it
		defended := false
		for _, b := range bots {
			if b.Owner == c.Owner {
				defended = true
				break
			}
		}

		if !defended {
			// Core is undefended - any enemy bot on it razes it
			for _, b := range bots {
				if b.Owner != c.Owner {
					// Capture!
					gs.captureCore(c, b.Owner)
					break // Only one capture per core per turn
				}
			}
		}
	}
}

// captureCore handles the capture of a core by a player.
func (gs *GameState) captureCore(c *Core, capturer int) {
	// Scoring: +2 to capturer, -1 to owner
	gs.Players[capturer].Score += 2
	if c.Owner < len(gs.Players) {
		gs.Players[c.Owner].Score--
	}

	// Raze the core
	c.Active = false

	gs.Events = append(gs.Events, Event{
		Type: EventCoreCaptured,
		Turn: gs.Turn,
		Details: map[string]interface{}{
			"core_pos":  c.Position,
			"old_owner": c.Owner,
			"new_owner": capturer,
		},
	})
}

// executeCollection handles energy collection.
func (gs *GameState) executeCollection() {
	// For each energy node with energy, check collection
	for _, en := range gs.Energy {
		if !en.HasEnergy {
			continue
		}

		// Find all adjacent bots
		var adjBots []*Bot
		for _, b := range gs.Bots {
			if !b.Alive {
				continue
			}
			// Adjacent means distance <= sqrt(2), i.e., distance^2 <= 2
			// Or on the tile (distance 0)
			d2 := gs.Grid.Distance2(b.Position, en.Position)
			if d2 <= 2 {
				adjBots = append(adjBots, b)
			}
		}

		if len(adjBots) == 0 {
			continue // No bots adjacent
		}

		// Check if multiple players are adjacent (contested)
		players := make(map[int]bool)
		for _, b := range adjBots {
			players[b.Owner] = true
		}

		if len(players) > 1 {
			// Contested - energy is destroyed
			en.HasEnergy = false
			en.Tick = 0
			continue
		}

		// Uncontested - collect energy
		playerID := adjBots[0].Owner
		if playerID < len(gs.Players) {
			gs.Players[playerID].Energy++
		}
		en.HasEnergy = false
		en.Tick = 0

		gs.Events = append(gs.Events, Event{
			Type: EventEnergyCollected,
			Turn: gs.Turn,
			Details: map[string]interface{}{
				"pos":    en.Position,
				"player": playerID,
			},
		})
	}
}

// executeSpawns handles bot spawning at active cores.
func (gs *GameState) executeSpawns() {
	// For each player, check if they can spawn
	for _, p := range gs.Players {
		if p.Energy < gs.Config.SpawnCost {
			continue
		}

		// Find active cores owned by this player that are unoccupied
		for _, c := range gs.Cores {
			if !c.Active || c.Owner != p.ID {
				continue
			}

			// Check if core is occupied
			occupied := false
			for _, b := range gs.Bots {
				if b.Alive && b.Position == c.Position {
					occupied = true
					break
				}
			}

			if !occupied && p.Energy >= gs.Config.SpawnCost {
				// Spawn a bot
				gs.SpawnBot(p.ID, c.Position)
				p.Energy -= gs.Config.SpawnCost
			}
		}
	}
}

// executeEnergyTick handles energy node spawning.
func (gs *GameState) executeEnergyTick() {
	for _, en := range gs.Energy {
		if en.HasEnergy {
			continue // Already has energy
		}

		en.Tick++
		if en.Tick >= gs.Config.EnergyInterval {
			en.HasEnergy = true
			en.Tick = 0
		}
	}
}

// checkWinConditions checks for game-ending conditions.
func (gs *GameState) checkWinConditions() *MatchResult {
	// Count living bots per player
	livingPlayers := gs.GetLivingPlayers()
	totalBots := gs.GetLivingBotCount()

	// Condition 1: Sole Survivor - only one player has living bots
	if len(livingPlayers) == 1 {
		winner := livingPlayers[0]
		bonus := 0
		// Bonus +2 per surviving enemy core
		for _, c := range gs.Cores {
			if c.Active && c.Owner != winner {
				bonus += 2
			}
		}
		gs.Players[winner].Score += bonus
		return gs.createResult(winner, "elimination")
	}

	// Condition 2: Annihilation - all players eliminated simultaneously
	if len(livingPlayers) == 0 {
		return gs.createResult(-1, "draw")
	}

	// Condition 3: Dominance - one player controls >=80% of all bots for 100 consecutive turns
	if totalBots > 0 {
		for _, p := range gs.Players {
			botCount := gs.GetPlayerLivingBotCount(p.ID)
			if float64(botCount) >= 0.8*float64(totalBots) {
				gs.Dominance[p.ID]++
				if gs.Dominance[p.ID] >= 100 {
					return gs.createResult(p.ID, "dominance")
				}
			} else {
				gs.Dominance[p.ID] = 0
			}
		}
	}

	// Condition 4: Turn Limit
	if gs.Turn >= gs.Config.MaxTurns {
		// Highest score wins, ties broken by energy collected, then bots alive
		winner := gs.findWinnerByScore()
		return gs.createResult(winner, "turns")
	}

	return nil // No winner yet
}

// createResult creates a match result.
func (gs *GameState) createResult(winner int, reason string) *MatchResult {
	scores := make([]int, len(gs.Players))
	energy := make([]int, len(gs.Players))
	botsAlive := make([]int, len(gs.Players))

	for i, p := range gs.Players {
		scores[i] = p.Score
		energy[i] = p.Energy
		botsAlive[i] = gs.GetPlayerLivingBotCount(p.ID)
	}

	return &MatchResult{
		Winner:    winner,
		Reason:    reason,
		Turns:     gs.Turn,
		Scores:    scores,
		Energy:    energy,
		BotsAlive: botsAlive,
	}
}

// findWinnerByScore finds the winner based on score, energy, and bot count.
func (gs *GameState) findWinnerByScore() int {
	bestPlayer := 0
	bestScore := gs.Players[0].Score
	bestEnergy := gs.Players[0].Energy
	bestBots := gs.GetPlayerLivingBotCount(0)

	for i, p := range gs.Players {
		score := p.Score
		energy := p.Energy
		bots := gs.GetPlayerLivingBotCount(i)

		// Compare by score first, then energy, then bots
		if score > bestScore ||
			(score == bestScore && energy > bestEnergy) ||
			(score == bestScore && energy == bestEnergy && bots > bestBots) {
			bestPlayer = i
			bestScore = score
			bestEnergy = energy
			bestBots = bots
		}
	}

	return bestPlayer
}
