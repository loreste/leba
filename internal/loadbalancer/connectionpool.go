package loadbalancer

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	_ "github.com/lib/pq"              // PostgreSQL driver
)

// DBConnectionPool manages connection pools for database backends
type DBConnectionPool struct {
	mu       sync.RWMutex
	pools    map[string]*sql.DB
	poolInfo map[string]*PoolInfo
}

// PoolInfo contains information about a connection pool
type PoolInfo struct {
	Protocol          string
	Address           string
	Port              int
	MaxOpenConns      int
	MaxIdleConns      int
	ConnMaxLifetime   time.Duration
	Active            bool
	LastStatusMessage string
	LastStatusTime    time.Time
	Stats             sql.DBStats
}

// NewDBConnectionPool creates a new database connection pool manager
func NewDBConnectionPool() *DBConnectionPool {
	return &DBConnectionPool{
		pools:    make(map[string]*sql.DB),
		poolInfo: make(map[string]*PoolInfo),
	}
}

// AddBackend adds a backend to the connection pool
func (cp *DBConnectionPool) AddBackend(backend *Backend) error {
	if backend.Protocol != "mysql" && backend.Protocol != "postgres" {
		return fmt.Errorf("unsupported protocol for connection pool: %s", backend.Protocol)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%d", backend.Protocol, backend.Address, backend.Port)

	// Check if we already have this pool
	if _, exists := cp.pools[key]; exists {
		return nil // Already exists
	}

	var db *sql.DB
	var err error

	switch backend.Protocol {
	case "mysql":
		// Create MySQL DSN
		dsn := fmt.Sprintf("root:password@tcp(%s:%d)/", backend.Address, backend.Port)
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			return fmt.Errorf("failed to create MySQL connection pool: %v", err)
		}

	case "postgres":
		// Create PostgreSQL DSN
		dsn := fmt.Sprintf("postgres://postgres:password@%s:%d/?sslmode=disable", 
			backend.Address, backend.Port)
		db, err = sql.Open("postgres", dsn)
		if err != nil {
			return fmt.Errorf("failed to create PostgreSQL connection pool: %v", err)
		}
	}

	// Set pool configuration
	maxOpen := 10 // Default
	if backend.MaxOpenConnections > 0 {
		maxOpen = backend.MaxOpenConnections
	}
	db.SetMaxOpenConns(maxOpen)

	maxIdle := 5 // Default
	if backend.MaxIdleConnections > 0 {
		maxIdle = backend.MaxIdleConnections
	}
	db.SetMaxIdleConns(maxIdle)

	lifetime := 15 * time.Minute // Default
	if backend.ConnMaxLifetime > 0 {
		lifetime = time.Duration(backend.ConnMaxLifetime) * time.Second
	}
	db.SetConnMaxLifetime(lifetime)

	// Store connection pool and info
	cp.pools[key] = db
	cp.poolInfo[key] = &PoolInfo{
		Protocol:        backend.Protocol,
		Address:         backend.Address,
		Port:            backend.Port,
		MaxOpenConns:    maxOpen,
		MaxIdleConns:    maxIdle,
		ConnMaxLifetime: lifetime,
		Active:          true,
		LastStatusTime:  time.Now(),
	}

	// Start a goroutine to periodically check the connection
	go cp.monitorConnection(key)

	log.Printf("Added %s connection pool for %s:%d", 
		backend.Protocol, backend.Address, backend.Port)
	return nil
}

// GetConnection gets a database connection from the pool
func (cp *DBConnectionPool) GetConnection(protocol, address string, port int) (*sql.Conn, error) {
	key := fmt.Sprintf("%s:%s:%d", protocol, address, port)

	cp.mu.RLock()
	db, exists := cp.pools[key]
	info, infoExists := cp.poolInfo[key]
	cp.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no connection pool for %s:%d", address, port)
	}

	if !infoExists || !info.Active {
		return nil, fmt.Errorf("connection pool for %s:%d is not active", address, port)
	}

	// Get a connection from the pool with context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	conn, err := db.Conn(ctx)
	if err != nil {
		cp.mu.Lock()
		if info, ok := cp.poolInfo[key]; ok {
			info.LastStatusMessage = err.Error()
			info.LastStatusTime = time.Now()
		}
		cp.mu.Unlock()
		return nil, fmt.Errorf("failed to get connection from pool: %v", err)
	}

	return conn, nil
}

