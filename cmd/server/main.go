package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"leba/internal/config"
	"leba/internal/loadbalancer"
	"leba/internal/version"
)

// Global variables for cleanup
var (
	dnsManager *loadbalancer.DNSDiscoveryManager
	failoverManager *loadbalancer.FailoverManager
)

func main() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Starting LEBA v%s...", version.Version)
	
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to the configuration file")
	logFile := flag.String("log", "", "Path to log file (if empty, logs to stdout)")
	watchConfig := flag.Bool("watch-config", true, "Watch config file for changes")
	watchInterval := flag.Int("watch-interval", 30, "Config watch interval in seconds")
	showVersion := flag.Bool("version", false, "Show version information and exit")
	flag.Parse()
	
	// Show version information if requested
	if *showVersion {
		fmt.Printf("LEBA - Load-Efficient Balancing Architecture v%s\n", version.Version)
		fmt.Printf("Git commit: %s\n", version.GitCommit)
		fmt.Printf("Built: %s\n", version.BuildDate)
		fmt.Printf("Go version: %s\n", runtime.Version())
		fmt.Printf("OS/Arch: %s\n", runtime.GOOS+"/"+runtime.GOARCH)
		os.Exit(0)
	}
	
	// Set up logging
	setupLogging(*logFile)
	
	// Set up cleanup handler
	setupCleanup()
	
	// Load configuration from YAML file
	log.Printf("Loading configuration from %s...", *configPath)
	configManager := config.GetConfigManager(*configPath)
	cfg, err := configManager.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Println("Configuration successfully loaded.")
	
	// Start config watcher if enabled
	if *watchConfig {
		log.Printf("Starting config watcher with interval of %d seconds", *watchInterval)
		configManager.StartConfigWatcher(time.Duration(*watchInterval) * time.Second)
		configManager.AddConfigChangeListener(handleConfigChange)
	}

	// Set up TLS configuration if enabled
	var tlsConfig *tls.Config
	if cfg.TLSConfig.Enabled {
		log.Println("TLS is enabled. Setting up certificates...")
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			PreferServerCipherSuites: true,
		}
		
		cert, err := tls.LoadX509KeyPair(cfg.TLSConfig.CertFile, cfg.TLSConfig.KeyFile)
		if err != nil {
			log.Fatalf("Failed to load TLS certificates: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		tlsConfig.NextProtos = []string{"h2"} // Enable HTTP/2
		
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
			Role:               backendCfg.Role,
			Health:             true, // Assume healthy at start
			CircuitTimeout:     10 * time.Second, // Default circuit timeout
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
	
	// Initialize the failover manager
	log.Println("Setting up failover manager...")
	failoverManager = loadbalancer.NewFailoverManager(backendPool)
	
	// Initialize the DNS discovery manager
	log.Println("Setting up DNS discovery manager...")
	dnsManager = loadbalancer.NewDNSDiscoveryManager(backendPool, failoverManager)
	
	// Configure DNS-based backends from structured config
	hasDNSBackends := false
	if len(cfg.DNSBackends) > 0 {
		hasDNSBackends = true
		for _, dnsCfg := range cfg.DNSBackends {
			log.Printf("Adding DNS backend: Domain=%s, Protocol=%s, Port=%d", 
				dnsCfg.Domain, dnsCfg.Protocol, dnsCfg.Port)
			
			dnsConfig := &loadbalancer.DNSBackendConfig{
				Domain:              dnsCfg.Domain,
				Protocol:            dnsCfg.Protocol,
				Port:                dnsCfg.Port,
				Weight:              dnsCfg.Weight,
				IsPrimary:           dnsCfg.IsPrimary,
				GroupName:           dnsCfg.GroupName,
				ResolutionTimeout:   dnsCfg.ResolutionTimeout,
				ResolutionInterval:  dnsCfg.ResolutionInterval,
				TTL:                 dnsCfg.TTL,
			}
			
			dnsManager.AddDNSBackend(dnsConfig)
		}
	}
	
	// Configure DNS-based backends from string format
	if len(cfg.DNSBackendStrings) > 0 {
		hasDNSBackends = true
		for _, dnsStr := range cfg.DNSBackendStrings {
			// Parse the string in format "domain:port/protocol"
			dnsConfig, err := loadbalancer.ParseDNSBackend(dnsStr, "", false)
			if err != nil {
				log.Printf("Warning: Failed to parse DNS backend string '%s': %v", dnsStr, err)
				continue
			}
			
			log.Printf("Adding DNS backend from string: %s (%s:%d)", 
				dnsStr, dnsConfig.Domain, dnsConfig.Port)
			dnsManager.AddDNSBackend(dnsConfig)
		}
	}
	
	// Configure failover groups
	if len(cfg.FailoverGroups) > 0 {
		for _, group := range cfg.FailoverGroups {
			log.Printf("Setting up failover group: %s", group.Name)
			
			// Determine failover mode
			mode := loadbalancer.NormalMode
			if strings.ToLower(group.Mode) == "active-passive" {
				mode = loadbalancer.ActivePassiveMode
			}
			
			// Create group
			failoverManager.CreateBackendGroup(group.Name, mode)
			
			// Add backends to group
			for _, backend := range group.Backends {
				isPrimary := backend == group.Primary
				log.Printf("Adding backend %s to group %s (primary: %v)", 
					backend, group.Name, isPrimary)
				failoverManager.AddBackendToGroup(group.Name, backend, isPrimary)
			}
		}
	}
	
	// Start DNS discovery if any DNS backends were configured
	if hasDNSBackends {
		dnsManager.Start()
	}

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
	
	// Set DNS and failover managers on the load balancer
	lb.SetDNSManager(dnsManager)
	lb.SetFailoverManager(failoverManager)
	
	log.Println("Load Balancer instance created successfully.")

	// Start all services and setup signal handling for graceful shutdown
	log.Println("Starting all services...")
	lb.StartServices(cfg.FrontendServices)
}

// setupLogging configures the logging system
func setupLogging(logFilePath string) {
	if logFilePath == "" {
		return // Use stdout
	}
	
	// Create log directory if needed
	logDir := filepath.Dir(logFilePath)
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Printf("Warning: Failed to create log directory: %v", err)
		}
	}
	
	// Set up logging to file
	logFile, err := os.OpenFile(
		logFilePath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		log.Printf("Warning: Failed to open log file: %v", err)
	} else {
		log.SetOutput(logFile)
	}
}

