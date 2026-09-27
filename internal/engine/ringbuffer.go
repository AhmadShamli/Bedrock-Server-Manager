package engine

import (
	"sync"
)

// RingBuffer is a thread-safe circular buffer holding a fixed number of string lines.
type RingBuffer struct {
	mu       sync.RWMutex
	capacity int
	lines    []string
	start    int
	count    int
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &RingBuffer{
		capacity: capacity,
		lines:    make([]string, capacity),
	}
}

// Write appends a line to the ring buffer.
func (rb *RingBuffer) Write(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count < rb.capacity {
		rb.lines[rb.count] = line
		rb.count++
	} else {
		rb.lines[rb.start] = line
		rb.start = (rb.start + 1) % rb.capacity
	}
}

// GetAll returns all stored lines in chronological order.
func (rb *RingBuffer) GetAll() []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	result := make([]string, rb.count)
	for i := 0; i < rb.count; i++ {
		idx := (rb.start + i) % rb.capacity
		result[i] = rb.lines[idx]
	}
	return result
}

// Count returns the number of lines currently stored.
func (rb *RingBuffer) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}
