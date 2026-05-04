package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/julian-alarcon/dothesplit/api/internal/apigen"
)

func (s *Server) Healthz(c *gin.Context) {
	c.Status(http.StatusOK)
}

func (s *Server) Readyz(c *gin.Context) {
	if err := s.Pool.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, apigen.Error{
			Code: "not_ready", Message: err.Error(),
		})
		return
	}
	c.Status(http.StatusOK)
}
