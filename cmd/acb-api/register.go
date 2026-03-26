package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var validBotName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{1,30}[a-zA-Z0-9]$`)

type RegisterRequest struct {
	Name        string `json:"name"`
	EndpointURL string `json:"endpoint_url"`
	Owner       string `json:"owner"`
	Description string `json:"description,omitempty"`
}

type RegisterResponse struct {
	BotID        string `json:"bot_id"`
	SharedSecret string `json:"shared_secret"`
	Message      string `json:"message"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Owner = strings.TrimSpace(req.Owner)
	req.EndpointURL = strings.TrimSpace(req.EndpointURL)

	if !validBotName.MatchString(req.Name) {
		writeError(w, http.StatusBadRequest, "name must be 3-32 alphanumeric/hyphen chars")
		return
	}
	if req.EndpointURL == "" {
		writeError(w, http.StatusBadRequest, "endpoint_url is required")
		return
	}
	if req.Owner == "" {
		writeError(w, http.StatusBadRequest, "owner is required")
		return
	}

	// Health check the bot endpoint
	if err := s.checkBotHealth(req.EndpointURL); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("bot health check failed: %v", err))
		return
	}

	botID, err := generateID("b_", 4) // b_ + 8 hex chars
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate bot ID")
		return
	}

	secret, err := generateSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate shared secret")
		return
	}

	// Encrypt secret for storage
	encryptedSecret := secret // default: store plaintext if no key
	if s.cfg.EncryptionKey != "" {
		encryptedSecret, err = encryptSecret(secret, s.cfg.EncryptionKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt secret")
			return
		}
	}

	_, err = s.db.ExecContext(r.Context(),
		`INSERT INTO bots (bot_id, name, owner, endpoint_url, shared_secret, status, description, last_active)
		 VALUES ($1, $2, $3, $4, $5, 'active', $6, NOW())`,
		botID, req.Name, req.Owner, req.EndpointURL, encryptedSecret, req.Description,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "bot name already taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to register bot")
		return
	}

	writeJSON(w, http.StatusCreated, RegisterResponse{
		BotID:        botID,
		SharedSecret: secret,
		Message:      "Bot registered. Save the shared_secret — it will not be shown again.",
	})
}

func (s *Server) checkBotHealth(endpointURL string) error {
	url := strings.TrimRight(endpointURL, "/") + "/health"
	client := &http.Client{Timeout: time.Duration(s.cfg.BotTimeoutSecs) * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected 200, got %d", resp.StatusCode)
	}
	return nil
}

type RotateKeyRequest struct {
	BotID        string `json:"bot_id"`
	SharedSecret string `json:"shared_secret"`
	Retire       bool   `json:"retire,omitempty"`
}

type RotateKeyResponse struct {
	NewSecret string `json:"new_secret,omitempty"`
	Message   string `json:"message"`
}

func (s *Server) handleRotateKey(w http.ResponseWriter, r *http.Request) {
	var req RotateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Verify current secret
	var storedSecret string
	err := s.db.QueryRowContext(r.Context(),
		`SELECT shared_secret FROM bots WHERE bot_id = $1`, req.BotID,
	).Scan(&storedSecret)
	if err != nil {
		writeError(w, http.StatusNotFound, "bot not found")
		return
	}

	// Decrypt stored secret for comparison
	plainSecret := storedSecret
	if s.cfg.EncryptionKey != "" {
		plainSecret, err = decryptSecret(storedSecret, s.cfg.EncryptionKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "decryption error")
			return
		}
	}

	if plainSecret != req.SharedSecret {
		writeError(w, http.StatusUnauthorized, "invalid shared secret")
		return
	}

	if req.Retire {
		_, err = s.db.ExecContext(r.Context(),
			`UPDATE bots SET status = 'retired' WHERE bot_id = $1`, req.BotID,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to retire bot")
			return
		}
		writeJSON(w, http.StatusOK, RotateKeyResponse{Message: "Bot retired."})
		return
	}

	newSecret, err := generateSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate new secret")
		return
	}

	encryptedSecret := newSecret
	if s.cfg.EncryptionKey != "" {
		encryptedSecret, err = encryptSecret(newSecret, s.cfg.EncryptionKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt secret")
			return
		}
	}

	_, err = s.db.ExecContext(r.Context(),
		`UPDATE bots SET shared_secret = $1 WHERE bot_id = $2`,
		encryptedSecret, req.BotID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update secret")
		return
	}

	writeJSON(w, http.StatusOK, RotateKeyResponse{
		NewSecret: newSecret,
		Message:   "Secret rotated. Save the new secret — it will not be shown again.",
	})
}

func (s *Server) handleBotStatus(w http.ResponseWriter, r *http.Request) {
	botID := r.PathValue("bot_id")

	var bot struct {
		BotID       string   `json:"bot_id"`
		Name        string   `json:"name"`
		Owner       string   `json:"owner"`
		Status      string   `json:"status"`
		Rating      float64  `json:"rating"`
		RatingMu    float64  `json:"rating_mu"`
		RatingPhi   float64  `json:"rating_phi"`
		Description *string  `json:"description,omitempty"`
		CreatedAt   string   `json:"created_at"`
		LastActive  *string  `json:"last_active,omitempty"`
	}

	var desc, lastActive *string
	err := s.db.QueryRowContext(r.Context(),
		`SELECT bot_id, name, owner, status, rating_mu, rating_phi, description, created_at, last_active
		 FROM bots WHERE bot_id = $1`, botID,
	).Scan(&bot.BotID, &bot.Name, &bot.Owner, &bot.Status, &bot.RatingMu, &bot.RatingPhi,
		&desc, &bot.CreatedAt, &lastActive)
	if err != nil {
		writeError(w, http.StatusNotFound, "bot not found")
		return
	}

	bot.Description = desc
	bot.LastActive = lastActive
	bot.Rating = bot.RatingMu - 2*bot.RatingPhi // conservative display rating

	writeJSON(w, http.StatusOK, bot)
}
