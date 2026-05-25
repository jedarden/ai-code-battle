// Generate maps/index.json and maps/{map_id}.json from the maps/ directory
// This script bypasses the database and reads map files directly.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
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

// MapIndexEntry is the per-map summary in maps/index.json.
type MapIndexEntry struct {
	MapID       string  `json:"map_id"`
	PlayerCount int     `json:"player_count"`
	Status      string  `json:"status"`
	Engagement  float64 `json:"engagement"`
	WallDensity float64 `json:"wall_density"`
	EnergyCount int     `json:"energy_count"`
	GridWidth   int     `json:"grid_width"`
	GridHeight  int     `json:"grid_height"`
	NetVotes    int     `json:"net_votes"`
	CreatedAt   string  `json:"created_at"`
}

// MapIndexFile represents maps/index.json.
type MapIndexFile struct {
	UpdatedAt     string                     `json:"updated_at"`
	Maps          []MapIndexEntry            `json:"maps"`
	ByPlayerCount map[string][]MapIndexEntry `json:"by_player_count"`
}

// MapDetail represents maps/{map_id}.json
type MapDetail struct {
	MapID       string     `json:"map_id"`
	PlayerCount int        `json:"player_count"`
	Status      string     `json:"status"`
	Engagement  float64    `json:"engagement"`
	WallDensity float64    `json:"wall_density"`
	EnergyCount int        `json:"energy_count"`
	GridWidth   int        `json:"grid_width"`
	GridHeight  int        `json:"grid_height"`
	NetVotes    int        `json:"net_votes"`
	CreatedAt   string     `json:"created_at"`
	Walls       []Position `json:"walls"`
	Cores       []Core     `json:"cores"`
	EnergyNodes []Position `json:"energy_nodes"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <maps-dir> <output-dir>\n", os.Args[0])
		os.Exit(1)
	}

	mapsDir := os.Args[1]
	outputDir := os.Args[2]

	// Create maps output directory
	mapsOutDir := filepath.Join(outputDir, "maps")
	if err := os.MkdirAll(mapsOutDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create maps directory: %v\n", err)
		os.Exit(1)
	}

	entries := make([]MapIndexEntry, 0)
	byPlayerCount := make(map[string][]MapIndexEntry)

	playerCounts := []int{2, 3, 4, 6}
	for _, players := range playerCounts {
		playerDir := filepath.Join(mapsDir, fmt.Sprintf("%dplayer", players))
		files, err := os.ReadDir(playerDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read %s: %v\n", playerDir, err)
			continue
		}

		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
				continue
			}

			filePath := filepath.Join(playerDir, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to read %s: %v\n", filePath, err)
				continue
			}

			var mapFile MapFile
			if err := json.Unmarshal(data, &mapFile); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", filePath, err)
				continue
			}

			mapID := mapFile.ID
			if mapID == "" {
				mapID = fmt.Sprintf("map_%dp_%s", players, file.Name()[:len(file.Name())-5])
			}

			energyCount := len(mapFile.EnergyNodes)
			entry := MapIndexEntry{
				MapID:       mapID,
				PlayerCount: players,
				Status:      "active",
				Engagement:  0.0,
				WallDensity: mapFile.WallDensity,
				EnergyCount: energyCount,
				GridWidth:   mapFile.Cols,
				GridHeight:  mapFile.Rows,
				NetVotes:    0,
				CreatedAt:   time.Now().Format(time.RFC3339),
			}
			entries = append(entries, entry)
			key := fmt.Sprintf("%d", players)
			byPlayerCount[key] = append(byPlayerCount[key], entry)

			// Write individual map detail file
			detail := MapDetail{
				MapID:       mapID,
				PlayerCount: players,
				Status:      "active",
				Engagement:  0.0,
				WallDensity: mapFile.WallDensity,
				EnergyCount: energyCount,
				GridWidth:   mapFile.Cols,
				GridHeight:  mapFile.Rows,
				NetVotes:    0,
				CreatedAt:   time.Now().Format(time.RFC3339),
				Walls:       mapFile.Walls,
				Cores:       mapFile.Cores,
				EnergyNodes: mapFile.EnergyNodes,
			}

			detailPath := filepath.Join(mapsOutDir, mapID+".json")
			detailJSON, _ := json.MarshalIndent(detail, "", "  ")
			if err := os.WriteFile(detailPath, detailJSON, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write %s: %v\n", detailPath, err)
			}
		}
	}

	// Write index.json
	index := MapIndexFile{
		UpdatedAt:     time.Now().Format(time.RFC3339),
		Maps:          entries,
		ByPlayerCount: byPlayerCount,
	}
	indexJSON, _ := json.MarshalIndent(index, "", "  ")
	indexPath := filepath.Join(mapsOutDir, "index.json")
	if err := os.WriteFile(indexPath, indexJSON, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write maps/index.json: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %d map entries in %s\n", len(entries), indexPath)
}
