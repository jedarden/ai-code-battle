package main

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	if err := s.rdb.Ping(ctx).Err(); err != nil {
		writeError(w, http.StatusServiceUnavailable, "valkey unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
