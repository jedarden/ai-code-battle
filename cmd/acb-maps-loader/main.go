package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"
)

// MapFile represents the structure of map JSON files in the maps/ directory.
type MapFile struct {
	ID          string     `json:"id"`
	Players     int        `json:"players"`
	Rows        int        `json:"rows"`
	Cols        int        `json:"cols"`
	WallDensity float64    `json:"wall_density"`
	Walls       []Position `json:"walls"`
	Cores       []Core     `json:"cores"`
	EnergyNodes []Position `json:"energy_nodes"`
	Generated   string     `json:"generated,omitempty"`
}

// Position represents a row/column position.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Core represents a core with position and owner.
type Core struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
}

// MapGeometry represents the geometry stored in map_json.
type MapGeometry struct {
	Walls       []Position        `json:"walls"`
	Cores       []MapDatabaseCore `json:"cores"`
	EnergyNodes []Position        `json:"energy_nodes"`
}

// MapDatabaseCore represents a core as stored in the database map_json.
type MapDatabaseCore struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
}

func main() {
	// Default connection string - override with environment variables
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "host=localhost port=5432 user=postgres password=postgres dbname=acb sslmode=disable"
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Get the project root directory
	projectRoot := os.Getenv("ACB_PROJECT_ROOT")
	if projectRoot == "" {
		// Try to detect from current directory
		if _, err := os.Stat("maps"); err == nil {
			projectRoot = "."
		} else if _, err := os.Stat("../maps"); err == nil {
			projectRoot = ".."
		} else {
			log.Fatal("Cannot find project root. Set ACB_PROJECT_ROOT environment variable or run from project root.")
		}
	}

	mapsDir := filepath.Join(projectRoot, "maps")

	// Load maps for each player count
	playerCounts := []int{2, 3, 4, 6}
	totalLoaded := 0

	for _, players := range playerCounts {
		playerDir := filepath.Join(mapsDir, fmt.Sprintf("%dplayer", players))
		loaded, err := loadMapsForPlayerCount(db, playerDir, players)
		if err != nil {
			log.Printf("Error loading maps for %d players: %v", players, err)
			continue
		}
		totalLoaded += loaded
		log.Printf("Loaded %d maps for %d players", loaded, players)
	}

	log.Printf("Total maps loaded: %d", totalLoaded)
}

func loadMapsForPlayerCount(db *sql.DB, dir string, playerCount int) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		if err := loadMapFile(db, filePath, playerCount); err != nil {
			log.Printf("Warning: failed to load %s: %v", filePath, err)
			continue
		}
		loaded++
	}

	return loaded, nil
}

func loadMapFile(db *sql.DB, filePath string, playerCount int) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var mapFile MapFile
	if err := json.Unmarshal(data, &mapFile); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Count energy nodes
	energyCount := len(mapFile.EnergyNodes)

	// Build map_json geometry
	geometry := MapGeometry{
		Walls:       mapFile.Walls,
		Cores:       make([]MapDatabaseCore, len(mapFile.Cores)),
		EnergyNodes: mapFile.EnergyNodes,
	}
	for i, c := range mapFile.Cores {
		geometry.Cores[i] = MapDatabaseCore{
			Position: c.Position,
			Owner:    c.Owner,
		}
	}

	mapJSON, err := json.Marshal(geometry)
	if err != nil {
		return fmt.Errorf("failed to marshal map_json: %w", err)
	}

	// Use the map ID from the file if present, otherwise generate one
	mapID := mapFile.ID
	if mapID == "" {
		mapID = fmt.Sprintf("map_%dp_%s", playerCount, strings.TrimSuffix(filepath.Base(filePath), ".json"))
	}

	// Insert or update the map
	query := `
		INSERT INTO maps (map_id, player_count, status, engagement, wall_density, energy_count, grid_width, grid_height, map_json)
		VALUES ($1, $2, 'active', 0.0, $3, $4, $5, $6, $7)
		ON CONFLICT (map_id) DO UPDATE SET
			wall_density = EXCLUDED.wall_density,
			energy_count = EXCLUDED.energy_count,
			grid_width = EXCLUDED.grid_width,
			grid_height = EXCLUDED.grid_height,
			map_json = EXCLUDED.map_json
	`

	_, err = db.Exec(query, mapID, playerCount, mapFile.WallDensity, energyCount, mapFile.Cols, mapFile.Rows, mapJSON)
	if err != nil {
		return fmt.Errorf("failed to insert map: %w", err)
	}

	return nil
}
