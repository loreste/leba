package loadbalancer

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"sync"
)

// MTLSManager manages mutual TLS connections to backends
type MTLSManager struct {
	clientCerts     map[string]*tls.Certificate
	caCerts         map[string]*x509.CertPool
	mu              sync.RWMutex
	defaultCertPool *x509.CertPool
}

// NewMTLSManager creates a new mTLS manager
func NewMTLSManager() *MTLSManager {
	return &MTLSManager{
		clientCerts: make(map[string]*tls.Certificate),
		caCerts:     make(map[string]*x509.CertPool),
	}
}

// AddClientCert adds a client certificate for a backend
func (m *MTLSManager) AddClientCert(backendID string, certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load client certificate: %v", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.clientCerts[backendID] = &cert

	log.Printf("Added client certificate for backend %s", backendID)
	return nil
}

// AddCACert adds a CA certificate for a backend
func (m *MTLSManager) AddCACert(backendID string, caFile string) error {
	caData, err := os.ReadFile(caFile)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %v", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caData) {
		return fmt.Errorf("failed to parse CA certificate")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.caCerts[backendID] = certPool

	log.Printf("Added CA certificate for backend %s", backendID)
	return nil
}

// SetDefaultCAs sets the default CA certificate pool
func (m *MTLSManager) SetDefaultCAs(caFile string) error {
	caData, err := os.ReadFile(caFile)
	if err != nil {
		return fmt.Errorf("failed to read default CA certificate: %v", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caData) {
		return fmt.Errorf("failed to parse default CA certificate")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultCertPool = certPool

	log.Printf("Set default CA certificate pool")
	return nil
}

// GetTLSConfig returns a TLS config for a specific backend
func (m *MTLSManager) GetTLSConfig(backendID string) *tls.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Start with secure defaults
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	// Add client certificate if available
	if cert, ok := m.clientCerts[backendID]; ok {
		config.Certificates = []tls.Certificate{*cert}
	}

	// Add CA cert if available, otherwise use default
	if caPool, ok := m.caCerts[backendID]; ok {
		config.RootCAs = caPool
	} else if m.defaultCertPool != nil {
		config.RootCAs = m.defaultCertPool
	}

	return config
}

// RemoveBackend removes certificates for a backend
func (m *MTLSManager) RemoveBackend(backendID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.clientCerts, backendID)
	delete(m.caCerts, backendID)

	log.Printf("Removed certificates for backend %s", backendID)
}

// HasCertificatesFor checks if there are certificates for a backend
func (m *MTLSManager) HasCertificatesFor(backendID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, hasCert := m.clientCerts[backendID]
	_, hasCA := m.caCerts[backendID]

	return hasCert || hasCA
}