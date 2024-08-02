// internal/loadbalancer/loadbalancer.go
package loadbalancer

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"leba/internal/config"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/patrickmn/go-cache"
)

// LoadBalancer represents the load balancer
type LoadBalancer struct {
	frontendAddress string
	backendPool     *BackendPool
	tlsConfig       *tls.Config
	cache           *cache.Cache
	certFile        string
	keyFile         string
}

// NewLoadBalancer creates a new LoadBalancer
func NewLoadBalancer(frontendAddress string, backendPool *BackendPool, tlsConfig *tls.Config, cacheConfig config.CacheConfig, certFile, keyFile string) *LoadBalancer {
	var c *cache.Cache
	if cacheConfig.Enabled {
		c = cache.New(time.Duration(cacheConfig.Expiration)*time.Second, time.Duration(cacheConfig.CleanupInterval)*time.Second)
	}
	return &LoadBalancer{
		frontendAddress: frontendAddress,
		backendPool:     backendPool,
		tlsConfig:       tlsConfig,
		cache:           c,
		certFile:        certFile,
		keyFile:         keyFile,
	}
}

// Start starts the load balancer
func (lb *LoadBalancer) Start() error {
	if lb.tlsConfig != nil && lb.certFile != "" && lb.keyFile != "" {
		return lb.startTLSServer()
	}
	return lb.startHTTPServer()
}

func (lb *LoadBalancer) startHTTPServer() error {
	http.HandleFunc("/", lb.handleRequest)
	log.Printf("Starting HTTP server on %s", lb.frontendAddress)
	return http.ListenAndServe(lb.frontendAddress, nil)
}

func (lb *LoadBalancer) startTLSServer() error {
	http.HandleFunc("/", lb.handleRequest)
	log.Printf("Starting HTTPS server on %s", lb.frontendAddress)
	return http.ListenAndServeTLS(lb.frontendAddress, lb.certFile, lb.keyFile, nil)
}

func (lb *LoadBalancer) handleRequest(w http.ResponseWriter, r *http.Request) {
	backend, err := lb.selectLeastConnectionsBackend()
	if err != nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	switch backend.Protocol {
	case "http", "https":
		lb.forwardHTTPRequest(w, r, backend)
	case "mysql":
		lb.forwardDatabaseRequest(w, r, backend, "mysql")
	case "postgresql":
		lb.forwardDatabaseRequest(w, r, backend, "postgres")
	default:
		http.Error(w, "Unsupported protocol", http.StatusBadRequest)
	}
}

func (lb *LoadBalancer) forwardHTTPRequest(w http.ResponseWriter, r *http.Request, backend *Backend) {
	req, err := http.NewRequest(r.Method, backend.Address+r.RequestURI, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	req.Header = r.Header.Clone()

	// Update X-Forwarded-For header
	clientIP := r.RemoteAddr
	if existingClientIP := r.Header.Get("X-Forwarded-For"); existingClientIP != "" {
		clientIP = fmt.Sprintf("%s, %s", existingClientIP, clientIP)
	}
	req.Header.Set("X-Forwarded-For", clientIP)

	// Set X-Forwarded-Proto header
	if r.TLS != nil {
		req.Header.Set("X-Forwarded-Proto", "https")
	} else {
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	// Preserve the original Host header
	req.Host = r.Host

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to forward request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (lb *LoadBalancer) forwardDatabaseRequest(w http.ResponseWriter, r *http.Request, backend *Backend, dbType string) {
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Query parameter is required", http.StatusBadRequest)
		return
	}

	query = strings.TrimSpace(query)
	normalizedQuery := strings.ToLower(query)

	if lb.cache != nil {
		if cachedResult, found := lb.cache.Get(normalizedQuery); found {
			w.Write([]byte(cachedResult.(string)))
			return
		}
	}

	var dsn string
	if dbType == "mysql" {
		dsn = fmt.Sprintf("root@tcp(%s)/", backend.Address)
	} else if dbType == "postgres" {
		dsn = fmt.Sprintf("postgres://%s@%s/?sslmode=disable", "postgres", backend.Address)
	}

	db, err := sql.Open(dbType, dsn)
	if err != nil {
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	db.SetMaxOpenConns(backend.MaxOpenConnections)
	db.SetMaxIdleConns(backend.MaxIdleConnections)
	db.SetConnMaxLifetime(time.Duration(backend.ConnMaxLifetime) * time.Second)

	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, "Failed to execute query", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []string
	columns, err := rows.Columns()
	if err != nil {
		http.Error(w, "Failed to fetch columns", http.StatusInternalServerError)
		return
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			http.Error(w, "Failed to read query result", http.StatusInternalServerError)
			return
		}

		rowResult := make(map[string]interface{})
		for i, col := range columns {
			rowResult[col] = values[i]
		}
		result, _ := json.Marshal(rowResult)
		results = append(results, string(result))
	}

	finalResult := strings.Join(results, "\n")

	if lb.cache != nil {
		lb.cache.Set(normalizedQuery, finalResult, cache.DefaultExpiration)
	}

	w.Write([]byte(finalResult))
}

func (lb *LoadBalancer) selectLeastConnectionsBackend() (*Backend, error) {
	lb.backendPool.mu.RLock()
	defer lb.backendPool.mu.RUnlock()

	var selectedBackend *Backend
	for _, backend := range lb.backendPool.backends {
		if !backend.Health {
			continue
		}
		if selectedBackend == nil || backend.ActiveConnections < selectedBackend.ActiveConnections {
			selectedBackend = backend
		}
	}

	if selectedBackend == nil {
		return nil, errors.New("no healthy backends available")
	}

	return selectedBackend, nil
}
