package loadbalancer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// UniversalHealthChecker provides health checking for all protocol types
type UniversalHealthChecker struct {
	// Core configuration
	lb                *LoadBalancer
	checkInterval     time.Duration
	timeout           time.Duration
	maxRetries        int
	failureThreshold  int32
	recoveryThreshold int32

	// Protocol-specific checkers
	httpChecker    *HTTPHealthChecker
	tcpChecker     *TCPHealthChecker
	grpcChecker    *GRPCHealthChecker
	sipChecker     *EnhancedSIPHealthChecker
	customCheckers sync.Map // map[string]ProtocolHealthChecker

	// Backend status tracking
	backendStatus sync.Map // map[string]*UniversalBackendStatus

	// Statistics
	totalChecks       atomic.Int64
	totalFailures     atomic.Int64
	healthyBackends   atomic.Int32
	unhealthyBackends atomic.Int32

	// Control
	ctx    context.Context
	cancel context.CancelFunc

	// Events
	onBackendUp      func(backend *Backend, protocol string)
	onBackendDown    func(backend *Backend, protocol string, reason string)
	onBackendRecover func(backend *Backend, protocol string)

	// Advanced features
	enableCircuitBreaker  bool
	enableGradualRecovery bool
	enableLoadTesting     bool
}

// UniversalBackendStatus tracks the health status of any backend type
type UniversalBackendStatus struct {
	// Backend identification
	Backend        *Backend
	Protocol       string
	HealthCheckURL string

	// Health status
	Healthy          atomic.Bool
	LastCheck        atomic.Int64
	LastSuccess      atomic.Int64
	ConsecutiveFails atomic.Int32
	TotalChecks      atomic.Int64
	SuccessfulChecks atomic.Int64

	// Performance metrics
	AverageResponseTime atomic.Int64 // microseconds
	LastResponseTime    atomic.Int64
	MinResponseTime     atomic.Int64
	MaxResponseTime     atomic.Int64

	// Circuit breaker
	CircuitOpen     atomic.Bool
	LastCircuitTrip atomic.Int64

	// Protocol-specific data
	ProtocolData sync.Map // Extensible protocol-specific information
}

// ProtocolHealthChecker interface for protocol-specific health checkers
type ProtocolHealthChecker interface {
	CheckHealth(backend *Backend, status *UniversalBackendStatus) (bool, time.Duration, error)
	GetProtocol() string
	GetDefaultPort() int
	SupportsBackend(backend *Backend) bool
}

// HTTPHealthChecker handles HTTP/HTTPS health checks
type HTTPHealthChecker struct {
	client          *http.Client
	expectedStatus  []int
	expectedBody    string
	expectedHeaders map[string]string
	followRedirects bool
	validateSSL     bool
}

// TCPHealthChecker handles basic TCP connectivity checks
type TCPHealthChecker struct {
	dialTimeout      time.Duration
	writeData        []byte
	expectResponse   bool
	expectedResponse []byte
}

// GRPCHealthChecker handles gRPC health checks using the gRPC Health Checking Protocol
type GRPCHealthChecker struct {
	serviceName string
	enableTLS   bool
	tlsConfig   *tls.Config
}

// UniversalHealthConfig configures the universal health checker
type UniversalHealthConfig struct {
	CheckInterval     time.Duration
	Timeout           time.Duration
	MaxRetries        int
	FailureThreshold  int32
	RecoveryThreshold int32

	// Protocol-specific configs
	HTTPConfig *HTTPHealthConfig
	TCPConfig  *TCPHealthConfig
	GRPCConfig *GRPCHealthConfig
	SIPConfig  *SIPHealthCheckConfig

	// Advanced features
	EnableCircuitBreaker  bool
	EnableGradualRecovery bool
	EnableLoadTesting     bool

	// Event callbacks
	OnBackendUp      func(backend *Backend, protocol string)
	OnBackendDown    func(backend *Backend, protocol string, reason string)
	OnBackendRecover func(backend *Backend, protocol string)
}

// HTTPHealthConfig configures HTTP health checking
type HTTPHealthConfig struct {
	Path            string
	Method          string
	ExpectedStatus  []int
	ExpectedBody    string
	ExpectedHeaders map[string]string
	Timeout         time.Duration
	FollowRedirects bool
	ValidateSSL     bool
	Headers         map[string]string
	BasicAuth       *BasicAuth
}