// RemoveBackend removes a backend from the connection pool
func (cp *DBConnectionPool) RemoveBackend(protocol, address string, port int) error {
	key := fmt.Sprintf("%s:%s:%d", protocol, address, port)

	cp.mu.Lock()
	defer cp.mu.Unlock()

	db, exists := cp.pools[key]
	if !exists {
		return nil // Already removed
	}

	// Close the pool
	if err := db.Close(); err != nil {
		return fmt.Errorf("error closing connection pool: %v", err)
	}

	// Remove from maps
	delete(cp.pools, key)
	delete(cp.poolInfo, key)

	log.Printf("Removed %s connection pool for %s:%d", protocol, address, port)
	return nil
}

// monitorConnection periodically checks if a connection is still valid
func (cp *DBConnectionPool) monitorConnection(key string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cp.mu.RLock()
		db, exists := cp.pools[key]
		_, infoExists := cp.poolInfo[key] // Changed variable name to avoid unused variable
		cp.mu.RUnlock()

		if !exists || !infoExists {
			return // Pool was removed
		}

		// Check connection with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := db.PingContext(ctx)
		cancel()

		cp.mu.Lock()
		if poolInfo, ok := cp.poolInfo[key]; ok {
			poolInfo.Stats = db.Stats() // Update stats

			if err != nil {
				poolInfo.Active = false
				poolInfo.LastStatusMessage = err.Error()
				poolInfo.LastStatusTime = time.Now()
				log.Printf("Connection pool %s is unhealthy: %v", key, err)
			} else if !poolInfo.Active {
				// Transition from inactive to active
				poolInfo.Active = true
				poolInfo.LastStatusMessage = "Connection restored"
				poolInfo.LastStatusTime = time.Now()
				log.Printf("Connection pool %s is now healthy", key)
			}
		}
		cp.mu.Unlock()
	}
}

// GetActivePoolInfo gets information about all active connection pools
func (cp *DBConnectionPool) GetActivePoolInfo() []*PoolInfo {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	result := make([]*PoolInfo, 0, len(cp.poolInfo))
	for _, info := range cp.poolInfo {
		if info.Active {
			// Make a copy to avoid race conditions
			infoCopy := *info
			result = append(result, &infoCopy)
		}
	}
	return result
}

// GetPoolInfo gets information about a specific connection pool
func (cp *DBConnectionPool) GetPoolInfo(protocol, address string, port int) (*PoolInfo, bool) {
	key := fmt.Sprintf("%s:%s:%d", protocol, address, port)

	cp.mu.RLock()
	defer cp.mu.RUnlock()

	info, exists := cp.poolInfo[key]
	if !exists {
		return nil, false
	}

	// Make a copy to avoid race conditions
	infoCopy := *info
	return &infoCopy, true
}

// GetBackendConn gets a database connection for a specific backend
func (cp *DBConnectionPool) GetBackendConn(backend *Backend) (*sql.Conn, error) {
	if backend.Protocol != "mysql" && backend.Protocol != "postgres" {
		return nil, fmt.Errorf("protocol not supported for connection pooling: %s", backend.Protocol)
	}
	
	// Add backend to pool if it doesn't exist
	key := fmt.Sprintf("%s:%s:%d", backend.Protocol, backend.Address, backend.Port)
	cp.mu.RLock()
	_, exists := cp.pools[key]
	cp.mu.RUnlock()
	
	if !exists {
		if err := cp.AddBackend(backend); err != nil {
			return nil, err
		}
	}
	
	// Get connection from pool
	return cp.GetConnection(backend.Protocol, backend.Address, backend.Port)
}