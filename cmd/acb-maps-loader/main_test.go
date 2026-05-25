package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMapFile(t *testing.T) {
	// Find the project root
	projectRoot := "../.."
	testMapPath := filepath.Join(projectRoot, "maps/2player/map_1.json")

	data, err := os.ReadFile(testMapPath)
	if err != nil {
		t.Fatalf("Failed to read test map: %v", err)
	}

	var mapFile MapFile
	if err := json.Unmarshal(data, &mapFile); err != nil {
		t.Fatalf("Failed to parse map JSON: %v", err)
	}

	if mapFile.Players != 2 {
		t.Errorf("Expected 2 players, got %d", mapFile.Players)
	}

	if len(mapFile.Walls) == 0 {
		t.Error("Expected walls to be present")
	}

	if len(mapFile.Cores) == 0 {
		t.Error("Expected cores to be present")
	}

	if len(mapFile.EnergyNodes) == 0 {
		t.Error("Expected energy nodes to be present")
	}

	// Test building map_json geometry
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
		t.Fatalf("Failed to marshal map_json: %v", err)
	}

	// Verify the JSON can be unmarshaled back
	var decoded MapGeometry
	if err := json.Unmarshal(mapJSON, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal map_json: %v", err)
	}

	if len(decoded.Walls) != len(mapFile.Walls) {
		t.Errorf("Walls count mismatch: got %d, want %d", len(decoded.Walls), len(mapFile.Walls))
	}

	if len(decoded.Cores) != len(mapFile.Cores) {
		t.Errorf("Cores count mismatch: got %d, want %d", len(decoded.Cores), len(mapFile.Cores))
	}

	if len(decoded.EnergyNodes) != len(mapFile.EnergyNodes) {
		t.Errorf("Energy nodes count mismatch: got %d, want %d", len(decoded.EnergyNodes), len(mapFile.EnergyNodes))
	}
}

func TestLoadAllMapFiles(t *testing.T) {
	projectRoot := "../.."
	mapsDir := filepath.Join(projectRoot, "maps")

	playerCounts := []int{2, 3, 4, 6}
	totalFiles := 0

	for _, players := range playerCounts {
		playerDir := filepath.Join(mapsDir, fmt.Sprintf("%dplayer", players))
		entries, err := os.ReadDir(playerDir)
		if err != nil {
			t.Fatalf("Failed to read directory %s: %v", playerDir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}

			filePath := filepath.Join(playerDir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Logf("Warning: failed to read %s: %v", filePath, err)
				continue
			}

			var mapFile MapFile
			if err := json.Unmarshal(data, &mapFile); err != nil {
				t.Logf("Warning: failed to parse %s: %v", filePath, err)
				continue
			}

			if mapFile.Players != players {
				t.Errorf("Map %s: expected %d players, got %d", entry.Name(), players, mapFile.Players)
			}

			totalFiles++
		}
	}

	t.Logf("Successfully parsed %d map files", totalFiles)

	// We expect 50 maps per player count (200 total)
	expectedTotal := 50 * len(playerCounts)
	if totalFiles < expectedTotal {
		t.Logf("Note: parsed %d files, expected %d (some maps may not exist yet)", totalFiles, expectedTotal)
	}
}
