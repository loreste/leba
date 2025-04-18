package loadbalancer

import (
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

// BalancingMethod defines the load balancing method
type BalancingMethod int

const (
	// LeastConnectionsMethod routes to backend with fewest active connections
	LeastConnectionsMethod BalancingMethod = iota
	// RoundRobinMethod distributes requests sequentially
	RoundRobinMethod
	// WeightedMethod routes based on backend weights
	WeightedMethod
	// IPHashMethod routes based on client IP hash
	IPHashMethod
)

// ContentRoutingRule defines a content-based routing rule
type ContentRoutingRule struct {
	// Name is a unique identifier for the rule
	Name string
	// PathPattern is a regex for matching request paths
	PathPattern *regexp.Regexp
	// HeaderName is the header to check
	HeaderName string
	// HeaderValue is the value to match
	HeaderValue string
	// HostPattern is a regex for matching host header
	HostPattern *regexp.Regexp
	// QueryParam is the query parameter to check
	QueryParam string
	// QueryValue is the query value to match
	QueryValue string
	// Enabled indicates if the rule is active
	Enabled bool
	// BackendAddress is the target backend
	BackendAddress string
	// BackendProtocol is the protocol to use
	BackendProtocol string
}

// Router manages request routing
type Router struct {
	// Current balancing method
	balancingMethod BalancingMethod
	// Content routing rules
	contentRules    []*ContentRoutingRule
	// Round robin counter
	rrCounter       uint64
	// Mutex for rules
	mu              sync.RWMutex
}

// NewRouter creates a new router
func NewRouter() *Router {
	return &Router{
		balancingMethod: LeastConnectionsMethod,
		contentRules:    make([]*ContentRoutingRule, 0),
	}
}

// SetBalancingMethod sets the load balancing method
func (r *Router) SetBalancingMethod(method BalancingMethod) {
	r.balancingMethod = method
}

// AddContentRule adds a content-based routing rule
func (r *Router) AddContentRule(rule *ContentRoutingRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.contentRules = append(r.contentRules, rule)
}

// RemoveContentRule removes a content-based routing rule by name
func (r *Router) RemoveContentRule(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	for i, rule := range r.contentRules {
		if rule.Name == name {
			// Remove by replacing with last element and truncating
			r.contentRules[i] = r.contentRules[len(r.contentRules)-1]
			r.contentRules = r.contentRules[:len(r.contentRules)-1]
			return true
		}
	}
	
	return false
}

// GetRuleByName gets a content routing rule by name
func (r *Router) GetRuleByName(name string) *ContentRoutingRule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, rule := range r.contentRules {
		if rule.Name == name {
			return rule
		}
	}
	
	return nil
}

// ListContentRules lists all content-based routing rules
func (r *Router) ListContentRules() []*ContentRoutingRule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	rules := make([]*ContentRoutingRule, len(r.contentRules))
	copy(rules, r.contentRules)
	
	return rules
}

// GetBackendForContentRoute checks if a request matches any content routing rules
func (r *Router) GetBackendForContentRoute(req *http.Request) (string, string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, rule := range r.contentRules {
		if !rule.Enabled {
			continue
		}
		
		// Check if request matches this rule
		if r.requestMatchesRule(req, rule) {
			return rule.BackendAddress, rule.BackendProtocol, true
		}
	}
	
	return "", "", false
}

// requestMatchesRule checks if a request matches a routing rule
func (r *Router) requestMatchesRule(req *http.Request, rule *ContentRoutingRule) bool {
	// Check path pattern
	if rule.PathPattern != nil && !rule.PathPattern.MatchString(req.URL.Path) {
		return false
	}
	
	// Check header
	if rule.HeaderName != "" {
		headerVal := req.Header.Get(rule.HeaderName)
		if headerVal == "" || !strings.Contains(headerVal, rule.HeaderValue) {
			return false
		}
	}
	
	// Check host pattern
	if rule.HostPattern != nil {
		host := req.Host
		if idx := strings.Index(host, ":"); idx >= 0 {
			host = host[:idx] // Remove port if present
		}
		if !rule.HostPattern.MatchString(host) {
			return false
		}
	}
	
	// Check query parameter
	if rule.QueryParam != "" {
		queryVal := req.URL.Query().Get(rule.QueryParam)
		if queryVal == "" || queryVal != rule.QueryValue {
			return false
		}
	}
	
	// All checks passed
	return true
}

// RouteRequest routes a request to a backend based on the current balancing method
func (r *Router) RouteRequest(backends []*Backend, req *http.Request) *Backend {
	// Check for specific routing method
	switch r.balancingMethod {
	case RoundRobinMethod:
		return r.roundRobinRoute(backends)
	case WeightedMethod:
		return r.weightedRoute(backends)
	case IPHashMethod:
		return r.ipHashRoute(backends, req)
	default:
		// Default to least connections
		return r.leastConnectionsRoute(backends)
	}
}

// leastConnectionsRoute selects backend with fewest active connections
func (r *Router) leastConnectionsRoute(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}
	
	var selectedBackend *Backend
	minConnections := -1
	
	for _, backend := range backends {
		if !backend.IsAvailable() {
			continue
		}
		
		if minConnections == -1 || backend.ActiveConnections < minConnections {
			selectedBackend = backend
			minConnections = backend.ActiveConnections
		}
	}
	
	return selectedBackend
}

// roundRobinRoute distributes requests sequentially
func (r *Router) roundRobinRoute(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}
	
	// Filter available backends
	available := make([]*Backend, 0)
	for _, b := range backends {
		if b.IsAvailable() {
			available = append(available, b)
		}
	}
	
	if len(available) == 0 {
		return nil
	}
	
	// Get next index using atomic counter
	idx := int(atomic.AddUint64(&r.rrCounter, 1) % uint64(len(available)))
	return available[idx]
}

// weightedRoute selects backend based on weight
func (r *Router) weightedRoute(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}
	
	// Filter available backends and sum weights
	available := make([]*Backend, 0)
	totalWeight := 0
	
	for _, b := range backends {
		if b.IsAvailable() && b.Weight > 0 {
			available = append(available, b)
			totalWeight += b.Weight
		}
	}
	
	if len(available) == 0 || totalWeight == 0 {
		return nil
	}
	
	// Select random value in weight range
	randomWeight := rand.Intn(totalWeight) + 1
	currentWeight := 0
	
	// Find backend that corresponds to the random weight
	for _, b := range available {
		currentWeight += b.Weight
		if randomWeight <= currentWeight {
			return b
		}
	}
	
	// Fallback (should not happen)
	return available[0]
}

// ipHashRoute routes based on client IP hash
func (r *Router) ipHashRoute(backends []*Backend, req *http.Request) *Backend {
	if len(backends) == 0 {
		return nil
	}
	
	// Filter available backends
	available := make([]*Backend, 0)
	for _, b := range backends {
		if b.IsAvailable() {
			available = append(available, b)
		}
	}
	
	if len(available) == 0 {
		return nil
	}
	
	// Get client IP
	clientIP := getClientIP(req)
	
	// Simple hash function
	var hash uint32
	for i := 0; i < len(clientIP); i++ {
		hash = hash*31 + uint32(clientIP[i])
	}
	
	// Select backend using hash
	idx := int(hash % uint32(len(available)))
	return available[idx]
}