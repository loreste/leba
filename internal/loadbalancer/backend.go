package loadbalancer

import (
	"sync"
	"sync/atomic"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int32

const (
	// CircuitClosed means the circuit is closed and requests are allowed through
	CircuitClosed CircuitState = iota
	// CircuitOpen means the circuit is open and requests are blocked
	CircuitOpen
	// CircuitHalfOpen means the circuit is allowing a limited number of test requests
	CircuitHalfOpen
)

// Backend represents a server backend
type Backend struct {
	Address            string
	Protocol           string
	Port               int
	Weight             int
	MaxOpenConnections int
	MaxIdleConnections int
	ConnMaxLifetime    int
	Health             bool
	ActiveConnections  int
	Role               string // primary or replica
	FailureCount       int32
	LastFailure        time.Time
	CircuitState       CircuitState
	CircuitTimeout     time.Duration
	mu                 sync.Mutex
}

// CircuitBreaker threshold values
const (
	DefaultFailureThreshold   = 5        // Number of failures before opening circuit
	DefaultCircuitTimeout     = 10 * time.Second // Time to keep circuit open
	DefaultHalfOpenMaxRetries = 2        // Max retries in half-open state
)

// MarkFailure marks a backend as failed and updates the circuit breaker state
func (b *Backend) MarkFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	atomic.AddInt32(&b.FailureCount, 1)
	b.LastFailure = time.Now()

	// If we've reached the failure threshold, open the circuit
	if atomic.LoadInt32(&b.FailureCount) >= DefaultFailureThreshold {
		if b.CircuitState != CircuitOpen {
			b.CircuitState = CircuitOpen
			b.Health = false
			timeout := DefaultCircuitTimeout
			if b.CircuitTimeout > 0 {
				timeout = b.CircuitTimeout
			}
			
			// Schedule circuit half-open after timeout
			time.AfterFunc(timeout, func() {
				b.mu.Lock()
				defer b.mu.Unlock()
				
				if b.CircuitState == CircuitOpen {
					b.CircuitState = CircuitHalfOpen
					atomic.StoreInt32(&b.FailureCount, 0)
				}
			})
		}
	}
}

// MarkSuccess marks a backend as successful and resets failure count
func (b *Backend) MarkSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// If we were in half-open state, close the circuit
	if b.CircuitState == CircuitHalfOpen {
		b.CircuitState = CircuitClosed
		b.Health = true
	}

	// Reset failure count
	atomic.StoreInt32(&b.FailureCount, 0)
}

// IsAvailable checks if the backend is available for connections
func (b *Backend) IsAvailable() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// If circuit is open, backend is not available
	if b.CircuitState == CircuitOpen {
		return false
	}
	
	// If circuit is half-open, allow limited trials
	if b.CircuitState == CircuitHalfOpen {
		// Allow a few test requests through
		return atomic.LoadInt32(&b.FailureCount) < DefaultHalfOpenMaxRetries
	}
	
	// If circuit is closed, check health
	return b.Health
}

// BackendPool manages a collection of backends
type BackendPool struct {
	backends map[string]*Backend
	mu       sync.RWMutex
}

// NewBackendPool creates a new BackendPool
func NewBackendPool() *BackendPool {
	return &BackendPool{
		backends: make(map[string]*Backend),
	}
}

// AddBackend adds a backend to the pool
func (bp *BackendPool) AddBackend(backend *Backend) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	// Set default circuit timeout if not specified
	if backend.CircuitTimeout == 0 {
		backend.CircuitTimeout = DefaultCircuitTimeout
	}
	
	// Initialize circuit state
	backend.CircuitState = CircuitClosed
	
	bp.backends[backend.Address] = backend
}

// RemoveBackend removes a backend from the pool
func (bp *BackendPool) RemoveBackend(address string) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	delete(bp.backends, address)
}

// GetBackend retrieves a backend by address
func (bp *BackendPool) GetBackend(address string) (*Backend, bool) {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	backend, exists := bp.backends[address]
	return backend, exists
}

// ListBackends lists all backends
func (bp *BackendPool) ListBackends() []*Backend {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	result := make([]*Backend, 0, len(bp.backends))
	for _, backend := range bp.backends {
		result = append(result, backend)
	}
	return result
}

// AvailableBackends returns a list of available backends for a protocol
func (bp *BackendPool) AvailableBackends(protocol string) []*Backend {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	
	result := make([]*Backend, 0)
	for _, backend := range bp.backends {
		if backend.Protocol == protocol && backend.IsAvailable() {
			result = append(result, backend)
		}
	}
	return result
}

// HealthyBackendCount returns the count of healthy backends for a protocol
func (bp *BackendPool) HealthyBackendCount(protocol string) int {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	
	count := 0
	for _, backend := range bp.backends {
		if backend.Protocol == protocol && backend.IsAvailable() {
			count++
		}
	}
	return count
}