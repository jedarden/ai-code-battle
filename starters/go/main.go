// AI Code Battle - Go Starter Kit
//
// A minimal bot scaffold with HMAC authentication and a placeholder
// strategy. Implement computeMoves() to build your bot.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"acb-starter-go/game"
)

func main() {
	port := getEnv("BOT_PORT", "8080")
	secret := getEnv("BOT_SECRET", "")

	if secret == "" {
		log.Fatal("BOT_SECRET environment variable is required")
	}

	server := &Server{
		secret: secret,
	}

	http.HandleFunc("/turn", server.handleTurn)
	http.HandleFunc("/health", server.handleHealth)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Bot listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// Server handles HTTP requests for the bot.
type Server struct {
	secret string
}

func (s *Server) handleTurn(w http.ResponseWriter, r *http.Request) {
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

	headers := game.AuthHeaders{
		MatchID:   r.Header.Get("X-ACB-Match-Id"),
		Turn:      r.Header.Get("X-ACB-Turn"),
		Timestamp: r.Header.Get("X-ACB-Timestamp"),
		Signature: r.Header.Get("X-ACB-Signature"),
	}

	if !game.VerifyRequest(s.secret, headers, body) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var state game.GameState
	if err := json.Unmarshal(body, &state); err != nil {
		http.Error(w, "invalid game state", http.StatusBadRequest)
		return
	}

	moves := computeMoves(&state)
	response := game.MoveResponse{Moves: moves}
	responseBody, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to marshal response", http.StatusInternalServerError)
		return
	}

	responseSig := game.SignResponse(s.secret, headers.MatchID, headers.Turn, responseBody)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-ACB-Signature", responseSig)
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

// computeMoves is where you implement your bot's strategy.
//
// The game engine calls this function every turn with the current game state.
// Return a list of moves for your bots. Any bot not included in the response
// will hold position.
//
// Use the utilities in the game package:
//   - game.ToroidalManhattan() for distance calculations
//   - game.BFSDirection() for pathfinding
//   - game.Neighbors() for getting adjacent positions
//
// Example:
//
//	moves := []game.Move{}
//	for _, bot := range state.Bots {
//	    if bot.Owner == state.You.ID {
//	        moves = append(moves, game.Move{
//	            Position:  bot.Position,
//	            Direction: game.DirN, // Move north
//	        })
//	    }
//	}
//	return moves
func computeMoves(state *game.GameState) []game.Move {
	// TODO: Implement your strategy here!
	//
	// This stub returns no moves, which means all your bots will hold
	// position every turn. Replace this with your own logic.

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
