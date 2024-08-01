package loadbalancer

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"
)

// HealthChecker periodically checks the health of backends
type HealthChecker struct {
	backendPool *BackendPool
	interval    time.Duration
	stopChan    chan struct{}
}

// NewHealthChecker creates a new HealthChecker
func NewHealthChecker(backendPool *BackendPool, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		backendPool: backendPool,
		interval:    interval,
		stopChan:    make(chan struct{}),
	}
}

// Start begins the periodic health checks
func (hc *HealthChecker) Start() {
	ticker := time.NewTicker(hc.interval)
	go func() {
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
}

// Stop halts the periodic health checks
func (hc *HealthChecker) Stop() {
	close(hc.stopChan)
}

// checkHealth checks the health of all backends and updates their status
func (hc *HealthChecker) checkHealth() {
	hc.backendPool.mu.RLock()
	defer hc.backendPool.mu.RUnlock()

	for _, backend := range hc.backendPool.backends {
		go hc.checkBackendHealth(backend)
	}
}

// checkBackendHealth performs a health check on a single backend
func (hc *HealthChecker) checkBackendHealth(backend *Backend) {
	var err error
	switch backend.Protocol {
	case "http", "https":
		err = hc.checkHTTPHealth(backend)
	case "mysql":
		err = hc.checkMySQLHealth(backend)
	case "postgresql":
		err = hc.checkPostgresHealth(backend)
	default:
		log.Printf("Unsupported protocol for health check: %s", backend.Protocol)
		return
	}

	backend.mu.Lock()
	if err != nil {
		backend.Health = false
		log.Printf("Backend %s is unhealthy: %v", backend.Address, err)
	} else {
		backend.Health = true
		log.Printf("Backend %s is healthy", backend.Address)
	}
	backend.mu.Unlock()
}

// checkHTTPHealth performs a health check on an HTTP backend
func (hc *HealthChecker) checkHTTPHealth(backend *Backend) error {
	resp, err := http.Get(backend.Address)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

// checkMySQLHealth performs a health check on a MySQL backend
func (hc *HealthChecker) checkMySQLHealth(backend *Backend) error {
	dsn := fmt.Sprintf("root@tcp(%s)/", backend.Address)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}

// checkPostgresHealth performs a health check on a PostgreSQL backend
func (hc *HealthChecker) checkPostgresHealth(backend *Backend) error {
	dsn := fmt.Sprintf("postgres://postgres@%s/?sslmode=disable", backend.Address)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}
