package main

import (
	"crypto/tls"
	"log"

	"leba/internal/config"
	"leba/internal/loadbalancer"
)

func main() {
	// Load configuration from YAML file
	configPath := "config.yaml"
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set up TLS configuration if enabled
	var tlsConfig *tls.Config
	if cfg.TLSConfig.Enabled {
		cert, err := tls.LoadX509KeyPair(cfg.TLSConfig.CertFile, cfg.TLSConfig.KeyFile)
		if err != nil {
			log.Fatalf("Failed to load TLS certificates: %v", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h2"}, // Enable HTTP/2
		}
	}

	// Initialize the backend pool
	backendPool := loadbalancer.NewBackendPool()
	for _, backendCfg := range cfg.Backends {
		backend := &loadbalancer.Backend{
			Address:            backendCfg.Address,
			Protocol:           backendCfg.Protocol,
			Port:               backendCfg.Port,
			Weight:             backendCfg.Weight,
			MaxOpenConnections: backendCfg.MaxOpenConnections,
			MaxIdleConnections: backendCfg.MaxIdleConnections,
			ConnMaxLifetime:    backendCfg.ConnMaxLifetime,
			Health:             true, // Assume healthy at start
		}
		backendPool.AddBackend(backend)
	}

	// Convert InitialPeers to a slice of strings
	var peerAddresses []string
	for _, peer := range cfg.InitialPeers {
		peerAddresses = append(peerAddresses, peer.Address)
	}

	// Initialize the peer manager
	peerManager := loadbalancer.NewPeerManager(peerAddresses, backendPool)

	// Create the LoadBalancer instance
	lb := loadbalancer.NewLoadBalancer(
		cfg.FrontendAddress,
		backendPool,
		cfg.AllowedPorts, // Dynamically load allowed ports
		tlsConfig,
		cfg.CacheConfig,
		cfg.TLSConfig.CertFile,
		cfg.TLSConfig.KeyFile,
		peerManager,
	)

	// Start cluster synchronization
	go lb.StartClusterSync()

	// Start frontend services dynamically
	for _, service := range cfg.FrontendServices {
		go func(service config.FrontendService) {
			if err := lb.StartService(service.Protocol, service.Port); err != nil {
				log.Printf("Failed to start service for protocol %s on port %d: %v", service.Protocol, service.Port, err)
			}
		}(service)
	}

	// Keep the application running
	select {}
}
