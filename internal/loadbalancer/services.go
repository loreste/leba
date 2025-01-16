package loadbalancer

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
)

// routeRequest routes a request based on the protocol and ensures proper backend selection.
func (lb *LoadBalancer) routeRequest(protocol string) (*Backend, error) {
	log.Printf("Routing request for protocol: %s", protocol)

	lb.backendPool.mu.RLock()
	defer lb.backendPool.mu.RUnlock()

	var selectedBackend *Backend
	for _, backend := range lb.backendPool.backends {
		if backend.Protocol == protocol && backend.Health {
			if !lb.isPortAllowed(protocol, backend.Port) {
				log.Printf("Port %d is not allowed for protocol: %s", backend.Port, protocol)
				continue
			}
			if selectedBackend == nil || backend.ActiveConnections < selectedBackend.ActiveConnections {
				selectedBackend = backend
			}
		}
	}

	if selectedBackend == nil {
		errMsg := fmt.Sprintf("No available backend for protocol: %s", protocol)
		log.Println(errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	log.Printf("Selected backend for protocol %s: %s:%d", protocol, selectedBackend.Address, selectedBackend.Port)
	selectedBackend.mu.Lock()
	selectedBackend.ActiveConnections++
	selectedBackend.mu.Unlock()

	return selectedBackend, nil
}

// isPortAllowed checks if the given port is allowed for the protocol.
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
	log.Printf("Port %d is not allowed for protocol: %s", port, protocol)
	return false
}

// StartService starts a specific protocol service.
func (lb *LoadBalancer) StartService(protocol string, port int) error {
	address := fmt.Sprintf(":%d", port)
	log.Printf("Starting %s service on %s", protocol, address)

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

	case "sip":
		return lb.startSIP(address)

	case "sip_tls":
		return lb.startSIPTLS(address)

	default:
		log.Printf("Unsupported protocol: %s", protocol)
		return fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// handleHTTP handles HTTP requests.
func (lb *LoadBalancer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling HTTP request: %s", r.URL.Path)
	backend, err := lb.routeRequest("http")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to route HTTP request: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("http://%s%s", backend.Address, r.URL.Path), http.StatusTemporaryRedirect)
}

// handleHTTPS handles HTTPS requests.
func (lb *LoadBalancer) handleHTTPS(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handling HTTPS request: %s", r.URL.Path)
	backend, err := lb.routeRequest("https")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to route HTTPS request: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("https://%s%s", backend.Address, r.URL.Path), http.StatusTemporaryRedirect)
}

// startPostgres handles PostgreSQL connections.
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
		go lb.handleDatabaseConnection(conn, "postgres")
	}
}

// startMySQL handles MySQL connections.
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
		go lb.handleDatabaseConnection(conn, "mysql")
	}
}

// startSIP handles SIP connections.
func (lb *LoadBalancer) startSIP(address string) error {
	log.Printf("Starting SIP listener on %s", address)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start SIP listener: %v", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("SIP connection error: %v", err)
			continue
		}
		go lb.handleSIPConnection(conn, "sip")
	}
}

// startSIPTLS handles SIP connections over TLS.
func (lb *LoadBalancer) startSIPTLS(address string) error {
	log.Printf("Starting SIP-TLS listener on %s", address)
	listener, err := tls.Listen("tcp", address, lb.tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to start SIP-TLS listener: %v", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("SIP-TLS connection error: %v", err)
			continue
		}
		go lb.handleSIPConnection(conn, "sip_tls")
	}
}

// handleDatabaseConnection manages database connections.
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

	go func() {
		io.Copy(backendConn, conn)
	}()
	io.Copy(conn, backendConn)
}

// handleSIPConnection manages SIP connections.
func (lb *LoadBalancer) handleSIPConnection(conn net.Conn, protocol string) {
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

	go func() {
		io.Copy(backendConn, conn)
	}()
	io.Copy(conn, backendConn)
}
