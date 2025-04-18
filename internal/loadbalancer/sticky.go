package loadbalancer

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// StickySessionManager handles sticky session routing
type StickySessionManager struct {
	// Map of session ID to backend address
	sessions      map[string]string
	// Map of client IP to backend address
	ipSessions    map[string]string
	// Expiration time for session mappings
	expiry        time.Duration
	// Map of session creation time for cleaning
	sessionTimes  map[string]time.Time
	mu            sync.RWMutex
	// Cookie name for sticky sessions
	cookieName    string
	// Whether to use cookies or client IP
	useCookies    bool
	// Cleaner interval
	cleanInterval time.Duration
	// Shutdown signal
	done          chan struct{}
}

// NewStickySessionManager creates a new sticky session manager
func NewStickySessionManager(useCookies bool, expiry time.Duration) *StickySessionManager {
	ssm := &StickySessionManager{
		sessions:      make(map[string]string),
		ipSessions:    make(map[string]string),
		sessionTimes:  make(map[string]time.Time),
		expiry:        expiry,
		cookieName:    "LEBA_STICKY",
		useCookies:    useCookies,
		cleanInterval: 5 * time.Minute,
		done:          make(chan struct{}),
	}

	// Start cleaner routine
	go ssm.cleanExpiredSessions()

	return ssm
}

// GetBackendForRequest returns the sticky backend for a request
func (ssm *StickySessionManager) GetBackendForRequest(r *http.Request) (string, bool) {
	if ssm.useCookies {
		// Try cookie-based stickiness
		cookie, err := r.Cookie(ssm.cookieName)
		if err == nil && cookie.Value != "" {
			ssm.mu.RLock()
			backend, exists := ssm.sessions[cookie.Value]
			ssm.mu.RUnlock()
			
			if exists {
				// Update timestamp to extend expiry
				ssm.mu.Lock()
				ssm.sessionTimes[cookie.Value] = time.Now()
				ssm.mu.Unlock()
				
				return backend, true
			}
		}
	} else {
		// Use client IP for stickiness
		clientIP := getClientIP(r)
		
		ssm.mu.RLock()
		backend, exists := ssm.ipSessions[clientIP]
		ssm.mu.RUnlock()
		
		if exists {
			// Create a session ID for IP-based tracking
			sessionID := generateSessionID(clientIP)
			
			// Update timestamp to extend expiry
			ssm.mu.Lock()
			ssm.sessionTimes[sessionID] = time.Now()
			ssm.mu.Unlock()
			
			return backend, true
		}
	}
	
	return "", false
}

// SetBackendForRequest sets the backend for a request
func (ssm *StickySessionManager) SetBackendForRequest(w http.ResponseWriter, r *http.Request, backendAddr string) {
	clientIP := getClientIP(r)
	sessionID := generateSessionID(clientIP)
	
	ssm.mu.Lock()
	defer ssm.mu.Unlock()
	
	if ssm.useCookies {
		// Set cookie for stickiness
		cookie := &http.Cookie{
			Name:     ssm.cookieName,
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil, // Set secure flag if request is HTTPS
			MaxAge:   int(ssm.expiry.Seconds()),
			SameSite: http.SameSiteStrictMode,
		}
		http.SetCookie(w, cookie)
		
		// Store mapping
		ssm.sessions[sessionID] = backendAddr
	} else {
		// Just store IP mapping
		ssm.ipSessions[clientIP] = backendAddr
	}
	
	// Store time for expiry
	ssm.sessionTimes[sessionID] = time.Now()
	
	log.Printf("Created sticky session %s -> %s", sessionID, backendAddr)
}

// RemoveBackend removes all sessions associated with a backend
func (ssm *StickySessionManager) RemoveBackend(backendAddr string) {
	ssm.mu.Lock()
	defer ssm.mu.Unlock()
	
	// Remove from cookie-based sessions
	for id, addr := range ssm.sessions {
		if addr == backendAddr {
			delete(ssm.sessions, id)
			delete(ssm.sessionTimes, id)
		}
	}
	
	// Remove from IP-based sessions
	for ip, addr := range ssm.ipSessions {
		if addr == backendAddr {
			delete(ssm.ipSessions, ip)
			// Also need to clean up the generated session ID
			sessionID := generateSessionID(ip)
			delete(ssm.sessionTimes, sessionID)
		}
	}
}

// cleanExpiredSessions periodically removes expired sessions
func (ssm *StickySessionManager) cleanExpiredSessions() {
	ticker := time.NewTicker(ssm.cleanInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ssm.cleanSessions()
		case <-ssm.done:
			return
		}
	}
}

// cleanSessions removes expired sessions
func (ssm *StickySessionManager) cleanSessions() {
	now := time.Now()
	expiredIDs := make([]string, 0)
	expiredIPs := make([]string, 0)
	
	// Find expired sessions
	ssm.mu.RLock()
	for id, timestamp := range ssm.sessionTimes {
		if now.Sub(timestamp) > ssm.expiry {
			expiredIDs = append(expiredIDs, id)
		}
	}
	ssm.mu.RUnlock()
	
	// Find expired IP mappings by checking the hash
	ssm.mu.RLock()
	for ip := range ssm.ipSessions {
		sessionID := generateSessionID(ip)
		if timestamp, exists := ssm.sessionTimes[sessionID]; exists {
			if now.Sub(timestamp) > ssm.expiry {
				expiredIPs = append(expiredIPs, ip)
			}
		} else {
			// No timestamp, consider expired
			expiredIPs = append(expiredIPs, ip)
		}
	}
	ssm.mu.RUnlock()
	
	// Remove expired sessions
	if len(expiredIDs) > 0 || len(expiredIPs) > 0 {
		ssm.mu.Lock()
		for _, id := range expiredIDs {
			delete(ssm.sessions, id)
			delete(ssm.sessionTimes, id)
		}
		for _, ip := range expiredIPs {
			delete(ssm.ipSessions, ip)
		}
		ssm.mu.Unlock()
		
		log.Printf("Cleaned %d expired sessions", len(expiredIDs)+len(expiredIPs))
	}
}

// Stop stops the session manager
func (ssm *StickySessionManager) Stop() {
	close(ssm.done)
}

// GetStats returns statistics about the sticky session manager
func (ssm *StickySessionManager) GetStats() map[string]interface{} {
	ssm.mu.RLock()
	defer ssm.mu.RUnlock()
	
	return map[string]interface{}{
		"cookie_sessions": len(ssm.sessions),
		"ip_sessions":     len(ssm.ipSessions),
		"use_cookies":     ssm.useCookies,
		"expiry_seconds":  ssm.expiry.Seconds(),
	}
}

// Helper functions

// getClientIP gets the client IP from a request
func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	
	// Try X-Real-IP header
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// generateSessionID creates a session ID from client info
func generateSessionID(clientIP string) string {
	// Add timestamp to make it unique even for same client
	data := clientIP + strconv.FormatInt(time.Now().UnixNano(), 10)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}