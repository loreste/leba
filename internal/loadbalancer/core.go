package loadbalancer

import (
	"crypto/tls"
	"log"
	"sync"

	"leba/internal/config"
)

// LoadBalancer represents the main load balancer logic
type LoadBalancer struct {
	frontendAddress string
	backendPool     *BackendPool
	tlsConfig       *tls.Config
	cacheConfig     config.CacheConfig
	certFile        string
	keyFile         string
	peerManager     *PeerManager
	allowedPorts    map[string][]int
	logFilePath     string
	mu              sync.RWMutex
}

// NewLoadBalancer creates a new LoadBalancer instance
func NewLoadBalancer(
	frontendAddress string,
	backendPool *BackendPool,
	allowedPorts map[string][]int,
	tlsConfig *tls.Config,
	cacheConfig config.CacheConfig,
	certFile, keyFile, logFilePath string,
	peerManager *PeerManager,
) *LoadBalancer {
	log.Printf("Creating LoadBalancer for address: %s", frontendAddress)
	return &LoadBalancer{
		frontendAddress: frontendAddress,
		backendPool:     backendPool,
		allowedPorts:    allowedPorts,
		tlsConfig:       tlsConfig,
		cacheConfig:     cacheConfig,
		certFile:        certFile,
		keyFile:         keyFile,
		logFilePath:     logFilePath,
		peerManager:     peerManager,
	}
}
