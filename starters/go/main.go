package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
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
	// Read body
	var state VisibleState
	if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get auth headers
	matchID := r.Header.Get("X-ACB-Match-Id")
	turnStr := r.Header.Get("X-ACB-Turn")
	signature := r.Header.Get("X-ACB-Signature")

	// Verify signature (optional but recommended)
	body, _ := json.Marshal(state)
	if !verifySignature(body, matchID, turnStr, signature) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Compute moves
	moves := ComputeMoves(&state)

	// Send response
	response := map[string]any{"moves": moves}
	responseBody, _ := json.Marshal(response)
	turn, _ := strconv.Atoi(turnStr)
	sig := signResponse(responseBody, matchID, turn)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-ACB-Signature", sig)
	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
}

func verifySignature(body []byte, matchID, turnStr, signature string) bool {
	if signature == "" {
		return true // Skip verification if not provided
	}

	bodyHash := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%s.%s", matchID, turnStr, hex.EncodeToString(bodyHash[:]))
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
