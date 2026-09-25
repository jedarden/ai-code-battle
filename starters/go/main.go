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
	"time"
)

var sharedSecret string

func main() {
	secret := os.Getenv("SHARED_SECRET")
	if secret == "" {
		log.Fatal("SHARED_SECRET environment variable must be set")
	}
	sharedSecret = secret

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/turn", handleTurn)

	fmt.Printf("Bot listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func handleTurn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Invalid authentication", http.StatusUnauthorized)
		return
	}

	// Get auth headers
	matchID := r.Header.Get("X-ACB-Match-Id")
	turnStr := r.Header.Get("X-ACB-Turn")
	timestamp := r.Header.Get("X-ACB-Timestamp")
	botID := r.Header.Get("X-ACB-Bot-Id")
	signature := r.Header.Get("X-ACB-Signature")

	if matchID == "" || turnStr == "" || timestamp == "" || botID == "" || signature == "" {
		http.Error(w, "Invalid authentication", http.StatusUnauthorized)
		return
	}

	// Read raw body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify signature
	if !verifySignature(body, matchID, turnStr, timestamp, signature) {
		http.Error(w, "Invalid authentication", http.StatusUnauthorized)
		return
	}

	turn, err := strconv.Atoi(turnStr)
	if err != nil || turn < 0 {
		http.Error(w, "Invalid authentication", http.StatusUnauthorized)
		return
	}

	// Parse game state
	// Schema strictness comes before the identity comparison: a missing or
	// mis-typed field is authenticated malformed input (400), while present
	// but contradictory values are an authentication failure (401).
	if err := decodeStrictState(body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var identity struct {
		MatchID string `json:"match_id"`
		Turn    int    `json:"turn"`
	}
	if err := json.Unmarshal(body, &identity); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if identity.MatchID != matchID || identity.Turn != turn {
		http.Error(w, "Invalid authentication", http.StatusUnauthorized)
		return
	}

	var state VisibleState
	if err := json.Unmarshal(body, &state); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Compute moves
	moves := ComputeMoves(&state)

	// Send response
	response := map[string]any{"moves": moves}
	responseBody, _ := json.Marshal(response)
	sig := signResponse(responseBody, matchID, turn)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-ACB-Signature", sig)
	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
}

func verifySignature(body []byte, matchID, turnStr, timestamp, signature string) bool {
	if sharedSecret == "" || matchID == "" || turnStr == "" || timestamp == "" || signature == "" {
		return false
	}

	timestampUnix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	age := time.Since(time.Unix(timestampUnix, 0))
	if age < -30*time.Second || age > 30*time.Second {
		return false
	}

	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%s.%s.%s", matchID, turnStr, timestamp, hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, []byte(sharedSecret))
	mac.Write([]byte(signingString))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

func signResponse(body []byte, matchID string, turn int) string {
	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%d.%s", matchID, turn, hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, []byte(sharedSecret))
	mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}
