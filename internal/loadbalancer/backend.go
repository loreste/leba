package loadbalancer

import "sync"

// Backend represents a backend server
type Backend struct {
	Address            string
	Protocol           string
	Port               int
	Weight             int
	Health             bool
	MaxOpenConnections int
	MaxIdleConnections int
	ConnMaxLifetime    int // in seconds
	ActiveConnections  int
	mu                 sync.Mutex
}

// BackendPool represents a pool of backend servers
type BackendPool struct {
	mu       sync.RWMutex
	backends map[string]*Backend
}

// NewBackendPool creates a new BackendPool
func NewBackendPool() *BackendPool {
	return &BackendPool{
		backends: make(map[string]*Backend),
	}
}

// AddBackend adds a backend to the pool
func (p *BackendPool) AddBackend(backend *Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backends[backend.Address] = backend
}

// RemoveBackend removes a backend from the pool
func (p *BackendPool) RemoveBackend(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.backends, address)
}

// ListBackends lists all backends in the pool
func (p *BackendPool) ListBackends() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	backends := make([]*Backend, 0, len(p.backends))
	for _, backend := range p.backends {
		backends = append(backends, backend)
	}
	return backends
}
