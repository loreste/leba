package loadbalancer

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DNSResolver defines the interface for DNS resolution
type DNSResolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// DNSBackendConfig represents a DNS-based backend configuration
type DNSBackendConfig struct {
	// Domain name to resolve
	Domain string
	// Protocol to use
	Protocol string
	// Port to use
	Port int
	// Weight for load balancing
	Weight int
	// Whether this is a primary backend
	IsPrimary bool
	// Group name for failover
	GroupName string
	// Timeout for DNS resolution
	ResolutionTimeout time.Duration
	// Interval between DNS resolutions
	ResolutionInterval time.Duration
	// TTL for DNS entries (overrides DNS TTL if positive)
	TTL time.Duration
	// Last resolution time
	LastResolution time.Time
	// Last resolved addresses
	LastAddresses []string
}

// DNSDiscoveryManager manages DNS-based service discovery
type DNSDiscoveryManager struct {
	configs     map[string]*DNSBackendConfig
	resolver    DNSResolver
	backendPool *BackendPool
	failoverMgr *FailoverManager
	mu          sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewDNSDiscoveryManager creates a new DNS discovery manager
func NewDNSDiscoveryManager(backendPool *BackendPool, failoverMgr *FailoverManager) *DNSDiscoveryManager {
	return &DNSDiscoveryManager{
		configs:     make(map[string]*DNSBackendConfig),
		resolver:    net.DefaultResolver,
		backendPool: backendPool,
		failoverMgr: failoverMgr,
		stopCh:      make(chan struct{}),
	}
}

// AddDNSBackend adds a DNS-based backend for discovery
func (dm *DNSDiscoveryManager) AddDNSBackend(config *DNSBackendConfig) {
	if config.ResolutionInterval == 0 {
		config.ResolutionInterval = 30 * time.Second
	}
	if config.ResolutionTimeout == 0 {
		config.ResolutionTimeout = 5 * time.Second
	}

	dm.mu.Lock()
	dm.configs[config.Domain] = config
	dm.mu.Unlock()

	// Immediately resolve
	dm.resolveDomain(config)

	log.Printf("Added DNS backend: %s:%d (%s)", config.Domain, config.Port, config.Protocol)
}

// RemoveDNSBackend removes a DNS-based backend
func (dm *DNSDiscoveryManager) RemoveDNSBackend(domain string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	config, exists := dm.configs[domain]
	if !exists {
		return
	}

	// Remove all backends for this domain
	if dm.backendPool != nil {
		for _, addr := range config.LastAddresses {
			backendID := fmt.Sprintf("%s:%s:%d", domain, addr, config.Port)
			dm.backendPool.RemoveBackend(backendID)

			// Remove from failover group if necessary
			if config.GroupName != "" && dm.failoverMgr != nil {
				group := dm.failoverMgr.GetBackendGroup(config.GroupName)
				if group != nil {
					dm.failoverMgr.RemoveBackendFromGroup(config.GroupName, backendID)
				}
			}
		}
	}

	delete(dm.configs, domain)
	log.Printf("Removed DNS backend: %s", domain)
}

// GetDNSBackendConfig gets a DNS backend configuration
func (dm *DNSDiscoveryManager) GetDNSBackendConfig(domain string) *DNSBackendConfig {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.configs[domain]
}

// ListDNSBackends lists all DNS backends
func (dm *DNSDiscoveryManager) ListDNSBackends() []*DNSBackendConfig {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := make([]*DNSBackendConfig, 0, len(dm.configs))
	for _, config := range dm.configs {
		// Make a copy to avoid race conditions
		configCopy := *config
		result = append(result, &configCopy)
	}

	return result
}

// Start starts the DNS discovery process
func (dm *DNSDiscoveryManager) Start() {
	dm.wg.Add(1)
	go func() {
		defer dm.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				dm.resolveAllDomains()
			case <-dm.stopCh:
				log.Println("Stopping DNS discovery")
				return
			}
		}
	}()

	log.Println("Started DNS service discovery")
}

// Stop stops the DNS discovery process
func (dm *DNSDiscoveryManager) Stop() {
	close(dm.stopCh)
	dm.wg.Wait()
}

// resolveAllDomains resolves all registered domains
func (dm *DNSDiscoveryManager) resolveAllDomains() {
	dm.mu.RLock()
	configs := make([]*DNSBackendConfig, 0, len(dm.configs))
	for _, config := range dm.configs {
		configs = append(configs, config)
	}
	dm.mu.RUnlock()

	for _, config := range configs {
		// Check if it's time to resolve
		if time.Since(config.LastResolution) >= config.ResolutionInterval {
			dm.resolveDomain(config)
		}
	}
}

