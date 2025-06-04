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

// Backend represents a server backend with optimized atomic fields
type Backend struct {
	Address            string
	Protocol           string
	Port               int
	Weight             int
	MaxOpenConnections int
	MaxIdleConnections int
	ConnMaxLifetime    int

	// Atomic fields - must be 64-bit aligned
	ActiveConnections atomic.Int64
	FailureCount      atomic.Int32
	circuitState      atomic.Int32 // CircuitState as atomic
	health            atomic.Bool  // Health status as atomic
	lastFailure       atomic.Int64 // Unix timestamp

	// Non-atomic fields
	Role           string // primary or replica
	CircuitTimeout time.Duration
	mu             sync.Mutex // Only for complex state transitions
	Healthy        bool       // For compatibility with lock-free pool
	Health         bool       // Keep for backward compatibility
}

// CircuitBreaker threshold values
const (
	DefaultFailureThreshold   = 5                // Number of failures before opening circuit
	DefaultCircuitTimeout     = 10 * time.Second // Time to keep circuit open
	DefaultHalfOpenMaxRetries = 2                // Max retries in half-open state
)

// MarkFailure marks a backend as failed and updates the circuit breaker state
func (b *Backend) MarkFailure() {
	b.FailureCount.Add(1)
	b.lastFailure.Store(time.Now().Unix())

	// If we've reached the failure threshold, open the circuit
	if b.FailureCount.Load() >= DefaultFailureThreshold {
		// Try to transition from closed to open atomically
		if b.circuitState.CompareAndSwap(int32(CircuitClosed), int32(CircuitOpen)) {
			b.health.Store(false)
			b.Health = false // Keep sync for compatibility
			timeout := DefaultCircuitTimeout
			if b.CircuitTimeout > 0 {
				timeout = b.CircuitTimeout
			}

			// Schedule circuit half-open after timeout (no locks needed)
			go func() {
				time.Sleep(timeout)
				// Try to transition from open to half-open
				if b.circuitState.CompareAndSwap(int32(CircuitOpen), int32(CircuitHalfOpen)) {
					b.FailureCount.Store(0)
				}
			}()
		}
	}
}

// MarkSuccess marks a backend as successful and resets failure count
func (b *Backend) MarkSuccess() {
	// If we were in half-open state, close the circuit atomically
	if b.circuitState.CompareAndSwap(int32(CircuitHalfOpen), int32(CircuitClosed)) {
		b.health.Store(true)
		b.Health = true // Keep sync for compatibility
	}

	// Reset failure count atomically
	b.FailureCount.Store(0)
}

// IsAvailable checks if the backend is available for connections (lock-free)
func (b *Backend) IsAvailable() bool {
	state := CircuitState(b.circuitState.Load())

	// If circuit is open, backend is not available
	if state == CircuitOpen {
		return false
	}

	// If circuit is half-open, allow limited trials
	if state == CircuitHalfOpen {
		// Allow a few test requests through
		return b.FailureCount.Load() < DefaultHalfOpenMaxRetries
	}

	// If circuit is closed, check health
	return b.health.Load()
}

// GetCircuitState returns the current circuit state
func (b *Backend) GetCircuitState() CircuitState {
	return CircuitState(b.circuitState.Load())
}

// IsHealthy returns the health status of the backend (thread-safe)
func (b *Backend) IsHealthy() bool {
	return b.health.Load()
}

// SetHealthy sets the health status of the backend (thread-safe)
func (b *Backend) SetHealthy(healthy bool) {
	b.health.Store(healthy)
	// Keep backward compatibility fields in sync
	b.mu.Lock()
	b.Health = healthy
	b.Healthy = healthy
	b.mu.Unlock()
}

// BackendPool manages a collection of backends
type BackendPool struct {
	backends     map[string]*Backend
	mu           sync.RWMutex
	lockFreePool *LockFreeBackendPool // Lock-free pool for high-performance selection
}

// NewBackendPool creates a new BackendPool
func NewBackendPool() *BackendPool {
	return &BackendPool{
		backends:     make(map[string]*Backend),
		lockFreePool: NewLockFreeBackendPool(),
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

	// Initialize atomic fields
	backend.circuitState.Store(int32(CircuitClosed))
	backend.health.Store(backend.Health)
	backend.Healthy = backend.Health
	// Initialize other atomic counters if needed
	if backend.FailureCount.Load() == 0 && backend.Health {
		backend.FailureCount.Store(0)
	}

	bp.backends[backend.Address] = backend

	// Update lock-free pool
	bp.updateLockFreePool()
}

// RemoveBackend removes a backend from the pool
func (bp *BackendPool) RemoveBackend(address string) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	delete(bp.backends, address)

	// Update lock-free pool
	bp.updateLockFreePool()
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

// updateLockFreePool updates the lock-free pool with current backends
func (bp *BackendPool) updateLockFreePool() {
	// This method is called with bp.mu.Lock held
	backends := make([]*Backend, 0, len(bp.backends))
	for _, backend := range bp.backends {
		// Sync health status
		backend.Healthy = backend.IsAvailable()
		backends = append(backends, backend)
	}
	bp.lockFreePool.UpdateBackends(backends)
}

// SelectBackend selects a backend using the configured algorithm (lock-free)
func (bp *BackendPool) SelectBackend(protocol string) *Backend {
	// Use lock-free selection for high performance
	return bp.lockFreePool.SelectBackendP2C(protocol)
}

// SelectBackendLeastConn selects backend with least connections
func (bp *BackendPool) SelectBackendLeastConn(protocol string) *Backend {
	return bp.lockFreePool.SelectBackendLeastConn(protocol)
}

// SelectBackendRoundRobin selects backend using round-robin
func (bp *BackendPool) SelectBackendRoundRobin(protocol string) *Backend {
	return bp.lockFreePool.SelectBackendRoundRobin(protocol)
}

// SelectBackendWeighted selects backend using weighted algorithm
func (bp *BackendPool) SelectBackendWeighted(protocol string) *Backend {
	return bp.lockFreePool.SelectBackendWeighted(protocol)
}

// HandleFailure records a failure for a backend
func (bp *BackendPool) HandleFailure(backend *Backend) {
	backend.MarkFailure()
	bp.lockFreePool.RecordFailure(backend)

	// Update lock-free pool if backend state changed
	if CircuitState(backend.circuitState.Load()) == CircuitOpen {
		bp.mu.Lock()
		bp.updateLockFreePool()
		bp.mu.Unlock()
	}
}

// HandleSuccess records a success for a backend
func (bp *BackendPool) HandleSuccess(backend *Backend) {
	backend.MarkSuccess()
	bp.lockFreePool.ResetFailures(backend)

	// Update lock-free pool if backend state changed
	if CircuitState(backend.circuitState.Load()) == CircuitClosed {
		bp.mu.Lock()
		bp.updateLockFreePool()
		bp.mu.Unlock()
	}
}
