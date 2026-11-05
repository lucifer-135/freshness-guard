// Package events broadcasts verification progress to subscribers, such as the
// plugin API's event stream, without ever blocking the publisher.
package events

import (
	"sync"
	"time"
)

// Event types.
const (
	TypeReportStarted  = "report.started"
	TypeReportClaims   = "report.claims"
	TypeClaimVerdict   = "claim.verdict"
	TypeReportComplete = "report.complete"
	TypeReportError    = "report.error"
	// TypeCachePurged says cached answers were removed from the semantic cache.
	TypeCachePurged = "cache.purged"
)

// SubscriberBuffer is the number of events a subscriber can fall behind
// before further events are dropped for it.
const SubscriberBuffer = 64

// Event is one progress update.
type Event struct {
	ID       uint64    `json:"id"`
	Type     string    `json:"type"`
	ReportID string    `json:"report_id"`
	At       time.Time `json:"at"`
	Data     any       `json:"data,omitempty"`
}

// Publisher accepts events; *Broker implements it.
type Publisher interface {
	Publish(e Event)
}

// Broker fans events out to subscribers. It is safe for concurrent use.
type Broker struct {
	mu     sync.Mutex
	nextID uint64
	subs   map[chan Event]struct{}
	closed bool
}

// NewBroker returns a broker without subscribers.
func NewBroker() *Broker {
	return &Broker{subs: map[chan Event]struct{}{}}
}

// Publish numbers the event and sends it to every subscriber. A subscriber
// whose buffer is full misses the event; Publish never waits.
func (b *Broker) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.nextID++
	e.ID = b.nextID
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Subscribe returns a channel of events and a function that ends the
// subscription and closes the channel. The function may be called more than
// once.
func (b *Broker) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, SubscriberBuffer)
	if b.closed {
		close(ch)
		return ch, func() {}
	}
	b.subs[ch] = struct{}{}
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
	}
}

// Close ends every subscription. Later events are ignored.
func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subs {
		delete(b.subs, ch)
		close(ch)
	}
}
