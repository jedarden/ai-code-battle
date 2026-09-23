// Package main implements SiegeBot - a bot that blocks enemy core spawning by surrounding cores.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// Config holds bot configuration from environment variables.
type Config struct {
	Port   string
	Secret string
}

// GameConfig holds the game configuration from the engine.
type GameConfig struct {
	Rows           int `json:"rows"`
	Cols           int `json:"cols"`
	MaxTurns       int `json:"max_turns"`
	VisionRadius2  int `json:"vision_radius2"`
	AttackRadius2  int `json:"attack_radius2"`
	SpawnCost      int `json:"spawn_cost"`
	EnergyInterval int `json:"energy_interval"`
}

// Position represents a grid coordinate.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// VisibleBot represents a visible bot.
type VisibleBot struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
}

// VisibleCore represents a visible core.
type VisibleCore struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
	Active   bool     `json:"active"`
}

// ZoneBounds represents the active zone bounds.
type ZoneBounds struct {
	Center Position `json:"center"`
	Radius int      `json:"radius"`
	Active bool     `json:"active"`
}

// GameState represents the fog-filtered state visible to this bot.
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
	Zone   *ZoneBounds   `json:"zone,omitempty"`
}

// Direction represents a movement direction.
type Direction string

const (
	DirNone Direction = ""
	DirN    Direction = "N"
	DirE    Direction = "E"
	DirS    Direction = "S"
	DirW    Direction = "W"
)

// Move represents a bot movement order.
type Move struct {
	Position  Position  `json:"position"`
	Direction Direction `json:"direction"`
}

// MoveResponse is the response sent back to the engine.
type MoveResponse struct {
	Moves []Move `json:"moves"`
}

// Server holds the bot server state.
type Server struct {
	config   Config
	strategy *SiegeStrategy
	mu       sync.Mutex
}

func main() {
	config := Config{
		Port:   getEnv("BOT_PORT", "8080"),
		Secret: getEnv("BOT_SECRET", ""),
	}

	if config.Secret == "" {
		log.Fatal("BOT_SECRET environment variable is required")
	}

	server := &Server{
		config:   config,
		strategy: NewSiegeStrategy(),
	}

	http.HandleFunc("/turn", server.handleTurn)
	http.HandleFunc("/health", server.handleHealth)

	addr := fmt.Sprintf(":%s", config.Port)
	log.Printf("SiegeBot starting on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func (s *Server) handleTurn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	matchID := r.Header.Get("X-ACB-Match-Id")
	turnStr := r.Header.Get("X-ACB-Turn")
	timestamp := r.Header.Get("X-ACB-Timestamp")
	botID := r.Header.Get("X-ACB-Bot-Id")
	signature := r.Header.Get("X-ACB-Signature")

	if matchID == "" || turnStr == "" || timestamp == "" || botID == "" || signature == "" {
		http.Error(w, "invalid authentication", http.StatusUnauthorized)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify signature
	if err := verifySignature(s.config.Secret, matchID, turnStr, timestamp, body, signature); err != nil {
		http.Error(w, "invalid authentication", http.StatusUnauthorized)
		return
	}

	turn, err := strconv.Atoi(turnStr)
	if err != nil || turn < 0 {
		http.Error(w, "invalid authentication", http.StatusUnauthorized)
		return
	}

	// Parse game state
	var identity struct {
		MatchID *string `json:"match_id"`
		Turn    *int    `json:"turn"`
	}
	if err := json.Unmarshal(body, &identity); err != nil {
		http.Error(w, "invalid game state", http.StatusBadRequest)
		return
	}
	if identity.MatchID == nil || identity.Turn == nil || *identity.MatchID != matchID || *identity.Turn != turn {
		http.Error(w, "invalid authentication", http.StatusUnauthorized)
		return
	}

	var state GameState
	if err := json.Unmarshal(body, &state); err != nil {
		http.Error(w, "invalid game state", http.StatusBadRequest)
		return
	}

	// Compute moves
	s.mu.Lock()
	moves := s.strategy.ComputeMoves(&state)
	s.mu.Unlock()

	// Build response
	response := MoveResponse{Moves: moves}
	responseBody, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to marshal response", http.StatusInternalServerError)
		return
	}

	// Sign response
	responseSig := signResponse(s.config.Secret, matchID, turn, responseBody)
	w.Header().Set("X-ACB-Signature", responseSig)
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseBody)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// verifySignature verifies the timestamp-bound HMAC signature of an incoming request.
func verifySignature(secret, matchID, turnStr, timestamp string, body []byte, signature string) error {
	if secret == "" || matchID == "" || turnStr == "" || timestamp == "" || signature == "" {
		return fmt.Errorf("missing authentication")
	}

	timestampUnix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	age := time.Since(time.Unix(timestampUnix, 0))
	if age < -30*time.Second || age > 30*time.Second {
		return fmt.Errorf("expired timestamp")
	}

	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%s.%s.%s", matchID, turnStr, timestamp, hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// signResponse signs the response body.
func signResponse(secret, matchID string, turn int, body []byte) string {
	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%d.%s", matchID, turn, hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
