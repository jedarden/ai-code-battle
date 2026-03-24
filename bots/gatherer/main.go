// Package main implements GathererBot - a bot that maximizes energy collection while avoiding combat.
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
	Bots   []VisibleBot `json:"bots"`
	Energy []Position   `json:"energy"`
	Cores  []VisibleCore `json:"cores"`
	Walls  []Position   `json:"walls"`
	Dead   []VisibleBot `json:"dead"`
}

// Direction represents a movement direction.
type Direction string

const (
	DirN Direction = "N"
	DirE Direction = "E"
	DirS Direction = "S"
	DirW Direction = "W"
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
	config  Config
	strategy *GathererStrategy
	mu      sync.Mutex
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
		strategy: NewGathererStrategy(),
	}

	http.HandleFunc("/turn", server.handleTurn)
	http.HandleFunc("/health", server.handleHealth)

	addr := fmt.Sprintf(":%s", config.Port)
	log.Printf("GathererBot starting on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func (s *Server) handleTurn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
	sig := r.Header.Get("X-ACB-Signature")
	if sig == "" {
		http.Error(w, "missing signature", http.StatusUnauthorized)
		return
	}

	matchID := r.Header.Get("X-ACB-Match-Id")
	turnStr := r.Header.Get("X-ACB-Turn")

	if err := verifySignature(s.config.Secret, matchID, turnStr, body, sig); err != nil {
		http.Error(w, fmt.Sprintf("signature verification failed: %v", err), http.StatusUnauthorized)
		return
	}

	// Parse game state
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
	responseSig := signResponse(s.config.Secret, matchID, turnStr, responseBody)
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

// verifySignature verifies the HMAC signature of an incoming request.
func verifySignature(secret, matchID, turnStr string, body []byte, signature string) error {
	// Compute expected signature
	// signing_string = "{match_id}.{turn}.{timestamp}.{sha256(request_body)}"
	// For requests, we also need timestamp, but we simplify here for the bot side

	bodyHash := sha256.Sum256(body)
	turn, _ := strconv.Atoi(turnStr)
	signingString := fmt.Sprintf("%s.%d.%s", matchID, turn, hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// signResponse signs the response body.
func signResponse(secret, matchID, turnStr string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	turn, _ := strconv.Atoi(turnStr)
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
