package loadbalancer

import (
	"sync"
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

// GetBackend retrieves a specific backend by address
func (bp *BackendPool) GetBackend(address string) (*Backend, bool) {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	backend, exists := bp.backends[address]
	return backend, exists
}

// AddOrUpdateBackend adds a backend if it doesn't exist or updates it if it does
func (bp *BackendPool) AddOrUpdateBackend(newBackend *Backend) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	existingBackend, exists := bp.backends[newBackend.Address]
	if exists {
		// Update existing backend properties
		existingBackend.mu.Lock()
		defer existingBackend.mu.Unlock()

		existingBackend.Protocol = newBackend.Protocol
		existingBackend.Port = newBackend.Port
		existingBackend.Weight = newBackend.Weight
		existingBackend.MaxOpenConnections = newBackend.MaxOpenConnections
		existingBackend.MaxIdleConnections = newBackend.MaxIdleConnections
		existingBackend.ConnMaxLifetime = newBackend.ConnMaxLifetime
		existingBackend.Health = newBackend.Health
		existingBackend.Role = newBackend.Role
		existingBackend.ActiveConnections = newBackend.ActiveConnections
	} else {
		// Add new backend
		bp.backends[newBackend.Address] = newBackend
	}
}
