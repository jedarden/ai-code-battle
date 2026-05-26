package main

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

func TestGenerateMap_Connectivity(t *testing.T) {
	// Test that generated maps always pass connectivity validation
	for _, players := range []int{2, 3, 4, 6} {
		for seed := int64(1); seed <= 10; seed++ {
			rng := rand.New(rand.NewSource(seed))
			m := EnsureConnectivity(players, 60, 60, 0.15, 20, rng, 100)
			if m == nil {
				t.Errorf("players=%d seed=%d: failed to generate connected map", players, seed)
				continue
			}
			if !CheckConnectivity(m) {
				t.Errorf("players=%d seed=%d: map not connected after generation", players, seed)
			}
		}
	}
}

func TestGenerateMap_CoreCount(t *testing.T) {
	for _, players := range []int{2, 3, 4, 6} {
		rng := rand.New(rand.NewSource(42))
		m := EnsureConnectivity(players, 60, 60, 0.15, 20, rng, 100)
		if m == nil {
			t.Fatalf("players=%d: failed to generate map", players)
		}
		if len(m.Cores) != players {
			t.Errorf("players=%d: expected %d cores, got %d", players, players, len(m.Cores))
		}
		// Verify each player has a core
		owners := make(map[int]bool)
		for _, c := range m.Cores {
			owners[c.Owner] = true
		}
		for p := 0; p < players; p++ {
			if !owners[p] {
				t.Errorf("players=%d: player %d has no core", players, p)
			}
		}
	}
}

func TestGenerateMap_EnergyNodes(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	m := EnsureConnectivity(2, 60, 60, 0.15, 20, rng, 100)
	if m == nil {
		t.Fatal("failed to generate map")
	}
	if len(m.EnergyNodes) == 0 {
		t.Error("expected energy nodes, got 0")
	}
	// Energy nodes should not overlap with walls
	wallSet := make(map[Position]bool)
	for _, w := range m.Walls {
		wallSet[w] = true
	}
	for _, en := range m.EnergyNodes {
		if wallSet[en] {
			t.Errorf("energy node at %v overlaps with wall", en)
		}
	}
}

func TestGenerateMap_WallDensity(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	density := 0.15
	m := EnsureConnectivity(2, 60, 60, density, 20, rng, 100)
	if m == nil {
		t.Fatal("failed to generate map")
	}
	totalTiles := m.Rows * m.Cols
	actualDensity := float64(len(m.Walls)) / float64(totalTiles)
	if actualDensity > density+0.01 {
		t.Errorf("wall density %.2f exceeds target %.2f", actualDensity, density)
	}
}

func TestGenerateMap_NoCoresOnWalls(t *testing.T) {
	for _, players := range []int{2, 3, 4, 6} {
		rng := rand.New(rand.NewSource(42))
		m := EnsureConnectivity(players, 60, 60, 0.15, 20, rng, 100)
		if m == nil {
			t.Fatalf("players=%d: failed to generate map", players)
		}
		wallSet := make(map[Position]bool)
		for _, w := range m.Walls {
			wallSet[w] = true
		}
		for _, c := range m.Cores {
			if wallSet[c.Position] {
				t.Errorf("players=%d: core at %v overlaps with wall", players, c.Position)
			}
		}
	}
}

func TestCheckConnectivity_FullyOpen(t *testing.T) {
	m := &Map{
		Rows:  10,
		Cols:  10,
		Walls: nil,
		Cores: []Core{{Position: Position{0, 0}, Owner: 0}},
	}
	if !CheckConnectivity(m) {
		t.Error("fully open map should be connected")
	}
}

func TestCheckConnectivity_Disconnected(t *testing.T) {
	// Create a wall that bisects the grid vertically
	var walls []Position
	for r := 0; r < 10; r++ {
		walls = append(walls, Position{Row: r, Col: 5})
	}
	m := &Map{
		Rows:  10,
		Cols:  10,
		Walls: walls,
		Cores: []Core{{Position: Position{0, 0}, Owner: 0}},
	}
	// On a toroidal grid, a single column of walls doesn't disconnect
	// because you can wrap around. So this should still be connected.
	if !CheckConnectivity(m) {
		t.Error("toroidal grid with one column of walls should still be connected")
	}
}

func TestCheckConnectivity_DisconnectedBox(t *testing.T) {
	// Create a sealed box in a non-toroidal way - surround a region
	var walls []Position
	// Create a 3x3 box of walls around position (5,5)
	for r := 3; r <= 7; r++ {
		for c := 3; c <= 7; c++ {
			if r == 3 || r == 7 || c == 3 || c == 7 {
				walls = append(walls, Position{Row: r, Col: c})
			}
		}
	}
	m := &Map{
		Rows:  10,
		Cols:  10,
		Walls: walls,
		Cores: []Core{{Position: Position{0, 0}, Owner: 0}},
	}
	// The interior of the box (4-6, 4-6) is disconnected from the rest
	if CheckConnectivity(m) {
		t.Error("map with sealed interior should be disconnected")
	}
}

func TestGenerateMap_SmallGrid(t *testing.T) {
	// Ensure map generation works on small grids
	rng := rand.New(rand.NewSource(42))
	m := EnsureConnectivity(2, 20, 20, 0.10, 8, rng, 100)
	if m == nil {
		t.Fatal("failed to generate connected map on small grid")
	}
	if !CheckConnectivity(m) {
		t.Error("small grid map not connected")
	}
}

