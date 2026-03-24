// Command acb-mapgen generates symmetric maps for AI Code Battle.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"
)

// Map represents a generated map.
type Map struct {
	ID          string     `json:"id"`
	Players     int        `json:"players"`
	Rows        int        `json:"rows"`
	Cols        int        `json:"cols"`
	WallDensity float64    `json:"wall_density"`
	Walls       []Position `json:"walls"`
	Cores       []Core     `json:"cores"`
	EnergyNodes []Position `json:"energy_nodes"`
	Generated   time.Time  `json:"generated"`
}

// Position represents a grid coordinate.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Core represents a spawn point.
type Core struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
}

func main() {
	// Command-line flags
	players := flag.Int("players", 2, "Number of players (2, 3, 4, or 6)")
	rows := flag.Int("rows", 60, "Grid rows")
	cols := flag.Int("cols", 60, "Grid columns")
	wallDensity := flag.Float64("wall-density", 0.15, "Wall density (0.0-0.3)")
	energyNodes := flag.Int("energy-nodes", 20, "Energy nodes")
	seed := flag.Int64("seed", time.Now().UnixNano(), "Random seed")
	output := flag.String("output", "", "Output file (default: stdout)")
	maxAttempts := flag.Int("max-attempts", 100, "Max attempts to generate a connected map")
	help := flag.Bool("help", false, "Show help")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: acb-mapgen [options]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Generate a symmetric map for AI Code Battle.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "The generator ensures all passable tiles are reachable from\n")
		fmt.Fprintf(flag.CommandLine.Output(), "any core (full connectivity guarantee).\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Symmetry types:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  2 players: 180° rotational\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  3 players: 120° rotational\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  4 players: 90° rotational\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  6 players: 60° rotational\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Validate player count
	validPlayers := map[int]bool{2: true, 3: true, 4: true, 6: true}
	if !validPlayers[*players] {
		fmt.Fprintf(os.Stderr, "Error: invalid player count %d (must be 2, 3, 4, or 6)\n", *players)
		os.Exit(1)
	}

	// Validate wall density
	if *wallDensity < 0.05 || *wallDensity > 0.30 {
		fmt.Fprintf(os.Stderr, "Error: wall density must be between 0.05 and 0.30\n")
		os.Exit(1)
	}

	// Generate map with connectivity validation
	rng := rand.New(rand.NewSource(*seed))
	m := EnsureConnectivity(*players, *rows, *cols, *wallDensity, *energyNodes, rng, *maxAttempts)
	if m == nil {
		fmt.Fprintf(os.Stderr, "Error: failed to generate a connected map after %d attempts\n", *maxAttempts)
		fmt.Fprintf(os.Stderr, "Try reducing wall density or increasing max-attempts\n")
		os.Exit(1)
	}

	// Generate map ID
	m.ID = generateMapID(rng)
	m.Generated = time.Now().UTC()

	// Output
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal map: %v\n", err)
		os.Exit(1)
	}

	if *output != "" {
		if err := os.WriteFile(*output, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to write file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Map written to %s\n", *output)
	} else {
		fmt.Println(string(data))
	}
}

func generateMapID(rng *rand.Rand) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	return "map_" + string(b)
}

func generateMap(numPlayers, rows, cols int, wallDensity float64, numEnergyNodes int, rng *rand.Rand) *Map {
	m := &Map{
		Players:     numPlayers,
		Rows:        rows,
		Cols:        cols,
		WallDensity: wallDensity,
		Walls:       make([]Position, 0),
		Cores:       make([]Core, 0),
		EnergyNodes: make([]Position, 0),
	}

	centerRow := rows / 2
	centerCol := cols / 2

	// Helper to wrap position
	wrap := func(r, c int) Position {
		r = ((r % rows) + rows) % rows
		c = ((c % cols) + cols) % cols
		return Position{Row: r, Col: c}
	}

	// Generate cores with rotational symmetry
	for p := 0; p < numPlayers; p++ {
		angle := float64(p) * 2.0 * 3.14159 / float64(numPlayers)
		radius := 0.35 // 35% from center
		r := centerRow + int(float64(centerRow)*radius*cos(angle))
		c := centerCol + int(float64(centerCol)*radius*sin(angle))
		m.Cores = append(m.Cores, Core{
			Position: wrap(r, c),
			Owner:    p,
		})
	}

	// Generate energy nodes with rotational symmetry
	nodesPerSector := numEnergyNodes / numPlayers
	usedPositions := make(map[Position]bool)

	// Mark core positions as used
	for _, c := range m.Cores {
		usedPositions[c.Position] = true
	}

	for i := 0; i < nodesPerSector; i++ {
		for attempt := 0; attempt < 100; attempt++ {
			angle := rng.Float64() * 2.0 * 3.14159 / float64(numPlayers)
			radius := 0.2 + rng.Float64()*0.5 // 20-70% from center
			r := centerRow + int(float64(centerRow)*radius*cos(angle))
			c := centerCol + int(float64(centerCol)*radius*sin(angle))
			pos := wrap(r, c)

			if !usedPositions[pos] {
				usedPositions[pos] = true
				// Mirror for all players
				for p := 0; p < numPlayers; p++ {
					rotAngle := angle + float64(p)*2.0*3.14159/float64(numPlayers)
					rr := centerRow + int(float64(centerRow)*radius*cos(rotAngle))
					rc := centerCol + int(float64(centerCol)*radius*sin(rotAngle))
					m.EnergyNodes = append(m.EnergyNodes, wrap(rr, rc))
				}
				break
			}
		}
	}

	// Generate walls with rotational symmetry
	totalTiles := rows * cols
	targetWalls := int(float64(totalTiles) * wallDensity)
	wallsPerSector := targetWalls / numPlayers

	for i := 0; i < wallsPerSector; i++ {
		for attempt := 0; attempt < 100; attempt++ {
			angle := rng.Float64() * 2.0 * 3.14159 / float64(numPlayers)
			radius := 0.1 + rng.Float64()*0.7 // 10-80% from center
			r := centerRow + int(float64(centerRow)*radius*cos(angle))
			c := centerCol + int(float64(centerCol)*radius*sin(angle))
			pos := wrap(r, c)

			if !usedPositions[pos] {
				usedPositions[pos] = true
				// Mirror for all players
				for p := 0; p < numPlayers; p++ {
					rotAngle := angle + float64(p)*2.0*3.14159/float64(numPlayers)
					rr := centerRow + int(float64(centerRow)*radius*cos(rotAngle))
					rc := centerCol + int(float64(centerCol)*radius*sin(rotAngle))
					m.Walls = append(m.Walls, wrap(rr, rc))
				}
				break
			}
		}
	}

	return m
}

// Simple trig functions without importing math
func cos(x float64) float64 {
	// Normalize to [0, 2π)
	for x < 0 {
		x += 2.0 * 3.14159
	}
	for x >= 2.0*3.14159 {
		x -= 2.0 * 3.14159
	}

	// Taylor series approximation
	return 1 - x*x/2 + x*x*x*x/24 - x*x*x*x*x*x/720
}

func sin(x float64) float64 {
	// Normalize to [0, 2π)
	for x < 0 {
		x += 2.0 * 3.14159
	}
	for x >= 2.0*3.14159 {
		x -= 2.0 * 3.14159
	}

	// Taylor series approximation
	return x - x*x*x/6 + x*x*x*x*x/120
}
