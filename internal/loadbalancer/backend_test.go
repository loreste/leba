//go:build !integration
package loadbalancer

import (
	"testing"
	"time"
)

func TestBackendCircuitBreaker(t *testing.T) {
	// Create a backend
	backend := &Backend{
		Address:      "127.0.0.1",
		Protocol:     "http",
		Port:         8080,
		Health:       true,
		CircuitState: CircuitClosed,
	}

	// Test initial state
	if !backend.IsAvailable() {
		t.Errorf("Backend should be available initially")
	}

	// Test failure count below threshold
	for i := 0; i < DefaultFailureThreshold-1; i++ {
		backend.MarkFailure()
	}

	// Should still be available as threshold not reached
	if !backend.IsAvailable() {
		t.Errorf("Backend should still be available below failure threshold")
	}

	// Add one more failure to reach threshold
	backend.MarkFailure()

	// Circuit should be open now
	if backend.IsAvailable() {
		t.Errorf("Backend should not be available after reaching failure threshold")
	}

	// Mark success and verify it's still not available due to circuit being open
	backend.MarkSuccess()

	if backend.IsAvailable() {
		t.Errorf("Backend should not be available immediately after success if circuit is open")
	}

	// Manually transition to half-open state
	backend.mu.Lock()
	backend.CircuitState = CircuitHalfOpen
	backend.mu.Unlock()

	// Should be available for trial requests in half-open state
	if !backend.IsAvailable() {
		t.Errorf("Backend should be available for trial requests in half-open state")
	}

	// Mark success, which should close the circuit
	backend.MarkSuccess()

	// Should be available now
	if !backend.IsAvailable() {
		t.Errorf("Backend should be available after success in half-open state")
	}

	// Verify circuit is closed
	if backend.CircuitState != CircuitClosed {
		t.Errorf("Expected circuit state to be closed, got %v", backend.CircuitState)
	}
}

func TestBackendPoolOperations(t *testing.T) {
	pool := NewBackendPool()

	// Add a backend
	backend1 := &Backend{
		Address:      "192.168.1.1",
		Protocol:     "http",
		Port:         80,
		Health:       true,
		CircuitState: CircuitClosed,
	}
	pool.AddBackend(backend1)

	// Add another backend
	backend2 := &Backend{
		Address:      "192.168.1.2",
		Protocol:     "https",
		Port:         443,
		Health:       true,
		CircuitState: CircuitClosed,
	}
	pool.AddBackend(backend2)

	// Test listing backends
	backends := pool.ListBackends()
	if len(backends) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(backends))
	}

	// Test getting available backends
	httpBackends := pool.AvailableBackends("http")
	if len(httpBackends) != 1 {
		t.Errorf("Expected 1 HTTP backend, got %d", len(httpBackends))
	}

	httpsBackends := pool.AvailableBackends("https")
	if len(httpsBackends) != 1 {
		t.Errorf("Expected 1 HTTPS backend, got %d", len(httpsBackends))
	}

	// Mark one backend as unhealthy
	backend1.Health = false

	// Test available backends again
	httpBackends = pool.AvailableBackends("http")
	if len(httpBackends) != 0 {
		t.Errorf("Expected 0 available HTTP backends, got %d", len(httpBackends))
	}

	// Remove a backend
	pool.RemoveBackend(backend1.Address)

	// Check if removed
	backends = pool.ListBackends()
	if len(backends) != 1 {
		t.Errorf("Expected 1 backend after removal, got %d", len(backends))
	}
}

func TestCircuitBreakerTimeouts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Create a backend with a very short timeout for testing
	backend := &Backend{
		Address:        "127.0.0.1",
		Protocol:       "http",
		Port:           8080,
		Health:         true,
		CircuitState:   CircuitClosed,
		CircuitTimeout: 100 * time.Millisecond, // Very short timeout for testing
	}

	// Trigger circuit breaker
	for i := 0; i < DefaultFailureThreshold; i++ {
		backend.MarkFailure()
	}

	// Circuit should be open
	if backend.CircuitState != CircuitOpen {
		t.Errorf("Expected circuit state to be open, got %v", backend.CircuitState)
	}

	// Wait for circuit timeout (plus a small buffer)
	time.Sleep(150 * time.Millisecond)

	// Circuit should transition to half-open
	if backend.CircuitState != CircuitHalfOpen {
		t.Errorf("Expected circuit state to be half-open after timeout, got %v", backend.CircuitState)
	}
}