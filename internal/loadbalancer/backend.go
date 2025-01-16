package loadbalancer

import (
	"sync"
)

// Backend represents a server backend
// Updated to include Role for read/write differentiation
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
	mu                 sync.Mutex
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

// NewBackend creates a new Backend instance
func NewBackend(address, protocol string, port, weight int, role string) *Backend {
	return &Backend{
		Address:  address,
		Protocol: protocol,
		Port:     port,
		Weight:   weight,
		Role:     role,
		Health:   true, // Default to healthy
	}
}

// AddBackend adds a backend to the pool
func (bp *BackendPool) AddBackend(backend *Backend) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.backends[backend.Address] = backend
}

// RemoveBackend removes a backend from the pool
func (bp *BackendPool) RemoveBackend(address string) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	delete(bp.backends, address)
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

// GetBackend retrieves a backend by its address
func (bp *BackendPool) GetBackend(address string) (*Backend, bool) {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	backend, exists := bp.backends[address]
	return backend, exists
}

// UpdateHealth updates the health status of a backend
func (bp *BackendPool) UpdateHealth(address string, healthy bool) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	if backend, exists := bp.backends[address]; exists {
		backend.Health = healthy
	}
}