// handleConfigChange is called when the configuration file changes
func handleConfigChange(newConfig *config.Config) {
	log.Println("Configuration changed. Some changes will take effect on restart.")
	
	// Check for DNS backend changes
	if len(newConfig.DNSBackends) > 0 || len(newConfig.DNSBackendStrings) > 0 {
		log.Println("DNS backend configuration changed, updating...")
		
		// Check if we need to (re)start the DNS discovery manager
		if dnsManager != nil {
			// Process new or updated DNS backends
			for _, dnsCfg := range newConfig.DNSBackends {
				log.Printf("Updating DNS backend: Domain=%s, Protocol=%s, Port=%d", 
					dnsCfg.Domain, dnsCfg.Protocol, dnsCfg.Port)
				
				dnsConfig := &loadbalancer.DNSBackendConfig{
					Domain:              dnsCfg.Domain,
					Protocol:            dnsCfg.Protocol,
					Port:                dnsCfg.Port,
					Weight:              dnsCfg.Weight,
					IsPrimary:           dnsCfg.IsPrimary,
					GroupName:           dnsCfg.GroupName,
					ResolutionTimeout:   dnsCfg.ResolutionTimeout,
					ResolutionInterval:  dnsCfg.ResolutionInterval,
					TTL:                 dnsCfg.TTL,
				}
				
				dnsManager.AddDNSBackend(dnsConfig)
			}
			
			// Process DNS backend strings
			for _, dnsStr := range newConfig.DNSBackendStrings {
				dnsConfig, err := loadbalancer.ParseDNSBackend(dnsStr, "", false)
				if err != nil {
					log.Printf("Warning: Failed to parse DNS backend string '%s': %v", dnsStr, err)
					continue
				}
				
				log.Printf("Updating DNS backend from string: %s", dnsStr)
				dnsManager.AddDNSBackend(dnsConfig)
			}
		}
	}
	
	// In a production environment, we'd update other load balancer settings
	// on-the-fly based on which parts of the config changed.
	// For most changes, a restart is safest to ensure all components
	// are properly updated.
}

// setupCleanup registers cleanup handlers
func setupCleanup() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cleanup()
		os.Exit(0)
	}()
}

// cleanup stops all services gracefully
func cleanup() {
	log.Println("Performing cleanup...")
	
	// Stop the DNS discovery manager
	if dnsManager != nil {
		log.Println("Stopping DNS discovery manager...")
		dnsManager.Stop()
	}
	
	// Stop the failover manager
	if failoverManager != nil {
		log.Println("Stopping failover manager...")
		failoverManager.Stop()
	}
	
	log.Println("Cleanup complete")
}