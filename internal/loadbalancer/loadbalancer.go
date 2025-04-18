package loadbalancer

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"crypto/tls"
	"leba/internal/config"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	_ "github.com/lib/pq"              // PostgreSQL driver
)

// RetryConfig contains retry configuration
type RetryConfig struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
}

// Default retry configuration
var DefaultRetryConfig = RetryConfig{
	MaxRetries:  3,
	InitialWait: 100 * time.Millisecond,
	MaxWait:     2 * time.Second,
}

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
	healthChecker   *HealthChecker
	retryConfig     RetryConfig
	connectionPool  *DBConnectionPool
	dnsManager      *DNSDiscoveryManager
	failoverManager *FailoverManager
	servers         map[string]*http.Server
	listeners       map[string]net.Listener
	mu              sync.RWMutex
	shutdown        chan struct{}
}

// NewLoadBalancer creates a new LoadBalancer instance
func NewLoadBalancer(frontendAddress string, backendPool *BackendPool, allowedPorts map[string][]int, tlsConfig *tls.Config, cacheConfig config.CacheConfig, certFile, keyFile string, peerManager *PeerManager) *LoadBalancer {
	log.Printf("Creating LoadBalancer for address: %s", frontendAddress)
	
	// Create health checker with 10-second interval
	healthChecker := NewHealthChecker(backendPool, 10*time.Second)
	
	// Create connection pool manager
	connectionPool := NewDBConnectionPool()
	
	return &LoadBalancer{
		frontendAddress: frontendAddress,
		backendPool:     backendPool,
		allowedPorts:    allowedPorts,
		tlsConfig:       tlsConfig,
		cacheConfig:     cacheConfig,
		certFile:        certFile,
		keyFile:         keyFile,
		peerManager:     peerManager,
		healthChecker:   healthChecker,
		retryConfig:     DefaultRetryConfig,
		connectionPool:  connectionPool,
		servers:         make(map[string]*http.Server),
		listeners:       make(map[string]net.Listener),
		shutdown:        make(chan struct{}),
	}
}

// SetDNSManager sets the DNS discovery manager
func (lb *LoadBalancer) SetDNSManager(dnsManager *DNSDiscoveryManager) {
	lb.dnsManager = dnsManager
}

// SetFailoverManager sets the failover manager
func (lb *LoadBalancer) SetFailoverManager(failoverManager *FailoverManager) {
	lb.failoverManager = failoverManager
}

// StartServices starts all configured services and sets up graceful shutdown
func (lb *LoadBalancer) StartServices(services []config.FrontendService) {
	// Start health checker
	lb.healthChecker.Start()
	
	// Start cluster synchronization
	go lb.StartClusterSync()
	
	// Create a WaitGroup to track all running services
	var wg sync.WaitGroup
	
	// Start all services
	for _, service := range services {
		wg.Add(1)
		go func(svc config.FrontendService) {
			defer wg.Done()
			if err := lb.StartService(svc.Protocol, svc.Port); err != nil {
				log.Printf("Failed to start service %s on port %d: %v", svc.Protocol, svc.Port, err)
			}
		}(service)
	}
	
	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		sig := <-signalChan
		log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)
		
		// Notify health checker to stop
		lb.healthChecker.Stop()
		
		// Stop DNS discovery manager if it exists
		if lb.dnsManager != nil {
			log.Println("Shutting down DNS discovery manager...")
			lb.dnsManager.Stop()
		}
		
		// Stop failover manager if it exists
		if lb.failoverManager != nil {
			log.Println("Shutting down failover manager...")
			lb.failoverManager.Stop()
		}
		
		// Stop all servers gracefully
		lb.mu.Lock()
		for id, server := range lb.servers {
			log.Printf("Shutting down server: %s", id)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := server.Shutdown(ctx); err != nil {
				log.Printf("Error shutting down server %s: %v", id, err)
			}
			cancel()
		}
		lb.mu.Unlock()
		
		close(lb.shutdown)
	}()
	
	// Wait for all services to complete
	wg.Wait()
	log.Println("All services have been stopped. Load balancer shutdown complete.")
}

// StartClusterSync periodically broadcasts the backend state to peers
func (lb *LoadBalancer) StartClusterSync() {
	log.Println("Starting cluster synchronization...")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Broadcasting backend state to peers...")
			lb.peerManager.BroadcastState()
		case <-lb.shutdown:
			log.Println("Stopping cluster synchronization...")
			return
		}
	}
}

