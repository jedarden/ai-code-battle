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
)

var directions = []string{"N", "E", "S", "W"}

type GameConfig struct {
	Rows           int `json:"rows"`
	Cols           int `json:"cols"`
	MaxTurns       int `json:"max_turns"`
	VisionRadius2  int `json:"vision_radius2"`
	AttackRadius2  int `json:"attack_radius2"`
	SpawnCost      int `json:"spawn_cost"`
	EnergyInterval int `json:"energy_interval"`
}

type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type VisibleBot struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
}

type VisibleCore struct {
	Position Position `json:"position"`
	Owner    int      `json:"owner"`
	Active   bool     `json:"active"`
}

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

type Move struct {
	Position  Position `json:"position"`
	Direction string   `json:"direction"`
}

type MoveResponse struct {
	Moves []Move `json:"moves"`
}

func main() {
	port := getEnv("BOT_PORT", "8080")
	secret := getEnv("BOT_SECRET", "")

	if secret == "" {
		log.Fatal("BOT_SECRET environment variable is required")
	}

	strategy := NewOpportunistStrategy()

	http.HandleFunc("/turn", func(w http.ResponseWriter, r *http.Request) {
		handleTurn(w, r, secret, strategy)
	})
	http.HandleFunc("/health", handleHealth)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Opportunist bot listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleTurn(w http.ResponseWriter, r *http.Request, secret string, strategy *OpportunistStrategy) {
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

	moves := strategy.ComputeMoves(&state)
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
