package loadbalancer

import (
	"fmt"
	"log"
	"strconv"
)

// AddOrUpdateBackend adds a new backend or updates an existing one.
func (bp *BackendPool) AddOrUpdateBackend(newBackend *Backend) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	key := bp.backendKey(newBackend)
	if existingBackend, exists := bp.backends[key]; exists {
		log.Printf("Updating backend: %s", key)
		existingBackend.mu.Lock()
		existingBackend.Weight = newBackend.Weight
		existingBackend.Health = newBackend.Health
		existingBackend.MaxOpenConnections = newBackend.MaxOpenConnections
		existingBackend.MaxIdleConnections = newBackend.MaxIdleConnections
		existingBackend.Role = newBackend.Role
		existingBackend.mu.Unlock()
	} else {
		log.Printf("Adding new backend: %s", key)
		bp.backends[key] = newBackend
	}
}

// SyncBackends synchronizes backend states with external sources (e.g., peers).
func (bp *BackendPool) SyncBackends(newBackends []*Backend) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	for _, newBackend := range newBackends {
		key := bp.backendKey(newBackend)
		if existingBackend, exists := bp.backends[key]; exists {
			log.Printf("Synchronizing backend: %s", key)
			existingBackend.Health = newBackend.Health
			existingBackend.ActiveConnections = newBackend.ActiveConnections
			existingBackend.Weight = newBackend.Weight
			existingBackend.Role = newBackend.Role
		} else {
			log.Printf("Adding new backend during synchronization: %s", key)
			bp.backends[key] = newBackend
		}
	}
}

// SelectBackend selects a healthy backend for the specified protocol.
func (bp *BackendPool) SelectBackend(protocol string) (*Backend, error) {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	var selectedBackend *Backend
	for _, backend := range bp.backends {
		if backend.Protocol == protocol && backend.Health {
			if selectedBackend == nil || backend.ActiveConnections < selectedBackend.ActiveConnections {
				selectedBackend = backend
			}
		}
	}

	if selectedBackend == nil {
		return nil, fmt.Errorf("no healthy backend available for protocol: %s", protocol)
	}
	return selectedBackend, nil
}

// IncrementActiveConnections increments the active connections for a backend.
func (bp *BackendPool) IncrementActiveConnections(address string, port int) error {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	key := bp.backendKey(&Backend{Address: address, Port: port})
	backend, exists := bp.backends[key]
	if !exists {
		return fmt.Errorf("backend %s not found", key)
	}

	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.ActiveConnections++
	return nil
}

// DecrementActiveConnections decrements the active connections for a backend.
func (bp *BackendPool) DecrementActiveConnections(address string, port int) error {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	key := bp.backendKey(&Backend{Address: address, Port: port})
	backend, exists := bp.backends[key]
	if !exists {
		return fmt.Errorf("backend %s not found", key)
	}

	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.ActiveConnections > 0 {
		backend.ActiveConnections--
	}
	return nil
}

// backendKey generates a unique key for a backend.
func (bp *BackendPool) backendKey(backend *Backend) string {
	return backend.Address + ":" + strconv.Itoa(backend.Port)
}
