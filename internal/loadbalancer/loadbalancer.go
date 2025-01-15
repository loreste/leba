package loadbalancer

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"crypto/tls"
	"leba/internal/config"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// LoadBalancer represents the main load balancer logic
type LoadBalancer struct {
	frontendAddress string
	backendPool     *BackendPool
	tlsConfig       *tls.Config
	cacheConfig     config.CacheConfig
	certFile        string
	keyFile         string
	mu              sync.RWMutex
}

// NewLoadBalancer creates a new LoadBalancer instance
func NewLoadBalancer(frontendAddress string, backendPool *BackendPool, tlsConfig *tls.Config, cacheConfig config.CacheConfig, certFile, keyFile string) *LoadBalancer {
	return &LoadBalancer{
		frontendAddress: frontendAddress,
		backendPool:     backendPool,
		tlsConfig:       tlsConfig,
		cacheConfig:     cacheConfig,
		certFile:        certFile,
		keyFile:         keyFile,
	}
}

// Start starts the load balancer's HTTP server
func (lb *LoadBalancer) Start() error {
	log.Printf("Starting load balancer on %s", lb.frontendAddress)
	http.HandleFunc("/", lb.handleRequest)
	return http.ListenAndServe(lb.frontendAddress, nil)
}

// handleRequest handles incoming HTTP requests and forwards them to a backend
func (lb *LoadBalancer) handleRequest(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Query parameter is missing", http.StatusBadRequest)
		return
	}

	backend, err := lb.routeQuery(query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to route request: %v", err), http.StatusInternalServerError)
		return
	}

	// Execute the query on the selected backend
	connStr := fmt.Sprintf("host=%s port=%d sslmode=disable", backend.Address, backend.Port)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to backend: %v", err), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Query execution failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			http.Error(w, fmt.Sprintf("Failed to read query result: %v", err), http.StatusInternalServerError)
			return
		}
		results = append(results, result)
	}

	backend.mu.Lock()
	backend.ActiveConnections--
	backend.mu.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strings.Join(results, "\n")))
}

// routeQuery routes the query based on type (read/write)
func (lb *LoadBalancer) routeQuery(query string) (*Backend, error) {
	lb.backendPool.mu.RLock()
	defer lb.backendPool.mu.RUnlock()

	var selectedBackend *Backend

	if isReadQuery(query) {
		// Route to the least-connected replica
		for _, backend := range lb.backendPool.backends {
			if backend.Role == "replica" && backend.Health {
				if selectedBackend == nil || backend.ActiveConnections < selectedBackend.ActiveConnections {
					selectedBackend = backend
				}
			}
		}
	} else {
		// Route to the primary
		for _, backend := range lb.backendPool.backends {
			if backend.Role == "primary" && backend.Health {
				selectedBackend = backend
				break
			}
		}
	}

	if selectedBackend == nil {
		return nil, fmt.Errorf("no available backend for query")
	}

	selectedBackend.mu.Lock()
	selectedBackend.ActiveConnections++
	selectedBackend.mu.Unlock()

	return selectedBackend, nil
}

// isReadQuery checks if a query is a read operation
func isReadQuery(query string) bool {
	query = strings.TrimSpace(strings.ToLower(query))
	return strings.HasPrefix(query, "select")
}