// resolveDomain resolves a single domain and updates backends
func (dm *DNSDiscoveryManager) resolveDomain(config *DNSBackendConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), config.ResolutionTimeout)
	defer cancel()

	// Use DNS resolver to get IP addresses
	addrs, err := dm.resolver.LookupHost(ctx, config.Domain)
	if err != nil {
		log.Printf("Failed to resolve domain %s: %v", config.Domain, err)
		return
	}

	// Update last resolution time and addresses
	dm.mu.Lock()
	oldAddrs := make([]string, len(config.LastAddresses))
	copy(oldAddrs, config.LastAddresses)
	config.LastResolution = time.Now()
	config.LastAddresses = addrs
	dm.mu.Unlock()

	// Find removed addresses
	removedAddrs := findRemovedItems(oldAddrs, addrs)
	for _, addr := range removedAddrs {
		backendID := fmt.Sprintf("%s:%s:%d", config.Domain, addr, config.Port)
		dm.backendPool.RemoveBackend(backendID)

		if config.GroupName != "" && dm.failoverMgr != nil {
			dm.failoverMgr.RemoveBackendFromGroup(config.GroupName, backendID)
		}

		log.Printf("Removed backend %s (DNS TTL expired)", backendID)
	}

	// Find new addresses
	addedAddrs := findAddedItems(oldAddrs, addrs)
	for _, addr := range addedAddrs {
		backendID := fmt.Sprintf("%s:%s:%d", config.Domain, addr, config.Port)

		// Add to backend pool
		backend := &Backend{
			Address:      addr,
			Protocol:     config.Protocol,
			Port:         config.Port,
			Weight:       config.Weight,
			Health:       true,
			CircuitState: CircuitClosed,
		}

		dm.backendPool.AddBackend(backend)

		// Add to failover group if necessary
		if config.GroupName != "" && dm.failoverMgr != nil {
			group := dm.failoverMgr.GetBackendGroup(config.GroupName)
			if group == nil {
				// Create group if it doesn't exist
				dm.failoverMgr.CreateBackendGroup(config.GroupName, ActivePassiveMode)
			}
			dm.failoverMgr.AddBackendToGroup(config.GroupName, backendID, config.IsPrimary)
		}

		log.Printf("Added backend %s from DNS resolution", backendID)
	}
}

// findRemovedItems finds items in oldList that are not in newList
func findRemovedItems(oldList, newList []string) []string {
	result := make([]string, 0)
	for _, old := range oldList {
		found := false
		for _, new := range newList {
			if old == new {
				found = true
				break
			}
		}
		if !found {
			result = append(result, old)
		}
	}
	return result
}

// findAddedItems finds items in newList that are not in oldList
func findAddedItems(oldList, newList []string) []string {
	result := make([]string, 0)
	for _, new := range newList {
		found := false
		for _, old := range oldList {
			if old == new {
				found = true
				break
			}
		}
		if !found {
			result = append(result, new)
		}
	}
	return result
}

// ParseDNSBackend parses a string in the format "domain:port/protocol" into a DNSBackendConfig
func ParseDNSBackend(input string, groupName string, isPrimary bool) (*DNSBackendConfig, error) {
	// Parse the input
	parts := strings.Split(input, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid format, expected domain:port/protocol")
	}

	protocol := parts[1]
	hostPort := strings.Split(parts[0], ":")
	if len(hostPort) != 2 {
		return nil, fmt.Errorf("invalid host:port format")
	}

	domain := hostPort[0]
	port, err := strconv.Atoi(hostPort[1])
	if err != nil {
		return nil, fmt.Errorf("invalid port: %v", err)
	}

	return &DNSBackendConfig{
		Domain:              domain,
		Protocol:            protocol,
		Port:                port,
		Weight:              1, // Default weight
		IsPrimary:           isPrimary,
		GroupName:           groupName,
		ResolutionTimeout:   5 * time.Second,  // Default timeout
		ResolutionInterval:  30 * time.Second, // Default interval
		TTL:                 0,                // Use DNS TTL
		LastResolution:      time.Time{},      // Zero time
		LastAddresses:       []string{},
	}, nil
}

// GetDNSDiscoveryStatus returns the status of DNS discovery
func (dm *DNSDiscoveryManager) GetDNSDiscoveryStatus() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	domains := make([]string, 0, len(dm.configs))
	totalBackends := 0

	status := make(map[string]interface{})
	domainStatus := make(map[string]interface{})

	for domain, config := range dm.configs {
		domains = append(domains, domain)
		totalBackends += len(config.LastAddresses)

		domainStatus[domain] = map[string]interface{}{
			"protocol":         config.Protocol,
			"port":             config.Port,
			"addresses":        config.LastAddresses,
			"address_count":    len(config.LastAddresses),
			"last_resolution":  config.LastResolution.Format(time.RFC3339),
			"group":            config.GroupName,
			"is_primary":       config.IsPrimary,
			"resolution_interval": config.ResolutionInterval.String(),
		}
	}

	status["domains"] = domains
	status["domain_count"] = len(domains)
	status["backend_count"] = totalBackends
	status["domain_status"] = domainStatus

	return status
}