//go:build basic
package loadbalancer

import (
	"testing"
)

func TestDNSBackendParsing(t *testing.T) {
	// Simple test to verify the ParseDNSBackend function works correctly
	result, err := ParseDNSBackend("example.com:80/http", "test-group", true)
	if err != nil {
		t.Errorf("Failed to parse DNS backend: %v", err)
	}
	
	if result.Domain != "example.com" {
		t.Errorf("Expected domain to be 'example.com', got '%s'", result.Domain)
	}
	
	if result.Port != 80 {
		t.Errorf("Expected port to be 80, got %d", result.Port)
	}
	
	if result.Protocol != "http" {
		t.Errorf("Expected protocol to be 'http', got '%s'", result.Protocol)
	}
}