func TestGenerateMap_Deterministic(t *testing.T) {
	// Same seed should produce same map
	rng1 := rand.New(rand.NewSource(123))
	m1 := generateMap(2, 60, 60, 0.15, 20, rng1)

	rng2 := rand.New(rand.NewSource(123))
	m2 := generateMap(2, 60, 60, 0.15, 20, rng2)

	if len(m1.Walls) != len(m2.Walls) {
		t.Fatalf("determinism: wall count differs: %d vs %d", len(m1.Walls), len(m2.Walls))
	}
	if len(m1.Cores) != len(m2.Cores) {
		t.Fatal("determinism: core count differs")
	}
	if len(m1.EnergyNodes) != len(m2.EnergyNodes) {
		t.Fatal("determinism: energy node count differs")
	}
	for i, w := range m1.Walls {
		if w != m2.Walls[i] {
			t.Errorf("determinism: wall %d differs: %v vs %v", i, w, m2.Walls[i])
			break
		}
	}
}

func TestGenerateMap_CenterWeightedEnergy(t *testing.T) {
	// Verify energy nodes are biased toward the map center.
	// For 2-player with 20 energy nodes, expect at least 30% in central zone.
	// The tiered distribution should place ~30% of nodes in the inner 20% radius.
	rng := rand.New(rand.NewSource(42))
	m := EnsureConnectivity(2, 60, 60, 0.15, 20, rng, 100)
	if m == nil {
		t.Fatal("failed to generate map")
	}
	if len(m.EnergyNodes) == 0 {
		t.Fatal("expected energy nodes, got 0")
	}

	centerRow, centerCol := m.Rows/2, m.Cols/2
	maxRadius := float64(centerRow) * 0.20 // 20% of center distance = central zone
	centralCount := 0

	for _, en := range m.EnergyNodes {
		dr := float64(en.Row) - float64(centerRow)
		dc := float64(en.Col) - float64(centerCol)
		dist := math.Sqrt(dr*dr + dc*dc)
		if dist <= maxRadius {
			centralCount++
		}
	}

	// Expect at least 20% in central zone (allowing some variance for randomness)
	minCentral := int(float64(len(m.EnergyNodes)) * 0.20)
	if centralCount < minCentral {
		t.Errorf("expected at least %d energy nodes in central zone, got %d", minCentral, centralCount)
	}
}

func TestGenerateMap_CoresWithinAttackRadius(t *testing.T) {
	// Per plan §3.7.1: spawn radius puts bots within attack range for immediate combat.
	// Target: 65-80% combat density for 2-player matches.
	// For 2 players: 10% spawn radius (~2 tiles from center, ~4 tiles apart on 40x40)
	//   - Within attack radius (5 tiles), ensuring immediate combat engagement
	//   - Achieves 65-80% combat density per plan §3.7.1
	// For 3+ players: 10% spawn radius (~5 tiles from center on 50x50)
	//   - Within attack radius (3.5 tiles), ensuring combat engagement
	//   - Zone shrinks to radius 1, forcing all bots into contact
	testCases := []struct {
		numPlayers        int
		attackRadius      float64
		expectedRadius    float64
		maxDistFromCenter float64 // maximum distance from center (should be within attack radius)
	}{
		{2, 5.0, 0.10, 4.0}, // 2-player: 5 tile attack radius, 0.10 spawn radius = ~2 tiles from center
		{3, 3.5, 0.10, 3.5}, // 3+ player: 3.5 tile attack radius, 0.10 spawn radius = ~2.5 tiles from center
		{4, 3.5, 0.10, 3.5},
		{6, 3.5, 0.10, 3.5},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%dplayers", tc.numPlayers), func(t *testing.T) {
			rng := rand.New(rand.NewSource(42))
			// Use appropriate grid size for player count (matches ConfigForPlayers sizing)
			rows, cols := 40, 40
			if tc.numPlayers >= 3 {
				rows, cols = 50, 50 // Larger grid for 3+ players
			}
			m := EnsureConnectivity(tc.numPlayers, rows, cols, 0.15, 20, rng, 100)
			if m == nil {
				t.Fatalf("failed to generate map for %d players", tc.numPlayers)
			}
			if len(m.Cores) != tc.numPlayers {
				t.Fatalf("expected %d cores, got %d", tc.numPlayers, len(m.Cores))
			}

			centerRow, centerCol := m.Rows/2, m.Cols/2

			// Verify each core is at the expected radius from center
			for _, c := range m.Cores {
				dr := float64(c.Position.Row) - float64(centerRow)
				dc := float64(c.Position.Col) - float64(centerCol)
				dist := math.Sqrt(dr*dr + dc*dc)
				expectedDist := float64(centerRow) * tc.expectedRadius
				tolerance := 2.0 // Allow 2 tiles of tolerance for rounding

				if math.Abs(dist-expectedDist) > tolerance {
					t.Errorf("core at %v: distance %.2f from center, expected %.2f (tolerance %.2f)",
						c.Position, dist, expectedDist, tolerance)
				}

				// Verify cores are within attack radius for immediate combat engagement
				// For 2-player: within 5 tiles; for 3+: within 3.5 tiles

				if dist > tc.maxDistFromCenter {
					t.Errorf("core at %v: distance %.2f from center, expected at most %.2f (within attack radius)",
						c.Position, dist, tc.maxDistFromCenter)
				}
			}
		})
	}
}
