package loadbalancer

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// HealthChecker periodically checks the health of backends
type HealthChecker struct {
	backendPool *BackendPool
	interval    time.Duration
	stopChan    chan struct{}
	httpClient  *http.Client
}

// NewHealthChecker creates a new HealthChecker
func NewHealthChecker(backendPool *BackendPool, interval time.Duration) *HealthChecker {
	// Create an HTTP client with reasonable timeouts
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &HealthChecker{
		backendPool: backendPool,
		interval:    interval,
		stopChan:    make(chan struct{}),
		httpClient:  httpClient,
	}
}

// Start begins the periodic health checks
func (hc *HealthChecker) Start() {
	ticker := time.NewTicker(hc.interval)
	go func() {
		// Run an initial health check immediately
		hc.checkHealth()
		
		for {
			select {
			case <-ticker.C:
				hc.checkHealth()
			case <-hc.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
	log.Println("Health checker started with interval:", hc.interval)
}

// Stop halts the periodic health checks
func (hc *HealthChecker) Stop() {
	close(hc.stopChan)
	log.Println("Health checker stopped")
}

// checkHealth checks the health of all backends and updates their status
func (hc *HealthChecker) checkHealth() {
	log.Println("Running health checks on all backends")
	
	hc.backendPool.mu.RLock()
	backends := make([]*Backend, 0, len(hc.backendPool.backends))
	for _, backend := range hc.backendPool.backends {
		backends = append(backends, backend)
	}
	hc.backendPool.mu.RUnlock()
	
	var wg sync.WaitGroup
	for _, backend := range backends {
		wg.Add(1)
		go func(b *Backend) {
			defer wg.Done()
			hc.checkBackendHealth(b)
		}(backend)
	}
	wg.Wait()
	log.Println("Completed health checks for all backends")
}

// checkBackendHealth performs a health check on a single backend
func (hc *HealthChecker) checkBackendHealth(backend *Backend) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	var err error
	switch backend.Protocol {
	case "http", "https":
		err = hc.checkHTTPHealth(ctx, backend)
	case "mysql":
		err = hc.checkMySQLHealth(ctx, backend)
	case "postgres":
		err = hc.checkPostgresHealth(ctx, backend)
	default:
		log.Printf("Unsupported protocol for health check: %s", backend.Protocol)
		return
	}

	previousHealth := backend.Health
	backend.mu.Lock()
	if err != nil {
		backend.Health = false
		if previousHealth {
			log.Printf("Backend %s is now UNHEALTHY: %v", backend.Address, err)
		}
	} else {
		backend.Health = true
		if !previousHealth {
			log.Printf("Backend %s is now HEALTHY", backend.Address)
		}
	}
	backend.mu.Unlock()
}

// checkHTTPHealth performs a health check on an HTTP backend
func (hc *HealthChecker) checkHTTPHealth(ctx context.Context, backend *Backend) error {
	protocol := "http"
	if backend.Protocol == "https" {
		protocol = "https"
	}
	
	url := fmt.Sprintf("%s://%s:%d/health", protocol, backend.Address, backend.Port)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	
	resp, err := hc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	return nil
}

// checkMySQLHealth performs a health check on a MySQL backend
func (hc *HealthChecker) checkMySQLHealth(ctx context.Context, backend *Backend) error {
	dsn := fmt.Sprintf("healthcheck:healthcheck@tcp(%s:%d)/?timeout=5s", 
		backend.Address, backend.Port)
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open MySQL connection: %v", err)
	}
	defer db.Close()
	
	// Set connection limits
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute)
	
	// Use context for timeout control
	return db.PingContext(ctx)
}

// checkPostgresHealth performs a health check on a PostgreSQL backend
func (hc *HealthChecker) checkPostgresHealth(ctx context.Context, backend *Backend) error {
	dsn := fmt.Sprintf("postgres://healthcheck:healthcheck@%s:%d/?sslmode=disable&connect_timeout=5", 
		backend.Address, backend.Port)
	
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open PostgreSQL connection: %v", err)
	}
	defer db.Close()
	
	// Set connection limits
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute)
	
	// Use context for timeout control
	return db.PingContext(ctx)
}