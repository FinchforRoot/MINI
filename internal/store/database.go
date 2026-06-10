package database

import (
	"sync"
	"time"
)

type TransferEvent struct {
	BlockNumber uint64    `json:"blockNumber"`
	TxHash      string    `json:"tx_hash"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Value       string    `json:"value"`
	Timestamp   time.Time `json:"timestamp"`
}

type EventStore struct {
	mu     sync.RWMutex
	events []TransferEvent
	limit  int
}

func NewEventStore(limit int) *EventStore {
	return &EventStore{
		events: make([]TransferEvent, 0, limit),
		limit:  limit,
	}
}

func (s *EventStore) Add(event TransferEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) >= s.limit {
		s.events = s.events[1:] // Remove oldest event
	}
	s.events = append(s.events, event)
}

func (s *EventStore) list() []TransferEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TransferEvent, len(s.events))
	copy(out, s.events)
	return out
}
