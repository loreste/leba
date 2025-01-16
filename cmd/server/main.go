package main

import (
	"log"
	"time"

	"leba/internal/config"
	"leba/internal/loadbalancer"
	"leba/internal/shared"
)

func main() {
	// Load configuration
	configPath := "config.yml"
	cfg, err := shared.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize Backend Pool
	backendPool := loadbalancer.NewBackendPool()
	for _, backendConfig := range cfg.Backends {
		backend := loadbalancer.NewBackend(
			backendConfig.Address,
			backendConfig.Protocol,
			backendConfig.Port,
			backendConfig.Weight,
			backendConfig.Role,
		)
		backendPool.AddBackend(backend)
	}

	// Convert CacheConfig
	cacheConfig := config.CacheConfig{
		Enabled:         cfg.CacheConfig.Enabled,
		Expiration:      time.Duration(cfg.CacheConfig.Expiration) * time.Second,
		CleanupInterval: time.Duration(cfg.CacheConfig.CleanupInterval) * time.Second,
	}

	// Convert TLSConfig
	tlsConfig := &loadbalancer.TLSConfig{
		Enabled:  cfg.TLSConfig.Enabled,
		CertFile: cfg.TLSConfig.CertFile,
		KeyFile:  cfg.TLSConfig.KeyFile,
	}

	// Configure TLS
	tlsConfigured, err := loadbalancer.ConfigureTLS(tlsConfig)
	if err != nil {
		log.Fatalf("Failed to configure TLS: %v", err)
	}

	// Convert InitialPeers
	var initialPeers []loadbalancer.PeerConfig
	for _, peer := range cfg.InitialPeers {
		initialPeers = append(initialPeers, loadbalancer.PeerConfig{
			Address: peer.Address,
			NodeID:  peer.NodeID,
		})
	}

	// Initialize Peering Manager
	peerManager := loadbalancer.NewPeerManager(initialPeers, backendPool, cfg.PeerSyncInterval)

	// Create Load Balancer
	loadBalancer := loadbalancer.NewLoadBalancer(
		cfg.FrontendAddress,
		backendPool,
		cfg.AllowedPorts,
		tlsConfigured,
		cacheConfig,
		cfg.SIPLogFilePath,
		cfg.LogFilePath,
		// cfg.AnotherConfigPath, // Removed as it does not exist in shared.Config need to work on that a bit more.
		peerManager,
	)

	// Start the Peering Manager
	go peerManager.Start() //TODO: Implement this method at some point

	// Start API Server
	go func() {
		if err := loadbalancer.StartAPI(cfg.APIPort, backendPool); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// Start Frontend Services
	for _, service := range cfg.FrontendServices {
		go func(service shared.FrontendService) {
			if err := loadBalancer.StartService(service.Protocol, service.Port); err != nil {
				log.Printf("Failed to start %s service on port %d: %v", service.Protocol, service.Port, err)
			}
		}(service)
	}

	// Block forever
	select {}
}
