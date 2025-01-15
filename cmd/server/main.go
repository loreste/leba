package main

import (
	"crypto/tls"
	"log"

	"leba/internal/config"
	"leba/internal/loadbalancer"
)

func main() {
	log.Println("Starting Load Balancer...")

	// Load configuration from YAML file
	configPath := "config.yaml"
	log.Printf("Loading configuration from %s...", configPath)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Println("Configuration successfully loaded.")

	// Set up TLS configuration if enabled
	var tlsConfig *tls.Config
	if cfg.TLSConfig.Enabled {
		log.Println("TLS is enabled. Setting up certificates...")
		cert, err := tls.LoadX509KeyPair(cfg.TLSConfig.CertFile, cfg.TLSConfig.KeyFile)
		if err != nil {
			log.Fatalf("Failed to load TLS certificates: %v", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h2"}, // Enable HTTP/2
		}
		log.Println("TLS configuration successfully set up.")
	} else {
		log.Println("TLS is disabled. Proceeding without encryption.")
	}

	// Initialize the backend pool
	log.Println("Initializing backend pool...")
	backendPool := loadbalancer.NewBackendPool()
	for _, backendCfg := range cfg.Backends {
		log.Printf("Adding backend: Address=%s, Protocol=%s, Port=%d, Role=%s",
			backendCfg.Address, backendCfg.Protocol, backendCfg.Port, backendCfg.Role)
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
	log.Println("Backend pool initialized successfully.")

	// Convert InitialPeers to a slice of strings
	log.Println("Setting up peer manager...")
	var peerAddresses []string
	for _, peer := range cfg.InitialPeers {
		log.Printf("Adding peer: Address=%s, NodeID=%s", peer.Address, peer.NodeID)
		peerAddresses = append(peerAddresses, peer.Address)
	}

	// Initialize the peer manager
	peerManager := loadbalancer.NewPeerManager(peerAddresses, backendPool)

	// Create the LoadBalancer instance
	log.Println("Creating Load Balancer instance...")
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
	log.Println("Load Balancer instance created successfully.")

	// Start cluster synchronization
	log.Println("Starting cluster synchronization...")
	go lb.StartClusterSync()

	// Start frontend services dynamically
	log.Println("Starting frontend services...")
	for _, service := range cfg.FrontendServices {
		go func(service config.FrontendService) {
			log.Printf("Starting service: Protocol=%s, Port=%d", service.Protocol, service.Port)
			if err := lb.StartService(service.Protocol, service.Port); err != nil {
				log.Printf("Failed to start service for protocol %s on port %d: %v", service.Protocol, service.Port, err)
			} else {
				log.Printf("Service started successfully: Protocol=%s, Port=%d", service.Protocol, service.Port)
			}
		}(service)
	}

	log.Println("Load Balancer is up and running.")

	// Keep the application running
	select {}
}