// StartService starts a specific protocol service
func (lb *LoadBalancer) StartService(protocol string, port int) error {
	address := fmt.Sprintf(":%d", port)
	serviceID := fmt.Sprintf("%s-%d", protocol, port)
	log.Printf("Starting %s service on %s", protocol, address)

	switch protocol {
	case "http":
		return lb.startHTTPService(serviceID, address, false)

	case "https":
		return lb.startHTTPService(serviceID, address, true)

	case "postgres":
		return lb.startTCPService(serviceID, address, "postgres")

	case "mysql":
		return lb.startTCPService(serviceID, address, "mysql")

	default:
		log.Printf("Unsupported protocol: %s", protocol)
		return fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// startHTTPService starts an HTTP or HTTPS service
func (lb *LoadBalancer) startHTTPService(serviceID, address string, isHTTPS bool) error {
	// Create a new ServeMux for this service
	mux := http.NewServeMux()
	
	// Register handlers
	mux.HandleFunc("/", lb.handleHTTP)
	mux.HandleFunc("/health", lb.handleHealthCheck)
	
	// Create server with reasonable timeouts
	server := &http.Server{
		Addr:    address,
		Handler: mux,
		// Good baseline values for production
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	
	// Store server reference for graceful shutdown
	lb.mu.Lock()
	lb.servers[serviceID] = server
	lb.mu.Unlock()
	
	// Start the server
	if isHTTPS {
		log.Printf("Starting HTTPS server on %s", address)
		if lb.tlsConfig != nil {
			server.TLSConfig = lb.tlsConfig
			return server.ListenAndServeTLS(lb.certFile, lb.keyFile)
		}
		return server.ListenAndServeTLS(lb.certFile, lb.keyFile)
	}
	
	log.Printf("Starting HTTP server on %s", address)
	return server.ListenAndServe()
}

// startTCPService starts a TCP-based service (PostgreSQL, MySQL)
func (lb *LoadBalancer) startTCPService(serviceID, address, protocol string) error {
	log.Printf("Starting %s listener on %s", protocol, address)
	
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start %s listener: %v", protocol, err)
	}
	
	// Store listener reference
	lb.mu.Lock()
	lb.listeners[serviceID] = listener
	lb.mu.Unlock()
	
	// Set up a context that's canceled on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-lb.shutdown
		cancel()
		listener.Close()
	}()
	
	// Accept connections in a loop
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					// Normal shutdown
					return
				default:
					log.Printf("%s connection accept error: %v", protocol, err)
					continue
				}
			}
			
			log.Printf("Accepted %s connection from %s", protocol, conn.RemoteAddr())
			go lb.handleDatabaseConnection(conn, protocol)
		}
	}()
	
	return nil
}

// handleHealthCheck handles load balancer health check requests
func (lb *LoadBalancer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check if we have at least one healthy backend
	lb.backendPool.mu.RLock()
	healthy := false
	for _, backend := range lb.backendPool.backends {
		if backend.IsAvailable() {
			healthy = true
			break
		}
	}
	lb.backendPool.mu.RUnlock()
	
	if !healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("No healthy backends available"))
		return
	}
	
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleHTTP handles HTTP requests
func (lb *LoadBalancer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	protocol := "http"
	if r.TLS != nil {
		protocol = "https"
	}
	
	log.Printf("Handling %s request: %s %s", protocol, r.Method, r.URL.Path)
	
	// Try to route with retry logic
	backend, err := lb.routeRequestWithRetry(protocol)
	if err != nil {
		log.Printf("%s routing error: %v", protocol, err)
		http.Error(w, fmt.Sprintf("Failed to route request: %v", err), http.StatusServiceUnavailable)
		return
	}
	
	// Clone the request
	outReq := r.Clone(r.Context())
	
	// Update the host if needed
	if r.Host != "" {
		outReq.Host = r.Host
	}
	
	// Create the target URL
	targetURL := fmt.Sprintf("%s://%s:%d%s", protocol, backend.Address, backend.Port, r.URL.Path)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}
	
	// Proxy the request (redirect is safer for now, but could be enhanced to proxy)
	http.Redirect(w, r, targetURL, http.StatusTemporaryRedirect)
	
	// Mark the transaction as successful
	backend.MarkSuccess()
	
	// Record connection completion
	lb.backendPool.mu.Lock()
	defer lb.backendPool.mu.Unlock()
	
	if b, ok := lb.backendPool.backends[backend.Address]; ok {
		b.mu.Lock()
		b.ActiveConnections--
		if b.ActiveConnections < 0 {
			b.ActiveConnections = 0
		}
		b.mu.Unlock()
	}
}

// handleDatabaseConnection handles database connections for PostgreSQL and MySQL
func (lb *LoadBalancer) handleDatabaseConnection(conn net.Conn, protocol string) {
	defer conn.Close()

	// Try to route with retry logic
	backend, err := lb.routeRequestWithRetry(protocol)
	if err != nil {
		log.Printf("Failed to route %s request: %v", protocol, err)
		return
	}

	// Check if we should use connection pooling
	if backend.Protocol == "postgres" || backend.Protocol == "mysql" {
		// Try to handle with connection pool
		err = lb.handlePooledDatabaseConnection(conn, backend)
		if err == nil {
			// Successfully handled with connection pool
			return
		}
		// If error, fall back to direct connection
		log.Printf("Connection pool error, falling back to direct connection: %v", err)
	}

	// Set reasonable timeouts for backend connection
	backendConn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", backend.Address, backend.Port), 5*time.Second)
	if err != nil {
		log.Printf("Failed to connect to %s backend: %v", protocol, err)
		backend.MarkFailure()
		return
	}
	defer backendConn.Close()

	log.Printf("Forwarding %s connection to backend: %s:%d", protocol, backend.Address, backend.Port)
	
	// Set TCP keepalive to detect dead connections
	if tcpConn, ok := backendConn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}
	
	// Create channels to signal connection completion
	done := make(chan struct{}, 2)
	
	// Copy data in both directions
	go func() {
		io.Copy(backendConn, conn)
		done <- struct{}{}
	}()
	
	go func() {
		io.Copy(conn, backendConn)
		done <- struct{}{}
	}()
	
	// Wait for either direction to complete
	<-done
	
	// Mark transaction as successful
	backend.MarkSuccess()
	
	// Update connection count
	backend.mu.Lock()
	backend.ActiveConnections--
	if backend.ActiveConnections < 0 {
		backend.ActiveConnections = 0
	}
	backend.mu.Unlock()
}

