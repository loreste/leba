package loadbalancer

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

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
	mu              sync.RWMutex
}

// NewLoadBalancer creates a new LoadBalancer instance
func NewLoadBalancer(frontendAddress string, backendPool *BackendPool, allowedPorts map[string][]int, tlsConfig *tls.Config, cacheConfig config.CacheConfig, certFile, keyFile string, peerManager *PeerManager) *LoadBalancer {
	log.Printf("Creating LoadBalancer for address: %s", frontendAddress)
	return &LoadBalancer{
		frontendAddress: frontendAddress,
		backendPool:     backendPool,
		allowedPorts:    allowedPorts,
		tlsConfig:       tlsConfig,
		cacheConfig:     cacheConfig,
		certFile:        certFile,
		keyFile:         keyFile,
		peerManager:     peerManager,
	}
}

// StartClusterSync periodically broadcasts the backend state to peers
func (lb *LoadBalancer) StartClusterSync() {
	log.Println("Starting cluster synchronization...")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("Broadcasting backend state to peers...")
		lb.peerManager.BroadcastState()
	}
}

// StartService starts a specific protocol service
func (lb *LoadBalancer) StartService(protocol string, port int) error {
	address := fmt.Sprintf(":%d", port)
	log.Printf("Attempting to start %s service on %s", protocol, address)

	switch protocol {
	case "http":
		log.Println("Starting HTTP service...")
		mux := http.NewServeMux()
		mux.HandleFunc("/", lb.handleHTTP)
		return http.ListenAndServe(address, mux)

	case "https":
		log.Println("Starting HTTPS service...")
		mux := http.NewServeMux()
		mux.HandleFunc("/", lb.handleHTTPS)
		return http.ListenAndServeTLS(address, lb.certFile, lb.keyFile, mux)

	case "postgres":
		log.Println("Starting PostgreSQL service...")
		return lb.startPostgres(address)

	case "mysql":
		log.Println("Starting MySQL service...")
		return lb.startMySQL(address)

	default:
		log.Printf("Unsupported protocol: %s", protocol)
		return fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// handleHTTP handles HTTP requests
func (lb *LoadBalancer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling HTTP request: %s", r.URL.Path)
	lb.routeAndForward(w, r, "http")
}

// handleHTTPS handles HTTPS requests
func (lb *LoadBalancer) handleHTTPS(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling HTTPS request: %s", r.URL.Path)
	lb.routeAndForward(w, r, "https")
}

// startPostgres handles PostgreSQL connections
func (lb *LoadBalancer) startPostgres(address string) error {
	log.Printf("Starting PostgreSQL listener on %s", address)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start PostgreSQL listener: %v", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("PostgreSQL connection error: %v", err)
			continue
		}
		log.Printf("Accepted PostgreSQL connection from %s", conn.RemoteAddr())
		go lb.handleDatabaseConnection(conn, "postgres")
	}
}

// startMySQL handles MySQL connections
func (lb *LoadBalancer) startMySQL(address string) error {
	log.Printf("Starting MySQL listener on %s", address)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start MySQL listener: %v", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("MySQL connection error: %v", err)
			continue
		}
		log.Printf("Accepted MySQL connection from %s", conn.RemoteAddr())
		go lb.handleDatabaseConnection(conn, "mysql")
	}
}

// handleDatabaseConnection handles database connections for PostgreSQL and MySQL
func (lb *LoadBalancer) handleDatabaseConnection(conn net.Conn, protocol string) {
	defer conn.Close()

	backend, err := lb.routeRequest(protocol)
	if err != nil {
		log.Printf("Failed to route %s request: %v", protocol, err)
		return
	}

	backendConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", backend.Address, backend.Port))
	if err != nil {
		log.Printf("Failed to connect to %s backend: %v", protocol, err)
		return
	}
	defer backendConn.Close()

	log.Printf("Forwarding %s connection to backend: %s:%d", protocol, backend.Address, backend.Port)
	go io.Copy(backendConn, conn)
	io.Copy(conn, backendConn)
}

// routeAndForward handles routing and forwarding HTTP/HTTPS requests
func (lb *LoadBalancer) routeAndForward(w http.ResponseWriter, r *http.Request, protocol string) {
	backend, err := lb.routeRequest(protocol)
	if err != nil {
		log.Printf("Routing error for protocol %s: %v", protocol, err)
		http.Error(w, fmt.Sprintf("Failed to route request: %v", err), http.StatusInternalServerError)
		return
	}
	url := fmt.Sprintf("%s://%s%s", protocol, backend.Address, r.URL.Path)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// routeRequest routes the query or connection based on protocol and validates ports
func (lb *LoadBalancer) routeRequest(protocol string) (*Backend, error) {
	log.Printf("Routing request for protocol: %s", protocol)
	lb.backendPool.mu.RLock()
	defer lb.backendPool.mu.RUnlock()

	var selectedBackend *Backend
	for _, backend := range lb.backendPool.backends {
		if backend.Protocol == protocol && backend.Health {
			if !lb.isPortAllowed(protocol, backend.Port) {
				log.Printf("Port %d not allowed for protocol %s", backend.Port, protocol)
				continue
			}
			if selectedBackend == nil || backend.ActiveConnections < selectedBackend.ActiveConnections {
				selectedBackend = backend
			}
		}
	}

	if selectedBackend == nil {
		log.Printf("No available backend for protocol: %s", protocol)
		return nil, fmt.Errorf("no available backend for protocol: %s", protocol)
	}

	log.Printf("Routing to backend: %s:%d", selectedBackend.Address, selectedBackend.Port)
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
