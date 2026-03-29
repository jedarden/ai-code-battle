package main

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/redis/go-redis/v9"
)

type Server struct {
	cfg     Config
	db      *sql.DB
	rdb     *redis.Client
	// Note: alerter removed - alerting now handled by acb-matchmaker deployment
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)
	mux.HandleFunc("POST /api/register", s.handleRegister)
	mux.HandleFunc("POST /api/rotate-key", s.handleRotateKey)
	mux.HandleFunc("GET /api/status/{bot_id}", s.handleBotStatus)
	mux.HandleFunc("POST /api/jobs/claim", s.handleJobClaim)
	mux.HandleFunc("POST /api/jobs/{job_id}/result", s.handleJobResult)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
