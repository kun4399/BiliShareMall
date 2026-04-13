package events

import "sync"

type Event struct {
	Name    string
	Payload any
}

type Bus struct {
	mu          sync.RWMutex
	nextID      int
	subscribers map[int]chan Event
}

func NewBus() *Bus {
	return &Bus{
		subscribers: map[int]chan Event{},
	}
}

func (b *Bus) Emit(name string, payload any) {
	if b == nil {
		return
	}

	event := Event{
		Name:    name,
		Payload: payload,
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *Bus) Subscribe(buffer int) (<-chan Event, func()) {
	if buffer <= 0 {
		buffer = 16
	}

	ch := make(chan Event, buffer)

	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.subscribers[id] = ch
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		registered, ok := b.subscribers[id]
		if ok {
			delete(b.subscribers, id)
		}
		b.mu.Unlock()

		if ok {
			close(registered)
		}
	}

	return ch, cancel
}
