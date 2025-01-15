package main

import (
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"

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
			Health:             true,            // Assume healthy at start
			Role:               backendCfg.Role, // Set primary or replica
		}
		backendPool.AddBackend(backend)
	}

	// Convert InitialPeers to []string
	peers := make([]string, len(cfg.InitialPeers))
	for i, peer := range cfg.InitialPeers {
		peers[i] = peer.Address
	}

	// Initialize PeerManager with peers
	peerManager := loadbalancer.NewPeerManager(peers, backendPool)

	// Create the LoadBalancer instance
	lb := loadbalancer.NewLoadBalancer(
		cfg.FrontendAddress,
		backendPool,
		tlsConfig,
		cfg.CacheConfig,
		cfg.TLSConfig.CertFile,
		cfg.TLSConfig.KeyFile,
		peerManager,
	)

	// Start peer state synchronization
	go lb.StartClusterSync()

	// Start each frontend service dynamically
	for _, service := range cfg.FrontendServices {
		go func(protocol string, port int) {
			if err := lb.StartService(protocol, port); err != nil {
				log.Fatalf("Failed to start %s service: %v", protocol, err)
			}
		}(service.Protocol, service.Port)
	}

	// HTTP endpoint to receive peer updates
	http.HandleFunc("/update_state", func(w http.ResponseWriter, r *http.Request) {
		var newState []loadbalancer.Backend
		err := json.NewDecoder(r.Body).Decode(&newState)
		if err != nil {
			http.Error(w, "Invalid state update", http.StatusBadRequest)
			return
		}
		peerManager.UpdateState(newState)
		w.WriteHeader(http.StatusOK)
	})

	// Keep the application running
	select {}
}
