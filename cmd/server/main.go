package main

import (
	"log"

	"leba/internal/loadbalancer"
	"leba/internal/shared"
)

func main() {
	// Load configuration
	cfg, err := shared.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Configure TLS
	tlsConfig, err := loadbalancer.ConfigureTLS(&cfg.TLSConfig)
	if err != nil {
		log.Fatalf("Failed to configure TLS: %v", err)
	}

	// Initialize BackendPool
	backendPool := loadbalancer.NewBackendPool()

	// Add backends from the configuration
	for _, backendCfg := range cfg.Backends {
		backend := loadbalancer.NewBackend(
			backendCfg.Address,
			backendCfg.Protocol,
			backendCfg.Port,
			backendCfg.Weight,
			backendCfg.Role,
		)
		backendPool.AddBackend(backend)
	}

	// Initialize PeeringManager
	peeringManager := loadbalancer.NewPeeringManager(*cfg, nil)

	// Create LoadBalancer instance
	loadBalancer := loadbalancer.NewLoadBalancer(
		cfg.FrontendAddress,
		backendPool,
		cfg.AllowedPorts,
		tlsConfig,
		cfg.CacheConfig,
		cfg.LogFilePath,
		cfg.SIPLogFilePath,
		peeringManager,
	)

	// Set LoadBalancer instance in PeeringManager
	peeringManager.SetLoadBalancer(loadBalancer)

	// Start PeeringManager
	go peeringManager.Start()

	// Start the API
	err = loadbalancer.StartAPI(cfg.APIPort, loadBalancer.GetBackendPool())
	if err != nil {
		log.Fatalf("Failed to start API: %v", err)
	}

	// Start frontend services
	for _, service := range cfg.FrontendServices {
		go func(service shared.FrontendService) {
			err := loadBalancer.StartService(service.Protocol, service.Port)
			if err != nil {
				log.Printf("Failed to start service %s on port %d: %v", service.Protocol, service.Port, err)
			}
		}(service)
	}

	// Block main goroutine
	select {}
}
