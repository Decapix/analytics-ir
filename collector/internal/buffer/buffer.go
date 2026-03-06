package buffer

import (
	"sync"
	"time"

	"analytics-ir/event-collector/internal/model"
)

type BatchBuffer struct {
	mu    sync.Mutex
	events []model.AnalyticsEvent
}

func NewBatchBuffer(capacity int) *BatchBuffer {
	return &BatchBuffer{events: make([]model.AnalyticsEvent, 0, capacity)}
}

func (b *BatchBuffer) Add(event model.AnalyticsEvent) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
	return len(b.events)
}

func (b *BatchBuffer) Drain() []model.AnalyticsEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.events) == 0 {
		return nil
	}
	batch := make([]model.AnalyticsEvent, len(b.events))
	copy(batch, b.events)
	b.events = b.events[:0]
	return batch
}

func (b *BatchBuffer) StartPeriodicFlush(interval time.Duration, flushFn func()) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			flushFn()
		}
	}()
}
