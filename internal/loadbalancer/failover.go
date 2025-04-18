package loadbalancer

import (
	"log"
	"sync"
	"time"
)

// FailoverMode indicates the current mode for a backend group
type FailoverMode int

const (
	// NormalMode means all backends in the group are used
	NormalMode FailoverMode = iota
	// ActivePassiveMode means only the primary is used unless it fails
	ActivePassiveMode
)

// BackendGroup represents a group of backends with failover capability
type BackendGroup struct {
	Name         string
	Backends     map[string]*Backend // Map of backend address to backend
	PrimaryKey   string              // Key of the current primary backend
	Mode         FailoverMode
	LastFailover time.Time
	mu           sync.RWMutex
}

// FailoverManager manages backend failover
type FailoverManager struct {
	groups      map[string]*BackendGroup
	mu          sync.RWMutex
	backendPool *BackendPool
	checkTicker *time.Ticker
	done        chan struct{}
}

// NewFailoverManager creates a new failover manager
func NewFailoverManager(backendPool *BackendPool) *FailoverManager {
	fm := &FailoverManager{
		groups:      make(map[string]*BackendGroup),
		backendPool: backendPool,
		done:        make(chan struct{}),
	}
	
	// Start the failover checker
	fm.checkTicker = time.NewTicker(5 * time.Second)
	go fm.monitorBackends()
	
	return fm
}

// CreateBackendGroup creates a new backend group
func (fm *FailoverManager) CreateBackendGroup(name string, mode FailoverMode) *BackendGroup {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	
	group := &BackendGroup{
		Name:     name,
		Backends: make(map[string]*Backend),
		Mode:     mode,
	}
	
	fm.groups[name] = group
	return group
}

// GetBackendGroup gets a backend group by name
func (fm *FailoverManager) GetBackendGroup(name string) *BackendGroup {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	
	return fm.groups[name]
}

// AddBackendToGroup adds a backend to a group
func (fm *FailoverManager) AddBackendToGroup(groupName, backendAddr string, isPrimary bool) bool {
	fm.mu.RLock()
	group, exists := fm.groups[groupName]
	fm.mu.RUnlock()
	
	if !exists {
		return false
	}
	
	// Get backend from pool
	fm.backendPool.mu.RLock()
	backend, exists := fm.backendPool.backends[backendAddr]
	fm.backendPool.mu.RUnlock()
	
	if !exists {
		return false
	}
	
	group.mu.Lock()
	defer group.mu.Unlock()
	
	group.Backends[backendAddr] = backend
	
	// Set as primary if requested or if this is the first backend
	if isPrimary || group.PrimaryKey == "" {
		group.PrimaryKey = backendAddr
	}
	
	log.Printf("Added backend %s to group %s (primary: %v)", 
		backendAddr, groupName, isPrimary)
	
	return true
}

// RemoveBackendFromGroup removes a backend from a group
func (fm *FailoverManager) RemoveBackendFromGroup(groupName, backendAddr string) bool {
	fm.mu.RLock()
	group, exists := fm.groups[groupName]
	fm.mu.RUnlock()
	
	if !exists {
		return false
	}
	
	group.mu.Lock()
	defer group.mu.Unlock()
	
	// Check if backend exists in group
	_, exists = group.Backends[backendAddr]
	if !exists {
		return false
	}
	
	// Remove backend
	delete(group.Backends, backendAddr)
	
	// If we removed the primary, elect a new one
	if group.PrimaryKey == backendAddr {
		for addr, backend := range group.Backends {
			if backend.IsAvailable() {
				group.PrimaryKey = addr
				group.LastFailover = time.Now()
				log.Printf("Elected new primary %s in group %s", addr, groupName)
				break
			}
		}
		
		// If no suitable primary found, clear it
		if group.PrimaryKey == backendAddr {
			group.PrimaryKey = ""
		}
	}
	
	return true
}

// GetBackendForGroup gets a backend for a group based on the failover mode
func (fm *FailoverManager) GetBackendForGroup(groupName string) *Backend {
	fm.mu.RLock()
	group, exists := fm.groups[groupName]
	fm.mu.RUnlock()
	
	if !exists {
		return nil
	}
	
	group.mu.RLock()
	defer group.mu.RUnlock()
	
	// In active-passive mode, use primary if available
	if group.Mode == ActivePassiveMode {
		if group.PrimaryKey != "" {
			primary := group.Backends[group.PrimaryKey]
			if primary != nil && primary.IsAvailable() {
				return primary
			}
		}
		
		// Primary not available, find a replica
		for addr, backend := range group.Backends {
			if addr != group.PrimaryKey && backend.IsAvailable() {
				return backend
			}
		}
		
		return nil
	}
	
	// In normal mode, find any available backend
	// Prefer primary if available
	if group.PrimaryKey != "" {
		primary := group.Backends[group.PrimaryKey]
		if primary != nil && primary.IsAvailable() {
			return primary
		}
	}
	
	// Try any available backend
	for _, backend := range group.Backends {
		if backend.IsAvailable() {
			return backend
		}
	}
	
	return nil
}

