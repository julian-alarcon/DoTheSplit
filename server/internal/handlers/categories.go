package handlers

import (
	"net/http"

	"github.com/julian-alarcon/dothesplit/server/internal/apigen"
)

// ListCategories returns the seeded category set (authenticated users only).
func (s *Server) ListCategories(w http.ResponseWriter, r *http.Request) {
	list, err := s.Categories.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]apigen.Category, 0, len(list))
	for _, cat := range list {
		out = append(out, apigen.Category{
			Id:         cat.ID,
			Slug:       cat.Slug,
			Label:      cat.Label,
			GroupLabel: cat.GroupLabel,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
