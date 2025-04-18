package loadbalancer

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

// APIServer provides management API for the load balancer
type APIServer struct {
	lb            *LoadBalancer
	server        *http.Server
	jwtSecret     []byte
	allowedIPs    map[string]bool
	ipMutex       sync.RWMutex
	rateLimiters  map[string]*rate.Limiter
	limitersMutex sync.RWMutex
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	Enabled  bool
	JWTKey   string
	Username string
	Password string
}

// Claims defines JWT claims for API auth
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// NewAPIServer creates a new API server
func NewAPIServer(lb *LoadBalancer, port string, authConfig AuthConfig) *APIServer {
	api := &APIServer{
		lb:           lb,
		jwtSecret:    []byte(authConfig.JWTKey),
		allowedIPs:   make(map[string]bool),
		rateLimiters: make(map[string]*rate.Limiter),
	}

	// Set up HTTP router with authentication and security middleware
	mux := http.NewServeMux()

	// Public health endpoint - no auth required
	mux.HandleFunc("/api/health", api.healthHandler)

	// Admin API endpoints - require authentication
	mux.HandleFunc("/api/backends", api.authMiddleware(api.rateLimit(api.ipFilter(api.backendHandler))))
	mux.HandleFunc("/api/stats", api.authMiddleware(api.rateLimit(api.ipFilter(api.statsHandler))))
	mux.HandleFunc("/api/config", api.authMiddleware(api.rateLimit(api.ipFilter(api.configHandler))))
	mux.HandleFunc("/api/ip-allowlist", api.authMiddleware(api.rateLimit(api.ipFilter(api.ipAllowlistHandler))))

	// Authentication endpoint
	mux.HandleFunc("/api/login", api.loginHandler(authConfig))

	// Create server with reasonable timeouts
	api.server = &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return api
}

// Start starts the API server
func (api *APIServer) Start() error {
	log.Printf("Starting API server on %s", api.server.Addr)
	return api.server.ListenAndServe()
}

// Stop stops the API server
func (api *APIServer) Stop() error {
	log.Println("Stopping API server")
	return api.server.Close()
}

// loginHandler handles user authentication and returns a JWT token
func (api *APIServer) loginHandler(authConfig AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var creds struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		err := json.NewDecoder(r.Body).Decode(&creds)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Simple authentication - in production, use a more secure approach
		if creds.Username != authConfig.Username || creds.Password != authConfig.Password {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Create the JWT claims with expiration
		expirationTime := time.Now().Add(24 * time.Hour)
		claims := &Claims{
			Username: creds.Username,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expirationTime),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				NotBefore: jwt.NewNumericDate(time.Now()),
				Issuer:    "leba-loadbalancer",
				Subject:   creds.Username,
			},
		}

		// Generate token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(api.jwtSecret)
		if err != nil {
			log.Printf("Error generating JWT token: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Return the token
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token": tokenString,
		})
	}
}

// authMiddleware adds JWT authentication to handlers
func (api *APIServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Check if it's a Bearer token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		tokenString := tokenParts[1]

		// Parse and validate the token
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			// Validate the signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return api.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Add the username to the request context
		// In Go 1.7+, we would use context.WithValue, but for simplicity, we'll just pass through
		next(w, r)
	}
}

// rateLimit implements rate limiting middleware
func (api *APIServer) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		// Get or create rate limiter for this IP
		api.limitersMutex.Lock()
		limiter, exists := api.rateLimiters[ip]
		if !exists {
			// Allow 5 requests per second with a burst of 10
			limiter = rate.NewLimiter(5, 10)
			api.rateLimiters[ip] = limiter
		}
		api.limitersMutex.Unlock()

		// Check if request is allowed
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}

