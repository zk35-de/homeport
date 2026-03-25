package api

import (
	"sync"
)

// MessageType identifies the kind of SSE message.
type MessageType string

const (
	ServiceStatusMsg MessageType = "service_status"
	WidgetRefreshMsg MessageType = "widget_refresh"
)

// Message is the envelope sent over SSE.
type Message struct {
	Type    MessageType `json:"type"`
	Payload any         `json:"payload"`
}

// UpdateHub manages SSE clients for live updates.
type UpdateHub struct {
	mu      sync.Mutex
	clients map[chan Message]struct{}
}

// NewUpdateHub creates an initialised hub.
func NewUpdateHub() *UpdateHub {
	return &UpdateHub{
		clients: make(map[chan Message]struct{}),
	}
}

// Broadcast sends a message to all connected SSE clients.
// Non-blocking: slow clients are silently skipped.
func (h *UpdateHub) Broadcast(msg Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (h *UpdateHub) add(ch chan Message) {
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
}

func (h *UpdateHub) remove(ch chan Message) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}
