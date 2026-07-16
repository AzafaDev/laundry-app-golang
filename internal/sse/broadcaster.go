package sse

import "sync"

type Event struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

type Broadcaster struct {
	mu   sync.RWMutex
	subs map[string]map[chan Event]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: make(map[string]map[chan Event]struct{})}
}

func (b *Broadcaster) Subscribe(channel string) chan Event {
	ch := make(chan Event, 8)
	b.mu.Lock()
	if b.subs[channel] == nil {
		b.subs[channel] = make(map[chan Event]struct{})
	}
	b.subs[channel][ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broadcaster) Unsubscribe(channel string, ch chan Event) {
	b.mu.Lock()
	delete(b.subs[channel], ch)
	if len(b.subs[channel]) == 0 {
		delete(b.subs, channel)
	}
	b.mu.Unlock()
	close(ch)
}

// Broadcast is best-effort: subscribers with a full buffer are skipped, not
// blocked on — there is no replay/outbox, matching the DB-notification
// fallback this layer sits on top of.
func (b *Broadcaster) Broadcast(channel, event string, data any) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs[channel] {
		select {
		case ch <- Event{Event: event, Data: data}:
		default:
		}
	}
}

var Default = NewBroadcaster()
