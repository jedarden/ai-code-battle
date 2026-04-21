// AI Code Battle - Go Starter Kit
//
// A minimal bot scaffold with HMAC authentication and a placeholder
// random strategy. Replace computeMoves() with your own logic.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
)

// Engine constants
var directions = []string{"N", "E", "S", "W"}

// GameConfig holds the match configuration.
type GameConfig struct {
	Rows           int `json:"rows"`
	Cols           int `json:"cols"`
	MaxTurns       int `json:"max_turns"`
	VisionRadius2  int `json:"vision_radius2"`
	AttackRadius2  int `json:"attack_radius2"`
	SpawnCost      int `json:"spawn_cost"`
	EnergyInterval int `json:"energy_interval"`
}

// Position is a grid coordinate.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// VisibleBot is a bot visible in fog of war.
type VisibleBot struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
}

// VisibleCore is a core visible in fog of war.
type VisibleCore struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
	Active   bool     `json:"active"`
}

// GameState is the fog-filtered state visible to this bot.
type GameState struct {
	MatchID string     `json:"match_id"`
	Turn    int        `json:"turn"`
	Config  GameConfig `json:"config"`
	You     struct {
		ID     int `json:"id"`
		Energy int `json:"energy"`
		Score  int `json:"score"`
	} `json:"you"`
	Bots   []VisibleBot  `json:"bots"`
	Energy []Position    `json:"energy"`
	Cores  []VisibleCore `json:"cores"`
	Walls  []Position    `json:"walls"`
	Dead   []VisibleBot  `json:"dead"`
}

// Move is a movement order for one bot.
type Move struct {
	Position  Position `json:"position"`
	Direction string   `json:"direction"`
}

// MoveResponse is sent back to the engine.
type MoveResponse struct {
	Moves []Move `json:"moves"`
}

func main() {
	port := getEnv("BOT_PORT", "8080")
	secret := getEnv("BOT_SECRET", "")

	if secret == "" {
		log.Fatal("BOT_SECRET environment variable is required")
	}

	http.HandleFunc("/turn", func(w http.ResponseWriter, r *http.Request) {
		handleTurn(w, r, secret)
	})
	http.HandleFunc("/health", handleHealth)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Bot listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleTurn(w http.ResponseWriter, r *http.Request, secret string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	sig := r.Header.Get("X-ACB-Signature")
	matchID := r.Header.Get("X-ACB-Match-Id")
	turnStr := r.Header.Get("X-ACB-Turn")
	timestamp := r.Header.Get("X-ACB-Timestamp")

	if sig == "" || !verifySignature(secret, matchID, turnStr, timestamp, body, sig) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var state GameState
	if err := json.Unmarshal(body, &state); err != nil {
		http.Error(w, "invalid game state", http.StatusBadRequest)
		return
	}

	moves := computeMoves(&state)
	response := MoveResponse{Moves: moves}
	responseBody, _ := json.Marshal(response)

	turn, _ := strconv.Atoi(turnStr)
	responseSig := signResponse(secret, matchID, turn, responseBody)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-ACB-Signature", responseSig)
	w.Write(responseBody)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func computeMoves(state *GameState) []Move {
	// Replace this with your strategy!
	var moves []Move
	for _, bot := range state.Bots {
		if bot.Owner == state.You.ID {
			if rand.Float64() < 0.5 {
				moves = append(moves, Move{
					Position:  bot.Position,
					Direction: directions[rand.Intn(len(directions))],
				})
			}
		}
	}
	return moves
}

func verifySignature(secret, matchID, turnStr, timestamp string, body []byte, signature string) bool {
	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%s.%s.%s", matchID, turnStr, timestamp, hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

func signResponse(secret, matchID string, turn int, body []byte) string {
	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%d.%s", matchID, turn, hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