// TCPHealthConfig configures TCP health checking
type TCPHealthConfig struct {
	WriteData        []byte
	ExpectResponse   bool
	ExpectedResponse []byte
	DialTimeout      time.Duration
}

// GRPCHealthConfig configures gRPC health checking
type GRPCHealthConfig struct {
	ServiceName string
	EnableTLS   bool
	TLSConfig   *tls.Config
	Timeout     time.Duration
}

// BasicAuth represents basic authentication credentials
type BasicAuth struct {
	Username string
	Password string
}

// NewUniversalHealthChecker creates a new universal health checker
func NewUniversalHealthChecker(lb *LoadBalancer, config *UniversalHealthConfig) *UniversalHealthChecker {
	if config == nil {
		config = &UniversalHealthConfig{
			CheckInterval:     30 * time.Second,
			Timeout:           5 * time.Second,
			MaxRetries:        3,
			FailureThreshold:  3,
			RecoveryThreshold: 2,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	uhc := &UniversalHealthChecker{
		lb:                    lb,
		checkInterval:         config.CheckInterval,
		timeout:               config.Timeout,
		maxRetries:            config.MaxRetries,
		failureThreshold:      config.FailureThreshold,
		recoveryThreshold:     config.RecoveryThreshold,
		ctx:                   ctx,
		cancel:                cancel,
		onBackendUp:           config.OnBackendUp,
		onBackendDown:         config.OnBackendDown,
		onBackendRecover:      config.OnBackendRecover,
		enableCircuitBreaker:  config.EnableCircuitBreaker,
		enableGradualRecovery: config.EnableGradualRecovery,
		enableLoadTesting:     config.EnableLoadTesting,
	}

	// Initialize protocol-specific checkers
	uhc.httpChecker = NewHTTPHealthChecker(config.HTTPConfig)
	uhc.tcpChecker = NewTCPHealthChecker(config.TCPConfig)
	uhc.grpcChecker = NewGRPCHealthChecker(config.GRPCConfig)

	// Initialize SIP checker if configured
	if config.SIPConfig != nil {
		uhc.sipChecker = NewEnhancedSIPHealthChecker(lb, config.SIPConfig)
	}

	return uhc
}

// Start begins the universal health checking process
func (uhc *UniversalHealthChecker) Start() {
	// Start main health check loop
	go uhc.healthCheckLoop()

	// Start SIP checker if available
	if uhc.sipChecker != nil {
		uhc.sipChecker.Start()
	}

	// Start load testing if enabled
	if uhc.enableLoadTesting {
		go uhc.loadTestingLoop()
	}
}

// Stop stops the universal health checker
func (uhc *UniversalHealthChecker) Stop() {
	uhc.cancel()

	if uhc.sipChecker != nil {
		uhc.sipChecker.Stop()
	}
}

// healthCheckLoop runs the main health check loop
func (uhc *UniversalHealthChecker) healthCheckLoop() {
	ticker := time.NewTicker(uhc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			uhc.performHealthChecks()
		case <-uhc.ctx.Done():
			return
		}
	}
}

// performHealthChecks checks all backends
func (uhc *UniversalHealthChecker) performHealthChecks() {
	backends := uhc.lb.backendPool.ListBackends()

	var wg sync.WaitGroup
	for _, backend := range backends {
		wg.Add(1)
		go func(b *Backend) {
			defer wg.Done()
			uhc.checkBackend(b)
		}(backend)
	}

	wg.Wait()
}

// checkBackend performs health check on a single backend
func (uhc *UniversalHealthChecker) checkBackend(backend *Backend) {
	statusKey := fmt.Sprintf("%s:%d", backend.Address, backend.Port)

	// Get or create backend status
	status := uhc.getOrCreateBackendStatus(statusKey, backend)

	// Skip if circuit breaker is open
	if uhc.enableCircuitBreaker && status.CircuitOpen.Load() {
		if time.Now().Unix()-status.LastCircuitTrip.Load() < 60 { // 1 minute circuit open
			return
		}
		// Reset circuit breaker
		status.CircuitOpen.Store(false)
	}

	// Record check attempt
	status.TotalChecks.Add(1)
	status.LastCheck.Store(time.Now().Unix())
	uhc.totalChecks.Add(1)

	// Perform protocol-specific health check
	healthy, responseTime, err := uhc.performProtocolHealthCheck(backend, status)

	// Update response time metrics
	if responseTime > 0 {
		status.LastResponseTime.Store(responseTime.Microseconds())
		uhc.updateResponseTimeMetrics(status, responseTime.Microseconds())
	}

	// Update backend status
	uhc.updateBackendStatus(status, backend, healthy, err)
}

// performProtocolHealthCheck performs the appropriate health check based on protocol
func (uhc *UniversalHealthChecker) performProtocolHealthCheck(backend *Backend, status *UniversalBackendStatus) (bool, time.Duration, error) {
	protocol := strings.ToLower(backend.Protocol)

	// Try multiple attempts for reliability
	for attempt := 0; attempt < uhc.maxRetries; attempt++ {
		startTime := time.Now()

		var healthy bool

		switch protocol {
		case "http", "https":
			healthy, _, _ = uhc.httpChecker.CheckHealth(backend, status)
		case "tcp":
			healthy, _, _ = uhc.tcpChecker.CheckHealth(backend, status)
		case "grpc":
			healthy, _, _ = uhc.grpcChecker.CheckHealth(backend, status)
		case "sip":
			// SIP is handled by the dedicated SIP health checker
			if uhc.sipChecker != nil {
				return uhc.sipChecker.IsServerHealthy(backend.Address, backend.Port), time.Since(startTime), nil
			}
			// Fallback to TCP check for SIP
			healthy, _, _ = uhc.tcpChecker.CheckHealth(backend, status)
		default:
			// Check for custom protocol checker
			if checker, ok := uhc.customCheckers.Load(protocol); ok {
				protocolChecker := checker.(ProtocolHealthChecker)
				healthy, _, _ = protocolChecker.CheckHealth(backend, status)
			} else {
				// Default to TCP connectivity check
				healthy, _, _ = uhc.tcpChecker.CheckHealth(backend, status)
			}
		}

		responseTime := time.Since(startTime)

		if healthy {
			return true, responseTime, nil
		}

		// Wait before retry
		if attempt < uhc.maxRetries-1 {
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}

	return false, 0, fmt.Errorf("all health check attempts failed")
}

// updateBackendStatus updates the backend status based on health check result
func (uhc *UniversalHealthChecker) updateBackendStatus(status *UniversalBackendStatus, backend *Backend, healthy bool, err error) {
	wasHealthy := status.Healthy.Load()

	if healthy {
		// Health check succeeded
		status.SuccessfulChecks.Add(1)
		status.LastSuccess.Store(time.Now().Unix())
		status.ConsecutiveFails.Store(0)

		if !wasHealthy {
			// Backend is recovering
			if uhc.enableGradualRecovery {
				// Gradual recovery: require multiple successes
				if status.SuccessfulChecks.Load() >= int64(uhc.recoveryThreshold) {
					uhc.markBackendHealthy(status, backend)
				}
			} else {
				// Immediate recovery
				uhc.markBackendHealthy(status, backend)
			}
		} else {
			// Backend remains healthy
			status.Healthy.Store(true)
		}
	} else {
		// Health check failed
		consecutiveFails := status.ConsecutiveFails.Add(1)
		uhc.totalFailures.Add(1)

		if wasHealthy && consecutiveFails >= uhc.failureThreshold {
			// Backend has failed
			uhc.markBackendUnhealthy(status, backend, err)

			// Open circuit breaker if enabled and too many failures
			if uhc.enableCircuitBreaker && consecutiveFails >= uhc.failureThreshold*2 {
				status.CircuitOpen.Store(true)
				status.LastCircuitTrip.Store(time.Now().Unix())
			}
		}
	}
}

// markBackendHealthy marks a backend as healthy
func (uhc *UniversalHealthChecker) markBackendHealthy(status *UniversalBackendStatus, backend *Backend) {
	status.Healthy.Store(true)
	backend.health.Store(true)
	backend.Health = true

	// Update load balancer
	uhc.lb.backendPool.HandleSuccess(backend)

	// Update counters
	uhc.healthyBackends.Add(1)
	uhc.unhealthyBackends.Add(-1)

	// Call event handler
	if uhc.onBackendRecover != nil {
		go uhc.onBackendRecover(backend, backend.Protocol)
	}
}

// markBackendUnhealthy marks a backend as unhealthy
func (uhc *UniversalHealthChecker) markBackendUnhealthy(status *UniversalBackendStatus, backend *Backend, err error) {
	status.Healthy.Store(false)
	backend.health.Store(false)
	backend.Health = false

	// Update load balancer
	uhc.lb.backendPool.HandleFailure(backend)

	// Update counters
	uhc.healthyBackends.Add(-1)
	uhc.unhealthyBackends.Add(1)

	// Call event handler
	reason := "unknown"
	if err != nil {
		reason = err.Error()
	}

	if uhc.onBackendDown != nil {
		go uhc.onBackendDown(backend, backend.Protocol, reason)
	}
}

// updateResponseTimeMetrics updates response time statistics
func (uhc *UniversalHealthChecker) updateResponseTimeMetrics(status *UniversalBackendStatus, responseTime int64) {
	// Update average (simple moving average)
	current := status.AverageResponseTime.Load()
	if current == 0 {
		status.AverageResponseTime.Store(responseTime)
	} else {
		newAvg := (current*9 + responseTime) / 10
		status.AverageResponseTime.Store(newAvg)
	}

	// Update min/max
	for {
		currentMin := status.MinResponseTime.Load()
		if currentMin == 0 || responseTime < currentMin {
			if status.MinResponseTime.CompareAndSwap(currentMin, responseTime) {
				break
			}
		} else {
			break
		}
	}

	for {
		currentMax := status.MaxResponseTime.Load()
		if responseTime > currentMax {
			if status.MaxResponseTime.CompareAndSwap(currentMax, responseTime) {
				break
			}
		} else {
			break
		}
	}
}

// getOrCreateBackendStatus gets or creates backend status
func (uhc *UniversalHealthChecker) getOrCreateBackendStatus(statusKey string, backend *Backend) *UniversalBackendStatus {
	if statusInterface, ok := uhc.backendStatus.Load(statusKey); ok {
		return statusInterface.(*UniversalBackendStatus)
	}

	status := &UniversalBackendStatus{
		Backend:  backend,
		Protocol: backend.Protocol,
	}

	status.Healthy.Store(backend.health.Load())
	status.LastCheck.Store(time.Now().Unix())

	uhc.backendStatus.Store(statusKey, status)
	return status
}

// RegisterProtocolChecker registers a custom protocol health checker
func (uhc *UniversalHealthChecker) RegisterProtocolChecker(protocol string, checker ProtocolHealthChecker) {
	uhc.customCheckers.Store(protocol, checker)
}

// loadTestingLoop performs periodic load testing
func (uhc *UniversalHealthChecker) loadTestingLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			uhc.performLoadTesting()
		case <-uhc.ctx.Done():
			return
		}
	}
}

