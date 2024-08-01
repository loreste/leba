// main.go
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

	// Initialize the cache configuration
	cacheConfig := cfg.CacheConfig

	// Create the LoadBalancer instance
	lb := loadbalancer.NewLoadBalancer(cfg.FrontendAddress, backendPool, tlsConfig, cacheConfig, cfg.TLSConfig.CertFile, cfg.TLSConfig.KeyFile)

	// Create the PeeringManager instance and start it
	peeringManager := loadbalancer.NewPeeringManager(cfg, lb)
	peeringManager.Start()

	// Start the load balancer
	if err := lb.Start(); err != nil {
		log.Fatalf("Failed to start load balancer: %v", err)
	}

	// Keep the application running
	select {}
}
