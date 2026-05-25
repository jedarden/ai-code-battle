package engine

import (
	"math"
	"math/rand"
	"testing"
)

// TestSpawnRadiusOutsideZone verifies that bots spawn outside the final zone.
// Per plan §3.7.1, the zone forces combat by shrinking. Bots must start OUTSIDE
// the final safe zone so they are forced inward as the zone contracts.
func TestSpawnRadiusOutsideZone(t *testing.T) {
	tests := []struct {
		name           string
		numPlayers     int
		coresPerPlayer int
	}{
		{"2-player", 2, 1},
		{"2-player-2-cores", 2, 2},
		{"3-player", 3, 1},
		{"4-player", 4, 1},
		{"6-player", 6, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ConfigForPlayers(tt.numPlayers, tt.coresPerPlayer)
			gs := NewGameState(cfg, rand.New(rand.NewSource(42)))

			// Add players
			for range tt.numPlayers {
				gs.AddPlayer()
			}

			// Generate map using the match runner
			mr := NewMatchRunner(cfg, WithRNG(rand.New(rand.NewSource(42))))
			mr.generateMap(gs, tt.numPlayers)

			// Verify all spawn positions are outside the final zone
			center := Position{Row: cfg.Rows / 2, Col: cfg.Cols / 2}

			for _, bot := range gs.Bots {
				if !bot.Alive {
					continue
				}

				// Calculate distance from center
				dist2 := gs.Grid.Distance2(bot.Position, center)
				dist := math.Sqrt(float64(dist2))

				// Verify spawn distance > zone min radius
				if dist <= float64(cfg.ZoneMinRadius) {
					t.Errorf("Player %d bot spawned at distance %.1f from center, <= zone min radius %d (position: %v)",
						bot.Owner, dist, cfg.ZoneMinRadius, bot.Position)
				}
			}
		})
	}
}

// TestSpawnRadiusForcesCombat verifies that when zone shrinks to minimum,
// bots on opposite sides are within attack radius of each other.
func TestSpawnRadiusForcesCombat(t *testing.T) {
	tests := []struct {
		name           string
		numPlayers     int
		coresPerPlayer int
	}{
		{"2-player", 2, 1},
		{"3-player", 3, 1},
		{"4-player", 4, 1},
		{"6-player", 6, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ConfigForPlayers(tt.numPlayers, tt.coresPerPlayer)
			gs := NewGameState(cfg, rand.New(rand.NewSource(42)))

			// Add players
			for range tt.numPlayers {
				gs.AddPlayer()
			}

			// Generate map
			mr := NewMatchRunner(cfg, WithRNG(rand.New(rand.NewSource(42))))
			mr.generateMap(gs, tt.numPlayers)

			// Simulate zone shrinking to minimum
			// Bots would be forced toward center, ending at zone edge
			centerRow := cfg.Rows / 2
			centerCol := cfg.Cols / 2

			// Calculate where each player's bot would end up at zone edge
			// (simplified: project to zone edge along the same angle)
			attackRadius := math.Sqrt(float64(cfg.AttackRadius2))

			for _, bot := range gs.Bots {
				if !bot.Alive || bot.Owner != 0 {
					continue
				}

				// Vector from center to bot
				dr := float64(bot.Position.Row - centerRow)
				dc := float64(bot.Position.Col - centerCol)

				// Normalize to zone edge
				dist := math.Sqrt(dr*dr + dc*dc)
				if dist == 0 {
					continue
				}
				scale := float64(cfg.ZoneMinRadius) / dist

				// Check distance to other players' projected zone edge positions
				for _, other := range gs.Bots {
					if !other.Alive || other.Owner <= bot.Owner {
						continue
					}

					odr := float64(other.Position.Row - centerRow)
					odc := float64(other.Position.Col - centerCol)
					odist := math.Sqrt(odr*odr + odc*odc)
					if odist == 0 {
						continue
					}
					oscale := float64(cfg.ZoneMinRadius) / odist

					// Distance between zone edge projections
					er := centerRow + int(dr*scale)
					ec := centerCol + int(dc*scale)
					eor := centerRow + int(odr*oscale)
					eoc := centerCol + int(odc*oscale)

					// Toroidal distance
					gridDist2 := gs.Grid.Distance2(Position{Row: er, Col: ec}, Position{Row: eor, Col: eoc})
					gridDist := math.Sqrt(float64(gridDist2))

					if gridDist > attackRadius {
						t.Errorf("Players %d and %d: projected zone edge distance %.1f > attack radius %.1f (bot pos: %v, other pos: %v)",
							bot.Owner, other.Owner, gridDist, attackRadius, bot.Position, other.Position)
					}
				}
			}
		})
	}
}
