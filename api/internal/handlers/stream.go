package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/julian-alarcon/dothesplit/api/internal/middleware"
	"github.com/julian-alarcon/dothesplit/api/internal/service"
)

// streamHeartbeat is how often a comment ping is written to keep the
// connection (and any intermediary proxy) alive when no events flow.
const streamHeartbeat = 25 * time.Second

// StreamGroupEvents is a long-lived Server-Sent Events endpoint. It pushes one
// `event: activity` frame whenever the group's activity feed gains a row, so
// other members' open dashboards refresh without polling. The frame carries a
// minimal JSON signal (ids + action + actor + timestamp); clients re-fetch.
//
// EventSource can't set the Authorization header, so the SPA consumes this via
// fetch + a ReadableStream reader, which carries the bearer token like any
// other request. Membership is enforced before any stream bytes are written.
func (s *Server) StreamGroupEvents(w http.ResponseWriter, r *http.Request) {
	u := middleware.User(r.Context())
	groupID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if s.Hub == nil {
		writeErr(w, http.StatusServiceUnavailable, "unavailable", "realtime stream not available")
		return
	}
	// Authz before any stream headers/bytes so a non-member gets a clean JSON error.
	if err := s.Groups.RequireMember(r.Context(), groupID, u.ID); err != nil {
		switch {
		case errors.Is(err, service.ErrNotMember):
			writeErr(w, http.StatusForbidden, "forbidden", "not a group member")
		default:
			writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		}
		return
	}

	rc := http.NewResponseController(w)
	// Clear the server's WriteTimeout for this long-lived response. Relies on
	// statusRecorder.Unwrap so the controller can reach the underlying conn.
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ch, cancel := s.Hub.Subscribe(groupID)
	defer cancel()

	// Initial comment so the client's reader unblocks and knows the stream is live.
	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	_ = rc.Flush()

	ticker := time.NewTicker(streamHeartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}
		case ev, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "id: %s\nevent: activity\ndata: %s\n\n", ev.ID, payload); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}
