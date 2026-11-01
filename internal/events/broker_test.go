package events_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lucifer-135/freshness-guard/internal/events"
)

func receive(t *testing.T, ch <-chan events.Event) events.Event {
	t.Helper()
	select {
	case e, ok := <-ch:
		require.True(t, ok, "channel closed")
		return e
	case <-time.After(time.Second):
		t.Fatal("no event within 1 second")
		return events.Event{}
	}
}

func TestSubscriberReceivesEventsInOrder(t *testing.T) {
	b := events.NewBroker()
	ch, cancel := b.Subscribe()
	defer cancel()
	for _, typ := range []string{events.TypeReportStarted, events.TypeReportClaims, events.TypeReportComplete} {
		b.Publish(events.Event{Type: typ, ReportID: "r-1"})
	}
	for i, typ := range []string{events.TypeReportStarted, events.TypeReportClaims, events.TypeReportComplete} {
		e := receive(t, ch)
		assert.Equal(t, uint64(i+1), e.ID)
		assert.Equal(t, typ, e.Type)
		assert.Equal(t, "r-1", e.ReportID)
	}
}

func TestEverySubscriberGetsEvents(t *testing.T) {
	b := events.NewBroker()
	a, cancelA := b.Subscribe()
	defer cancelA()
	c, cancelC := b.Subscribe()
	defer cancelC()
	b.Publish(events.Event{Type: events.TypeReportStarted})
	assert.Equal(t, events.TypeReportStarted, receive(t, a).Type)
	assert.Equal(t, events.TypeReportStarted, receive(t, c).Type)
}

func TestPublishNeverBlocks(t *testing.T) {
	b := events.NewBroker()
	ch, cancel := b.Subscribe()
	defer cancel()
	done := make(chan struct{})
	go func() {
		for range events.SubscriberBuffer + 10 {
			b.Publish(events.Event{Type: events.TypeClaimVerdict})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a subscriber that does not read")
	}
	assert.Len(t, ch, events.SubscriberBuffer)
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	b := events.NewBroker()
	ch, cancel := b.Subscribe()
	cancel()
	_, ok := <-ch
	assert.False(t, ok)
	assert.NotPanics(t, cancel)
	assert.NotPanics(t, func() { b.Publish(events.Event{Type: events.TypeReportStarted}) })
}

func TestCloseEndsSubscriptions(t *testing.T) {
	b := events.NewBroker()
	ch, cancel := b.Subscribe()
	b.Close()
	_, ok := <-ch
	assert.False(t, ok)
	assert.NotPanics(t, func() { b.Publish(events.Event{Type: events.TypeReportStarted}) })
	assert.NotPanics(t, cancel)
	assert.NotPanics(t, b.Close)
}