// handlePooledDatabaseConnection handles a database connection using the connection pool
func (lb *LoadBalancer) handlePooledDatabaseConnection(clientConn net.Conn, backend *Backend) error {
	// Add backend to connection pool if not already there
	if err := lb.connectionPool.AddBackend(backend); err != nil {
		return fmt.Errorf("failed to add to connection pool: %v", err)
	}

	// Get a connection from the pool
	dbConn, err := lb.connectionPool.GetBackendConn(backend)
	if err != nil {
		backend.MarkFailure()
		return fmt.Errorf("failed to get connection from pool: %v", err)
	}
	defer dbConn.Close()

	// For database connections, we connect directly but use the pool for health checks
	backendConn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", backend.Address, backend.Port), 5*time.Second)
	if err != nil {
		backend.MarkFailure()
		return fmt.Errorf("failed to connect to backend: %v", err)
	}
	defer backendConn.Close()
	
	// Set TCP keepalive to detect dead connections
	if tcpConn, ok := backendConn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	log.Printf("Using pooled %s connection to backend: %s:%d", 
		backend.Protocol, backend.Address, backend.Port)
	
	// Create channels to signal connection completion
	done := make(chan struct{}, 2)
	
	// Copy data in both directions
	go func() {
		io.Copy(backendConn, clientConn)
		done <- struct{}{}
	}()
	
	go func() {
		io.Copy(clientConn, backendConn)
		done <- struct{}{}
	}()
	
	// Wait for either direction to complete
	<-done
	
	// Mark transaction as successful
	backend.MarkSuccess()
	
	return nil
}

// routeRequestWithRetry tries to route a request with retry logic
func (lb *LoadBalancer) routeRequestWithRetry(protocol string) (*Backend, error) {
	var lastErr error
	
	// Implement exponential backoff
	wait := lb.retryConfig.InitialWait
	
	for i := 0; i < lb.retryConfig.MaxRetries; i++ {
		backend, err := lb.routeRequest(protocol)
		if err == nil {
			return backend, nil
		}
		
		lastErr = err
		
		// Wait before retry
		time.Sleep(wait)
		
		// Exponential backoff with jitter
		wait = time.Duration(float64(wait) * 1.5)
		if wait > lb.retryConfig.MaxWait {
			wait = lb.retryConfig.MaxWait
		}
	}
	
	return nil, fmt.Errorf("failed after %d retries: %v", lb.retryConfig.MaxRetries, lastErr)
}

// routeRequest routes the query or connection based on protocol and validates ports
func (lb *LoadBalancer) routeRequest(protocol string) (*Backend, error) {
	log.Printf("Routing request for protocol: %s", protocol)
	
	// Get available backends for the protocol
	availableBackends := lb.backendPool.AvailableBackends(protocol)
	
	if len(availableBackends) == 0 {
		return nil, fmt.Errorf("no available backend for protocol: %s", protocol)
	}
	
	// Use least connections algorithm
	var selectedBackend *Backend
	minConnections := -1
	
	for _, backend := range availableBackends {
		if !lb.isPortAllowed(protocol, backend.Port) {
			log.Printf("Port %d not allowed for protocol %s", backend.Port, protocol)
			continue
		}
		
		if minConnections == -1 || backend.ActiveConnections < minConnections {
			selectedBackend = backend
			minConnections = backend.ActiveConnections
		}
	}
	
	if selectedBackend == nil {
		return nil, fmt.Errorf("no available backend with allowed port for protocol: %s", protocol)
	}
	
	log.Printf("Routing to backend: %s:%d (current connections: %d)", 
		selectedBackend.Address, selectedBackend.Port, selectedBackend.ActiveConnections)
	
	selectedBackend.mu.Lock()
	selectedBackend.ActiveConnections++
	selectedBackend.mu.Unlock()
	
	return selectedBackend, nil
}

// isPortAllowed checks if the port is allowed for the given protocol
func (lb *LoadBalancer) isPortAllowed(protocol string, port int) bool {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if ports, exists := lb.allowedPorts[protocol]; exists {
		for _, allowedPort := range ports {
			if port == allowedPort {
				return true
			}
		}
	}
	return false
}