// performLoadTesting performs load testing on healthy backends
func (uhc *UniversalHealthChecker) performLoadTesting() {
	backends := uhc.lb.backendPool.ListBackends()

	for _, backend := range backends {
		if backend.health.Load() {
			go uhc.loadTestBackend(backend)
		}
	}
}

// loadTestBackend performs load testing on a single backend
func (uhc *UniversalHealthChecker) loadTestBackend(backend *Backend) {
	concurrency := 5
	var wg sync.WaitGroup
	successCount := atomic.Int32{}

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			statusKey := fmt.Sprintf("%s:%d", backend.Address, backend.Port)
			status := uhc.getOrCreateBackendStatus(statusKey, backend)

			healthy, _, _ := uhc.performProtocolHealthCheck(backend, status)
			if healthy {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	successRate := float64(successCount.Load()) / float64(concurrency) * 100
	fmt.Printf("Load test for %s:%d (%s) - Duration: %v, Success Rate: %.1f%%\n",
		backend.Address, backend.Port, backend.Protocol, duration, successRate)
}

// GetBackendStatus returns the status of all backends
func (uhc *UniversalHealthChecker) GetBackendStatus() map[string]*UniversalBackendStatus {
	status := make(map[string]*UniversalBackendStatus)

	uhc.backendStatus.Range(func(key, value interface{}) bool {
		status[key.(string)] = value.(*UniversalBackendStatus)
		return true
	})

	return status
}

// GetHealthyBackends returns all healthy backends
func (uhc *UniversalHealthChecker) GetHealthyBackends() []*Backend {
	var healthy []*Backend

	uhc.backendStatus.Range(func(key, value interface{}) bool {
		status := value.(*UniversalBackendStatus)
		if status.Healthy.Load() {
			healthy = append(healthy, status.Backend)
		}
		return true
	})

	return healthy
}

// GetHealthCheckStats returns overall health check statistics
func (uhc *UniversalHealthChecker) GetHealthCheckStats() UniversalHealthStats {
	totalBackends := int32(0)
	healthy := int32(0)
	unhealthy := int32(0)

	uhc.backendStatus.Range(func(key, value interface{}) bool {
		status := value.(*UniversalBackendStatus)
		totalBackends++
		if status.Healthy.Load() {
			healthy++
		} else {
			unhealthy++
		}
		return true
	})

	successRate := float64(0)
	if uhc.totalChecks.Load() > 0 {
		successRate = float64(uhc.totalChecks.Load()-uhc.totalFailures.Load()) / float64(uhc.totalChecks.Load()) * 100
	}

	return UniversalHealthStats{
		TotalChecks:       uhc.totalChecks.Load(),
		TotalFailures:     uhc.totalFailures.Load(),
		SuccessRate:       successRate,
		TotalBackends:     totalBackends,
		HealthyBackends:   healthy,
		UnhealthyBackends: unhealthy,
	}
}

// UniversalHealthStats contains overall health statistics
type UniversalHealthStats struct {
	TotalChecks       int64   `json:"total_checks"`
	TotalFailures     int64   `json:"total_failures"`
	SuccessRate       float64 `json:"success_rate"`
	TotalBackends     int32   `json:"total_backends"`
	HealthyBackends   int32   `json:"healthy_backends"`
	UnhealthyBackends int32   `json:"unhealthy_backends"`
}

// ForceHealthCheck forces a health check on a specific backend
func (uhc *UniversalHealthChecker) ForceHealthCheck(address string, port int) {
	backends := uhc.lb.backendPool.ListBackends()
	for _, backend := range backends {
		if backend.Address == address && backend.Port == port {
			go uhc.checkBackend(backend)
			break
		}
	}
}

// IsBackendHealthy checks if a specific backend is healthy
func (uhc *UniversalHealthChecker) IsBackendHealthy(address string, port int) bool {
	statusKey := fmt.Sprintf("%s:%d", address, port)
	if statusInterface, ok := uhc.backendStatus.Load(statusKey); ok {
		status := statusInterface.(*UniversalBackendStatus)
		return status.Healthy.Load()
	}
	return false
}

// Protocol-specific health checker implementations

// NewHTTPHealthChecker creates a new HTTP health checker
func NewHTTPHealthChecker(config *HTTPHealthConfig) *HTTPHealthChecker {
	if config == nil {
		config = &HTTPHealthConfig{
			Path:            "/health",
			Method:          "GET",
			ExpectedStatus:  []int{200},
			Timeout:         5 * time.Second,
			FollowRedirects: false,
			ValidateSSL:     true,
		}
	}

	client := &http.Client{
		Timeout: config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !config.FollowRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	if !config.ValidateSSL {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return &HTTPHealthChecker{
		client:          client,
		expectedStatus:  config.ExpectedStatus,
		expectedBody:    config.ExpectedBody,
		expectedHeaders: config.ExpectedHeaders,
		followRedirects: config.FollowRedirects,
		validateSSL:     config.ValidateSSL,
	}
}

// CheckHealth implements ProtocolHealthChecker for HTTP
func (hc *HTTPHealthChecker) CheckHealth(backend *Backend, status *UniversalBackendStatus) (bool, time.Duration, error) {
	// Determine scheme
	scheme := "http"
	if backend.Protocol == "https" {
		scheme = "https"
	}

	// Get health check path from backend config or use default
	path := "/health"
	if healthPath, ok := status.ProtocolData.Load("health_path"); ok {
		path = healthPath.(string)
	}

	// Build URL
	healthURL := fmt.Sprintf("%s://%s:%d%s", scheme, backend.Address, backend.Port, path)

	// Create request
	req, err := http.NewRequest("GET", healthURL, nil)
	if err != nil {
		return false, 0, fmt.Errorf("failed to create request: %v", err)
	}

	// Add headers
	req.Header.Set("User-Agent", "LEBA-HealthCheck/1.0")
	req.Header.Set("Accept", "*/*")

	start := time.Now()

	// Make request
	resp, err := hc.client.Do(req)
	if err != nil {
		return false, time.Since(start), fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	duration := time.Since(start)

	// Check status code
	statusOK := false
	for _, expectedStatus := range hc.expectedStatus {
		if resp.StatusCode == expectedStatus {
			statusOK = true
			break
		}
	}

	if !statusOK {
		return false, duration, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// TODO: Check expected body and headers if configured

	return true, duration, nil
}

func (hc *HTTPHealthChecker) GetProtocol() string { return "http" }
func (hc *HTTPHealthChecker) GetDefaultPort() int { return 80 }
func (hc *HTTPHealthChecker) SupportsBackend(backend *Backend) bool {
	return backend.Protocol == "http" || backend.Protocol == "https"
}

// NewTCPHealthChecker creates a new TCP health checker
func NewTCPHealthChecker(config *TCPHealthConfig) *TCPHealthChecker {
	if config == nil {
		config = &TCPHealthConfig{
			DialTimeout: 5 * time.Second,
		}
	}

	return &TCPHealthChecker{
		dialTimeout:      config.DialTimeout,
		writeData:        config.WriteData,
		expectResponse:   config.ExpectResponse,
		expectedResponse: config.ExpectedResponse,
	}
}

// CheckHealth implements ProtocolHealthChecker for TCP
func (tc *TCPHealthChecker) CheckHealth(backend *Backend, status *UniversalBackendStatus) (bool, time.Duration, error) {
	start := time.Now()

	// Connect to backend
	addr := fmt.Sprintf("%s:%d", backend.Address, backend.Port)
	conn, err := net.DialTimeout("tcp", addr, tc.dialTimeout)
	if err != nil {
		return false, time.Since(start), fmt.Errorf("TCP connection failed: %v", err)
	}
	defer conn.Close()

	// Set deadline
	conn.SetDeadline(time.Now().Add(tc.dialTimeout))

	// Write data if configured
	if len(tc.writeData) > 0 {
		_, err = conn.Write(tc.writeData)
		if err != nil {
			return false, time.Since(start), fmt.Errorf("TCP write failed: %v", err)
		}

		// Read response if expected
		if tc.expectResponse {
			buffer := make([]byte, 1024)
			_, err = conn.Read(buffer)
			if err != nil {
				return false, time.Since(start), fmt.Errorf("TCP read failed: %v", err)
			}

			// TODO: Validate expected response
		}
	}

	duration := time.Since(start)
	return true, duration, nil
}

func (tc *TCPHealthChecker) GetProtocol() string { return "tcp" }
func (tc *TCPHealthChecker) GetDefaultPort() int { return 80 }
func (tc *TCPHealthChecker) SupportsBackend(backend *Backend) bool {
	return backend.Protocol == "tcp"
}

// NewGRPCHealthChecker creates a new gRPC health checker
func NewGRPCHealthChecker(config *GRPCHealthConfig) *GRPCHealthChecker {
	if config == nil {
		config = &GRPCHealthConfig{
			ServiceName: "",
			EnableTLS:   false,
			Timeout:     5 * time.Second,
		}
	}

	return &GRPCHealthChecker{
		serviceName: config.ServiceName,
		enableTLS:   config.EnableTLS,
		tlsConfig:   config.TLSConfig,
	}
}

// CheckHealth implements ProtocolHealthChecker for gRPC
func (gc *GRPCHealthChecker) CheckHealth(backend *Backend, status *UniversalBackendStatus) (bool, time.Duration, error) {
	start := time.Now()

	// For now, fallback to TCP connectivity check
	// TODO: Implement proper gRPC health checking protocol
	addr := fmt.Sprintf("%s:%d", backend.Address, backend.Port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return false, time.Since(start), fmt.Errorf("gRPC connection failed: %v", err)
	}
	defer conn.Close()

	duration := time.Since(start)
	return true, duration, nil
}

func (gc *GRPCHealthChecker) GetProtocol() string { return "grpc" }
func (gc *GRPCHealthChecker) GetDefaultPort() int { return 443 }
func (gc *GRPCHealthChecker) SupportsBackend(backend *Backend) bool {
	return backend.Protocol == "grpc"
}

// Global universal health checker
var globalUniversalHealthChecker *UniversalHealthChecker

// InitializeUniversalHealthChecker initializes the global universal health checker
func InitializeUniversalHealthChecker(lb *LoadBalancer, config *UniversalHealthConfig) {
	globalUniversalHealthChecker = NewUniversalHealthChecker(lb, config)
	globalUniversalHealthChecker.Start()
}

// GetUniversalHealthChecker returns the global universal health checker
func GetUniversalHealthChecker() *UniversalHealthChecker {
	return globalUniversalHealthChecker
}
