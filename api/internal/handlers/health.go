package handlers

import (
	"log/slog"
	"net/http"

	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
)

// healthStatus is the JSON shape returned by /healthz and /readyz on success.
// It is intentionally not declared in the OpenAPI contract: probes live outside
// /v1 (see CLAUDE.md) and we don't want to version this.
type healthStatus struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func (s *Server) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthStatus{Status: "ok", Version: s.Version, Commit: s.Commit})
}

func (s *Server) Readyz(w http.ResponseWriter, r *http.Request) {
	if err := s.Pool.Ping(r.Context()); err != nil {
		// Don't echo the raw driver error to the client: it can leak
		// connection-string fragments and infrastructure detail. Log it
		// server-side and return a static message.
		slog.Error("readyz: database ping failed", slog.String("err", err.Error()))
		writeJSON(w, http.StatusServiceUnavailable, apigen.Error{
			Code: "not_ready", Message: "database unreachable",
		})
		return
	}
	writeJSON(w, http.StatusOK, healthStatus{Status: "ok", Version: s.Version, Commit: s.Commit})
}
