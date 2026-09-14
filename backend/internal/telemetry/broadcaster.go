package telemetry

import (
	"encoding/json"
	"sync"

	"github.com/KaungSat3203/statuspulse/backend/internal/models"
)

const defaultSubscriberBufferSize = 16

type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan []byte]struct{}
	bufferSize  int
}

func NewBroadcaster() *Broadcaster {
	return NewBroadcasterWithBuffer(defaultSubscriberBufferSize)
}

func NewBroadcasterWithBuffer(bufferSize int) *Broadcaster {
	if bufferSize < 1 {
		bufferSize = 1
	}

	return &Broadcaster{
		subscribers: make(map[chan []byte]struct{}),
		bufferSize:  bufferSize,
	}
}

func (b *Broadcaster) Subscribe() (<-chan []byte, func()) {
	subscriber := make(chan []byte, b.bufferSize)

	b.mu.Lock()
	b.subscribers[subscriber] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			if _, ok := b.subscribers[subscriber]; !ok {
				return
			}

			delete(b.subscribers, subscriber)
			close(subscriber)
		})
	}

	return subscriber, unsubscribe
}

func (b *Broadcaster) Publish(result models.CheckResult) {
	payload, err := json.Marshal(result)
	if err != nil {
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for subscriber := range b.subscribers {
		deliver(subscriber, payload)
	}
}

func (b *Broadcaster) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.subscribers)
}

func deliver(subscriber chan []byte, payload []byte) {
	message := append([]byte(nil), payload...)

	select {
	case subscriber <- message:
		return
	default:
	}

	select {
	case <-subscriber:
	default:
	}

	select {
	case subscriber <- message:
	default:
	}
}
