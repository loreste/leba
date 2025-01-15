package loadbalancer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"crypto/tls"
	"leba/internal/config"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	_ "github.com/lib/pq"              // PostgreSQL driver
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
	mu              sync.RWMutex
}

// NewLoadBalancer creates a new LoadBalancer instance
func NewLoadBalancer(frontendAddress string, backendPool *BackendPool, tlsConfig *tls.Config, cacheConfig config.CacheConfig, certFile, keyFile string, peerManager *PeerManager) *LoadBalancer {
	return &LoadBalancer{
		frontendAddress: frontendAddress,
		backendPool:     backendPool,
		tlsConfig:       tlsConfig,
		cacheConfig:     cacheConfig,
		certFile:        certFile,
		keyFile:         keyFile,
		peerManager:     peerManager,
	}
}

// StartClusterSync periodically broadcasts the backend state to peers
func (lb *LoadBalancer) StartClusterSync() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		lb.peerManager.BroadcastState()
	}
}

// StartService starts a specific protocol service
func (lb *LoadBalancer) StartService(protocol string, port int) error {
	address := fmt.Sprintf(":%d", port)
	log.Printf("Starting %s load balancer on %s", protocol, address)

	switch protocol {
	case "http":
		http.HandleFunc("/", lb.handleHTTP)
		return http.ListenAndServe(address, nil)

	case "https":
		http.HandleFunc("/", lb.handleHTTPS)
		return http.ListenAndServeTLS(address, lb.certFile, lb.keyFile, nil)

	case "postgres":
		return lb.startPostgres(address)

	case "mysql":
		return lb.startMySQL(address)

	default:
		return fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// handleHTTP handles HTTP requests
func (lb *LoadBalancer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	backend, err := lb.routeRequest("http")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to route HTTP request: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("http://%s%s", backend.Address, r.URL.Path), http.StatusTemporaryRedirect)
}

// handleHTTPS handles HTTPS requests
func (lb *LoadBalancer) handleHTTPS(w http.ResponseWriter, r *http.Request) {
	backend, err := lb.routeRequest("https")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to route HTTPS request: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("https://%s%s", backend.Address, r.URL.Path), http.StatusTemporaryRedirect)
}

// startPostgres handles PostgreSQL connections
func (lb *LoadBalancer) startPostgres(address string) error {
	log.Printf("Starting PostgreSQL load balancer on %s", address)
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

		go lb.handleDatabaseConnection(conn, "postgres")
	}
}

// startMySQL handles MySQL connections
func (lb *LoadBalancer) startMySQL(address string) error {
	log.Printf("Starting MySQL load balancer on %s", address)
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

	go io.Copy(backendConn, conn)
	io.Copy(conn, backendConn)
}

// routeRequest routes the query or connection based on protocol
func (lb *LoadBalancer) routeRequest(protocol string) (*Backend, error) {
	lb.backendPool.mu.RLock()
	defer lb.backendPool.mu.RUnlock()

	var selectedBackend *Backend
	for _, backend := range lb.backendPool.backends {
		if backend.Protocol == protocol && backend.Health {
			if selectedBackend == nil || backend.ActiveConnections < selectedBackend.ActiveConnections {
				selectedBackend = backend
			}
		}
	}

	if selectedBackend == nil {
		return nil, fmt.Errorf("no available backend for protocol: %s", protocol)
	}

	selectedBackend.mu.Lock()
	selectedBackend.ActiveConnections++
	selectedBackend.mu.Unlock()

	return selectedBackend, nil
}

// PeerManager manages communication between nodes in the cluster
type PeerManager struct {
	peers       []string
	stateMutex  sync.RWMutex
	backendPool *BackendPool
}

// NewPeerManager creates a new PeerManager
func NewPeerManager(peers []string, backendPool *BackendPool) *PeerManager {
	return &PeerManager{
		peers:       peers,
		backendPool: backendPool,
	}
}

// BroadcastState sends the local backend pool state to all peers
func (pm *PeerManager) BroadcastState() {
	pm.stateMutex.RLock()
	defer pm.stateMutex.RUnlock()

	for _, peer := range pm.peers {
		go func(peer string) {
			err := pm.sendStateToPeer(peer)
			if err != nil {
				log.Printf("Failed to send state to peer %s: %v", peer, err)
			}
		}(peer)
	}
}

// sendStateToPeer sends the backend pool state to a specific peer
func (pm *PeerManager) sendStateToPeer(peer string) error {
	pm.stateMutex.RLock()
	defer pm.stateMutex.RUnlock()

	stateData, err := json.Marshal(pm.backendPool.ListBackends())
	if err != nil {
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/update_state", peer), bytes.NewBuffer(stateData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non-OK response from peer %s: %s", peer, resp.Status)
	}
	return nil
}

// UpdateState updates the local backend pool state with data from a peer
func (pm *PeerManager) UpdateState(newState []Backend) {
	pm.stateMutex.Lock()
	defer pm.stateMutex.Unlock()

	for i := range newState {
		pm.backendPool.AddOrUpdateBackend(&newState[i])
	}
}

// AddOrUpdateBackend adds a backend if it doesn't exist or updates it if it does
func (bp *BackendPool) AddOrUpdateBackend(newBackend *Backend) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	existingBackend, exists := bp.backends[newBackend.Address]
	if exists {
		existingBackend.ActiveConnections = newBackend.ActiveConnections
		existingBackend.Health = newBackend.Health
	} else {
		bp.backends[newBackend.Address] = newBackend
	}
}
