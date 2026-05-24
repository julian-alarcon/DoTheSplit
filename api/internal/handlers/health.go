package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

func (s *Server) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, healthStatus{Status: "ok", Version: s.Version, Commit: s.Commit})
}

func (s *Server) Readyz(c *gin.Context) {
	if err := s.Pool.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, apigen.Error{
			Code: "not_ready", Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, healthStatus{Status: "ok", Version: s.Version, Commit: s.Commit})
}
