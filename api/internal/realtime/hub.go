// Package realtime fans out activity-feed events to connected SSE clients.
//
// Events originate from a Postgres AFTER INSERT trigger on activity_events that
// emits pg_notify on the 'activity_events' channel (see migration 0001). A
// single LISTEN connection (RunListener) decodes each notification into an
// Event and calls Hub.Publish, which delivers it to every client subscribed to
// that event's group. Because the capture point is the database, every writer -
// the API request path, the importers, and the separate recurring worker
// process - is covered uniformly, and only committed rows are ever delivered.
package realtime

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Event is the minimal change signal delivered to subscribers. It carries only
// identifiers (no amounts/notes), matching the trigger payload. Clients react
// by re-fetching, so a dropped event is self-healing.
type Event struct {
	ID           uuid.UUID  `json:"id"`
	GroupID      uuid.UUID  `json:"group_id"`
	ActorID      *uuid.UUID `json:"actor_id"`
	Action       string     `json:"action"`
	ExpenseID    *uuid.UUID `json:"expense_id"`
	SettlementID *uuid.UUID `json:"settlement_id"`
	CreatedAt    time.Time  `json:"created_at"`
}

// subBuffer bounds per-client queueing. A client that can't keep up drops
// events rather than blocking the listener or its peers; it re-syncs on the
// next delivered event because every event triggers a full re-fetch.
const subBuffer = 16

// Hub is a group-scoped pub/sub fan-out. The zero value is not usable; call
// NewHub. It is safe for concurrent use.
type Hub struct {
	mu   sync.RWMutex
	subs map[uuid.UUID]map[chan Event]struct{} // groupID -> set of subscriber channels
}

// NewHub returns an empty Hub ready for Subscribe/Publish.
func NewHub() *Hub {
	return &Hub{subs: make(map[uuid.UUID]map[chan Event]struct{})}
}

// Subscribe registers a new subscriber for groupID and returns its receive
// channel plus a cancel func. cancel removes the subscription and closes the
// channel; it is idempotent and safe to defer. The channel is buffered and
// must be drained by the caller's read loop.
func (h *Hub) Subscribe(groupID uuid.UUID) (<-chan Event, func()) {
	ch := make(chan Event, subBuffer)
	h.mu.Lock()
	set := h.subs[groupID]
	if set == nil {
		set = make(map[chan Event]struct{})
		h.subs[groupID] = set
	}
	set[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			if set := h.subs[groupID]; set != nil {
				delete(set, ch)
				if len(set) == 0 {
					delete(h.subs, groupID)
				}
			}
			h.mu.Unlock()
			close(ch)
		})
	}
	return ch, cancel
}

// Publish delivers ev to every subscriber of ev.GroupID. Delivery is
// non-blocking: if a subscriber's buffer is full the event is dropped for that
// subscriber (it will re-sync on its next event), so one slow client never
// stalls the listener or other clients.
func (h *Hub) Publish(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs[ev.GroupID] {
		select {
		case ch <- ev:
		default:
		}
	}
}
