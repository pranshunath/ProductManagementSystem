package workers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// EventHandler is the signature for functions that process a domain event
type EventHandler func(ctx context.Context, e Event) error

// PoolStats provides metrics on worker pool health and throughput
type PoolStats struct {
	WorkerCount    int   `json:"worker_count"`
	QueueCapacity  int   `json:"queue_capacity"`
	QueueLength    int   `json:"queue_length"`
	ProcessedCount int64 `json:"processed_count"`
	FailedCount    int64 `json:"failed_count"`
	IsRunning      bool  `json:"is_running"`
}

// WorkerPool manages a pool of background worker goroutines consuming from a buffered channel
type WorkerPool struct {
	workerCount    int
	queueCapacity  int
	queue          chan Event
	handlers       map[EventType][]EventHandler
	handlersMu     sync.RWMutex
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	isRunning      atomic.Bool
	stopOnce       sync.Once
	processedCount int64
	failedCount    int64
}

// NewWorkerPool creates a new WorkerPool with specified worker count and channel capacity
func NewWorkerPool(workers int, queueCap int) *WorkerPool {
	if workers <= 0 {
		workers = 4
	}
	if queueCap <= 0 {
		queueCap = 100
	}

	return &WorkerPool{
		workerCount:   workers,
		queueCapacity: queueCap,
		queue:         make(chan Event, queueCap),
		handlers:      make(map[EventType][]EventHandler),
	}
}

// RegisterHandler binds an event handler to a specific EventType
func (p *WorkerPool) RegisterHandler(eventType EventType, handler EventHandler) {
	p.handlersMu.Lock()
	defer p.handlersMu.Unlock()
	p.handlers[eventType] = append(p.handlers[eventType], handler)
}

// Start launches worker goroutines
func (p *WorkerPool) Start(parentCtx context.Context) {
	if p.isRunning.Swap(true) {
		return // Already running
	}

	p.ctx, p.cancel = context.WithCancel(parentCtx)

	log.Printf("[WORKER POOL] Starting %d background worker goroutines (Queue Buffer: %d)", p.workerCount, p.queueCapacity)

	for i := 1; i <= p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Dispatch queues an event for background processing without blocking the caller
func (p *WorkerPool) Dispatch(event Event) bool {
	if !p.isRunning.Load() {
		log.Printf("[WORKER POOL] Cannot dispatch event: worker pool is not running")
		return false
	}

	select {
	case p.queue <- event:
		return true
	default:
		// Queue full: log backpressure alert and track failed delivery
		log.Printf("[WORKER POOL BACKPRESSURE] Event queue is FULL (%d/%d). Dropping event %s [%s]",
			len(p.queue), p.queueCapacity, event.ID, event.Type)
		atomic.AddInt64(&p.failedCount, 1)
		return false
	}
}

// worker is the worker routine that consumes events from the buffered channel until closed
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for event := range p.queue {
		p.processEvent(id, event)
	}
}

// processEvent executes all registered handlers for the given event
func (p *WorkerPool) processEvent(workerID int, event Event) {
	p.handlersMu.RLock()
	handlers, exists := p.handlers[event.Type]
	p.handlersMu.RUnlock()

	if !exists || len(handlers) == 0 {
		// No handlers registered for this event type; count as processed
		atomic.AddInt64(&p.processedCount, 1)
		return
	}

	eventCtx, cancel := context.WithTimeout(p.ctx, 10*time.Second)
	defer cancel()

	hasError := false
	for _, handler := range handlers {
		if err := handler(eventCtx, event); err != nil {
			log.Printf("[WORKER %d ERROR] Failed processing event %s [%s]: %v", workerID, event.ID, event.Type, err)
			hasError = true
		}
	}

	if hasError {
		atomic.AddInt64(&p.failedCount, 1)
	} else {
		atomic.AddInt64(&p.processedCount, 1)
	}
}

// Stop gracefully shuts down the worker pool, draining remaining queued events
func (p *WorkerPool) Stop(timeout time.Duration) error {
	var err error
	p.stopOnce.Do(func() {
		p.isRunning.Store(false)
		close(p.queue) // Closing channel signals workers to drain remaining items

		done := make(chan struct{})
		go func() {
			p.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			log.Printf("[WORKER POOL] Gracefully drained and stopped all %d workers. Processed: %d, Failed: %d",
				p.workerCount, atomic.LoadInt64(&p.processedCount), atomic.LoadInt64(&p.failedCount))
		case <-time.After(timeout):
			if p.cancel != nil {
				p.cancel()
			}
			err = fmt.Errorf("worker pool shutdown timed out after %v", timeout)
			log.Printf("[WORKER POOL WARNING] %v", err)
		}
	})
	return err
}

// Stats returns a snapshot of worker pool metrics
func (p *WorkerPool) Stats() PoolStats {
	return PoolStats{
		WorkerCount:    p.workerCount,
		QueueCapacity:  p.queueCapacity,
		QueueLength:    len(p.queue),
		ProcessedCount: atomic.LoadInt64(&p.processedCount),
		FailedCount:    atomic.LoadInt64(&p.failedCount),
		IsRunning:      p.isRunning.Load(),
	}
}

// ErrQueueFull is returned when dispatch fails due to full buffer
var ErrQueueFull = errors.New("worker pool event queue is full")