// ipFilter implements IP allowlisting middleware
func (api *APIServer) ipFilter(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		// Check if IP allowlisting is active
		api.ipMutex.RLock()
		allowlistActive := len(api.allowedIPs) > 0
		allowed := api.allowedIPs[ip]
		api.ipMutex.RUnlock()

		// If allowlisting is active and IP is not allowed, block
		if allowlistActive && !allowed {
			log.Printf("Blocked request from unauthorized IP: %s", ip)
			http.Error(w, "IP not authorized", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

// ipAllowlistHandler manages the IP allowlist
func (api *APIServer) ipAllowlistHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Return current allowlist
		api.ipMutex.RLock()
		allowlist := make([]string, 0, len(api.allowedIPs))
		for ip := range api.allowedIPs {
			allowlist = append(allowlist, ip)
		}
		api.ipMutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"allowed_ips": allowlist,
		})

	case http.MethodPost:
		// Add IP to allowlist
		var req struct {
			IP string `json:"ip"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate IP
		if net.ParseIP(req.IP) == nil {
			http.Error(w, "Invalid IP address", http.StatusBadRequest)
			return
		}

		api.ipMutex.Lock()
		api.allowedIPs[req.IP] = true
		api.ipMutex.Unlock()

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("IP %s added to allowlist", req.IP),
		})

	case http.MethodDelete:
		// Remove IP from allowlist
		ip := r.URL.Query().Get("ip")
		if ip == "" {
			http.Error(w, "IP parameter required", http.StatusBadRequest)
			return
		}

		api.ipMutex.Lock()
		delete(api.allowedIPs, ip)
		api.ipMutex.Unlock()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("IP %s removed from allowlist", ip),
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// healthHandler returns the health status of the load balancer
func (api *APIServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check backend pool health
	healthy := false
	totalBackends := 0
	healthyBackends := 0

	api.lb.backendPool.mu.RLock()
	for _, backend := range api.lb.backendPool.backends {
		totalBackends++
		if backend.IsAvailable() {
			healthyBackends++
			healthy = true
		}
	}
	api.lb.backendPool.mu.RUnlock()

	status := "healthy"
	statusCode := http.StatusOK

	if !healthy {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           status,
		"total_backends":   totalBackends,
		"healthy_backends": healthyBackends,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	})
}

// backendHandler manages backend servers
func (api *APIServer) backendHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// List all backends
		api.lb.backendPool.mu.RLock()
		backends := api.lb.backendPool.ListBackends()
		api.lb.backendPool.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"backends": backends,
		})

	case http.MethodPost:
		// Add a new backend
		var backend Backend
		if err := json.NewDecoder(r.Body).Decode(&backend); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate backend
		if backend.Address == "" || backend.Protocol == "" || backend.Port <= 0 {
			http.Error(w, "Address, protocol, and port are required", http.StatusBadRequest)
			return
		}

		// Add to pool
		api.lb.backendPool.AddBackend(&backend)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("Backend %s:%d added", backend.Address, backend.Port),
		})

	case http.MethodDelete:
		// Remove a backend
		address := r.URL.Query().Get("address")
		if address == "" {
			http.Error(w, "Address parameter required", http.StatusBadRequest)
			return
		}

		api.lb.backendPool.RemoveBackend(address)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("Backend %s removed", address),
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// statsHandler returns statistics about the load balancer
func (api *APIServer) statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Collect stats
	totalBackends := 0
	healthyBackends := 0
	activeConnections := 0
	
	api.lb.backendPool.mu.RLock()
	for _, backend := range api.lb.backendPool.backends {
		totalBackends++
		if backend.IsAvailable() {
			healthyBackends++
		}
		activeConnections += backend.ActiveConnections
	}
	api.lb.backendPool.mu.RUnlock()
	
	// Get cache stats if available
	var cacheStats map[string]interface{}
	
	// Get connection pool stats
	poolStats := api.lb.connectionPool.GetActivePoolInfo()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_backends":     totalBackends,
		"healthy_backends":   healthyBackends,
		"active_connections": activeConnections,
		"cache":              cacheStats,
		"connection_pools":   poolStats,
		"timestamp":          time.Now().UTC().Format(time.RFC3339),
	})
}

// configHandler manages load balancer configuration
func (api *APIServer) configHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Get current configuration
		api.lb.mu.RLock()
		config := map[string]interface{}{
			"allowed_ports": api.lb.allowedPorts,
		}
		api.lb.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)

	case http.MethodPatch:
		// Update specific config values
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Process updates (for now, just allowed_ports)
		if portsUpdate, ok := updates["allowed_ports"].(map[string]interface{}); ok {
			newAllowedPorts := make(map[string][]int)
			
			for protocol, ports := range portsUpdate {
				if portsArray, ok := ports.([]interface{}); ok {
					intPorts := make([]int, 0, len(portsArray))
					for _, p := range portsArray {
						if portNum, ok := p.(float64); ok {
							intPorts = append(intPorts, int(portNum))
						}
					}
					newAllowedPorts[protocol] = intPorts
				}
			}
			
			api.lb.mu.Lock()
			api.lb.allowedPorts = newAllowedPorts
			api.lb.mu.Unlock()
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Configuration updated",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}