// SetGroupMode sets the failover mode for a group
func (fm *FailoverManager) SetGroupMode(groupName string, mode FailoverMode) bool {
	fm.mu.RLock()
	group, exists := fm.groups[groupName]
	fm.mu.RUnlock()
	
	if !exists {
		return false
	}
	
	group.mu.Lock()
	group.Mode = mode
	group.mu.Unlock()
	
	log.Printf("Set failover mode for group %s to %v", groupName, mode)
	return true
}

// SetPrimary sets the primary backend for a group
func (fm *FailoverManager) SetPrimary(groupName, backendAddr string) bool {
	fm.mu.RLock()
	group, exists := fm.groups[groupName]
	fm.mu.RUnlock()
	
	if !exists {
		return false
	}
	
	group.mu.Lock()
	defer group.mu.Unlock()
	
	// Check if backend exists in group
	_, exists = group.Backends[backendAddr]
	if !exists {
		return false
	}
	
	group.PrimaryKey = backendAddr
	log.Printf("Set primary for group %s to %s", groupName, backendAddr)
	
	return true
}

// monitorBackends monitors backend health and initiates failover if needed
func (fm *FailoverManager) monitorBackends() {
	for {
		select {
		case <-fm.checkTicker.C:
			fm.checkGroupHealth()
		case <-fm.done:
			return
		}
	}
}

// checkGroupHealth checks health of all backend groups
func (fm *FailoverManager) checkGroupHealth() {
	fm.mu.RLock()
	groups := make([]*BackendGroup, 0, len(fm.groups))
	for _, group := range fm.groups {
		groups = append(groups, group)
	}
	fm.mu.RUnlock()
	
	for _, group := range groups {
		fm.checkSingleGroup(group)
	}
}

// checkSingleGroup checks health of a single backend group
func (fm *FailoverManager) checkSingleGroup(group *BackendGroup) {
	group.mu.Lock()
	defer group.mu.Unlock()
	
	// If no primary, try to elect one
	if group.PrimaryKey == "" {
		for addr, backend := range group.Backends {
			if backend.IsAvailable() {
				group.PrimaryKey = addr
				group.LastFailover = time.Now()
				log.Printf("Elected new primary %s in group %s", addr, group.Name)
				break
			}
		}
		return
	}
	
	// Check if primary is healthy
	primary, exists := group.Backends[group.PrimaryKey]
	if !exists || !primary.IsAvailable() {
		// Primary is down, find a new one
		for addr, backend := range group.Backends {
			if addr != group.PrimaryKey && backend.IsAvailable() {
				group.PrimaryKey = addr
				group.LastFailover = time.Now()
				log.Printf("Failover in group %s: Primary %s failed, new primary is %s",
					group.Name, primary.Address, addr)
				break
			}
		}
		
		// If no suitable primary found, clear it
		if !exists || !group.Backends[group.PrimaryKey].IsAvailable() {
			log.Printf("No healthy backends found in group %s", group.Name)
			group.PrimaryKey = ""
		}
	}
}

// Stop stops the failover manager
func (fm *FailoverManager) Stop() {
	close(fm.done)
	fm.checkTicker.Stop()
}

// ListGroups lists all backend groups
func (fm *FailoverManager) ListGroups() []string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	
	groups := make([]string, 0, len(fm.groups))
	for name := range fm.groups {
		groups = append(groups, name)
	}
	
	return groups
}

// GetGroupStatus gets status information for a group
func (fm *FailoverManager) GetGroupStatus(groupName string) map[string]interface{} {
	fm.mu.RLock()
	group, exists := fm.groups[groupName]
	fm.mu.RUnlock()
	
	if !exists {
		return nil
	}
	
	group.mu.RLock()
	defer group.mu.RUnlock()
	
	// Count healthy backends
	healthyCount := 0
	for _, backend := range group.Backends {
		if backend.IsAvailable() {
			healthyCount++
		}
	}
	
	primaryHealthy := false
	if group.PrimaryKey != "" {
		if primary, exists := group.Backends[group.PrimaryKey]; exists {
			primaryHealthy = primary.IsAvailable()
		}
	}
	
	return map[string]interface{}{
		"name":             group.Name,
		"mode":             group.Mode,
		"primary_key":      group.PrimaryKey,
		"primary_healthy":  primaryHealthy,
		"total_backends":   len(group.Backends),
		"healthy_backends": healthyCount,
		"last_failover":    group.LastFailover.Format(time.RFC3339),
	}
}