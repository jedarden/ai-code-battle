package engine

import "sort"

// TurnPhase represents a phase of turn execution.
type TurnPhase int

const (
	PhaseMove TurnPhase = iota
	PhaseCombat
	PhaseZone
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

	// Phase: ZONE - shrinking zone kills bots outside
	gs.executeZone()

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
	intended := make(map[int]Position)     // bot ID -> intended position
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

// executeZone handles the shrinking zone (storm) that forces combat.
func (gs *GameState) executeZone() {
	if !gs.Config.ZoneEnabled {
		return
	}

	// Zone is now activated BEFORE getting moves (in RunMatch)
	// This allows bots to see the zone is active and react accordingly

	// Zone center is fixed at map center (set in NewGameState)
	// This forces bots toward the center as the zone shrinks, ensuring contact.

	// Check if zone should shrink (skip the turn zone starts)
	// The zone starts at turn ZoneStartTurn, so we skip shrinking on that turn
	if gs.ZoneActive && gs.Turn > gs.Config.ZoneStartTurn && (gs.Turn-gs.Config.ZoneStartTurn)%gs.Config.ZoneShrinkInterval == 0 {
		if gs.ZoneRadius > gs.Config.ZoneMinRadius {
			gs.ZoneRadius -= gs.Config.ZoneShrinkStep
			if gs.ZoneRadius < gs.Config.ZoneMinRadius {
				gs.ZoneRadius = gs.Config.ZoneMinRadius
			}
		}
		// Ensure zone radius is at least the minimum (handles overshoot from shrink step)
		if gs.ZoneRadius < gs.Config.ZoneMinRadius {
			gs.ZoneRadius = gs.Config.ZoneMinRadius
		}
	}

	// Kill bots outside the zone (only when zone is active)
	if !gs.ZoneActive {
		return
	}

	for _, b := range gs.Bots {
		if !b.Alive {
			continue
		}

		// Calculate distance from zone center (accounting for toroidal wrap)
		dist2 := gs.Grid.Distance2(b.Position, gs.ZoneCenter)
		if dist2 > gs.ZoneRadius*gs.ZoneRadius {
			// Mark bot as dead
			b.Alive = false
			gs.DeadBots = append(gs.DeadBots, b)

			if b.Owner < len(gs.Players) {
				gs.Players[b.Owner].BotCount--
			}

			// Emit zone_death event
			gs.Events = append(gs.Events, Event{
				Type: EventZoneDeath,
				Turn: gs.Turn,
				Details: map[string]interface{}{
					"bot_id":   b.ID,
					"owner":    b.Owner,
					"position": b.Position,
				},
			})
		}
	}
}

// setInitialZoneRadius sets the zone to a fixed initial radius based on map dimensions.
// The zone starts at a radius that forces bots inward immediately, ensuring
// contact is forced before energy farming dominates.
func (gs *GameState) setInitialZoneRadius() {
	// Calculate the distance from center to the nearest map edge
	// For a rectangular grid, this is the minimum of half-rows and half-cols
	halfRows := gs.Config.Rows / 2
	halfCols := gs.Config.Cols / 2
	distToEdge := halfRows
	if halfCols < distToEdge {
		distToEdge = halfCols
	}

	// Set initial zone radius to 90% of the distance from center to edge
	// This ensures all spawn positions (30% from center) are inside the zone
	// Zone shrinks 1 tile/turn, forcing bots toward center over time
	gs.ZoneRadius = (distToEdge * 90) / 100

	// Ensure minimum initial radius of 7 for very small maps
	if gs.ZoneRadius < 7 {
		gs.ZoneRadius = 7
	}
}

// executeCombat resolves the focus-fire combat algorithm.
func (gs *GameState) executeCombat() {
	// For each bot, count enemies within attack radius
	enemyCounts := make(map[int]int)     // bot ID -> enemy count
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

		// Focus-fire algorithm: enemies coordinate on the most vulnerable target
		// "Most vulnerable" = has the MOST enemies nearby (highest enemy count)
		// I die if I'm strictly more vulnerable than all my enemies
		// In 1v1 ties (equal enemy counts), neither dies - no numerical advantage to exploit
		mostVulnerable := true
		for _, e := range botsInRadius[b.ID] {
			theirEnemyCount := enemyCounts[e.ID]
			if theirEnemyCount >= myEnemyCount {
				// This enemy is equally or more vulnerable than me
				// Enemies won't focus-fire on me when there's an equally good target
				mostVulnerable = false
				break
			}
		}
		if mostVulnerable {
			// I'm strictly the most vulnerable, all enemies focus-fire on me
			dead[b.ID] = true
		}
	}

	// Kill the dead bots and emit combat_death events
	for _, b := range gs.Bots {
		if dead[b.ID] {
			b.Alive = false
			gs.DeadBots = append(gs.DeadBots, b)

			if b.Owner < len(gs.Players) {
				gs.Players[b.Owner].BotCount--
			}

			// Build killers array (enemies within attack radius)
			var killers []map[string]interface{}
			for _, e := range botsInRadius[b.ID] {
				killers = append(killers, map[string]interface{}{
					"bot_id":   e.ID,
					"owner":    e.Owner,
					"position": e.Position,
				})
				// Track combat deaths for the killer's player
				if e.Owner < len(gs.CombatDeaths) {
					gs.CombatDeaths[e.Owner]++
				}
					// Award score for the kill
					if e.Owner < len(gs.Players) {
						gs.Players[e.Owner].Score += gs.Config.KillScore
					}
			}

			gs.Events = append(gs.Events, Event{
				Type: EventCombatDeath,
				Turn: gs.Turn,
				Details: map[string]interface{}{
					"bot_id":   b.ID,
					"owner":    b.Owner,
					"position": b.Position,
					"killers":  killers,
				},
			})
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
// When multiple cores are eligible, the core idle longest spawns first
// (deterministic tiebreak: lowest core ID wins).
func (gs *GameState) executeSpawns() {
	// For each player, check if they can spawn
	for _, p := range gs.Players {
		if p.Energy < gs.Config.SpawnCost {
			continue
		}

		// Collect eligible cores: active, owned by this player, unoccupied
		var eligible []*Core
		for _, c := range gs.Cores {
			if !c.Active || c.Owner != p.ID {
				continue
			}
			occupied := false
			for _, b := range gs.Bots {
				if b.Alive && b.Position == c.Position {
					occupied = true
					break
				}
			}
			if !occupied {
				eligible = append(eligible, c)
			}
		}

		// Sort by (lastSpawnedTurn ASC, core ID ASC) — idle-longest first
		sortCoresByPriority(eligible)

		for _, c := range eligible {
			if p.Energy < gs.Config.SpawnCost {
				break
			}
			gs.SpawnBot(p.ID, c.Position)
			c.LastSpawnedTurn = gs.Turn
			p.Energy -= gs.Config.SpawnCost
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

	// Condition 4: Stalemate - no progress for 50 consecutive turns
	currentEnergy := 0
	for _, p := range gs.Players {
		currentEnergy += p.Energy
	}
	currentBots := gs.GetLivingBotCount()
	if currentEnergy != gs.LastTotalEnergy || currentBots != gs.LastTotalBots || len(gs.DeadBots) > 0 {
		gs.StalemateTurns = 0
		gs.LastTotalEnergy = currentEnergy
		gs.LastTotalBots = currentBots
	} else {
		gs.StalemateTurns++
	}
	if gs.StalemateTurns >= 50 {
		winner := gs.findWinnerByScore()
		return gs.createResult(winner, "stalemate")
	}

	// Condition 5: Turn Limit
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
		Winner:       winner,
		Reason:       reason,
		Turns:        gs.Turn,
		Scores:       scores,
		Energy:       energy,
		BotsAlive:    botsAlive,
		CombatDeaths: gs.CombatDeaths,
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

// sortCoresByPriority sorts cores by (LastSpawnedTurn ASC, ID ASC).
// The core idle longest (lowest LastSpawnedTurn) spawns first;
// equal idle time is broken by lower core ID.
func sortCoresByPriority(cores []*Core) {
	sort.Slice(cores, func(i, j int) bool {
		if cores[i].LastSpawnedTurn != cores[j].LastSpawnedTurn {
			return cores[i].LastSpawnedTurn < cores[j].LastSpawnedTurn
		}
		return cores[i].ID < cores[j].ID
	})
}
