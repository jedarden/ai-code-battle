package main

import (
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
