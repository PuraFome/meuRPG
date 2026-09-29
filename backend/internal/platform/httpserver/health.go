package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// dbPingTimeout keeps /readyz fast even when the database hangs.
const dbPingTimeout = 2 * time.Second

// probeResponse is the JSON body of /healthz and /readyz.
type probeResponse struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
}

// handleHealthz is the liveness probe: "is the process alive?". It never
// checks dependencies. If it did, a database outage would make the platform
// restart perfectly healthy containers, which cannot fix the database.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeProbe(w, http.StatusOK, probeResponse{Status: "ok"})
}

// handleReadyz is the readiness probe: "can this instance serve traffic
// right now?". It answers 503 while shutting down or when the database is
// configured but unreachable.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.draining.Load() {
		writeProbe(w, http.StatusServiceUnavailable, probeResponse{Status: "shutting_down"})
		return
	}

	if s.db == nil {
		writeProbe(w, http.StatusOK, probeResponse{Status: "ready", Database: "disabled"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dbPingTimeout)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		// The details go to the log, not to the (unauthenticated) caller.
		s.logger.WarnContext(r.Context(), "readiness: database ping failed", "error", err)
		writeProbe(w, http.StatusServiceUnavailable, probeResponse{Status: "not_ready", Database: "down"})
		return
	}

	writeProbe(w, http.StatusOK, probeResponse{Status: "ready", Database: "up"})
}

func writeProbe(w http.ResponseWriter, status int, body probeResponse) {
	w.Header().Set("Content-Type", "application/json")
	// Probes must reflect the current state, never a cached one.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// Nothing useful can be done if the client has gone away.
	_ = json.NewEncoder(w).Encode(body)
}
