//go:build !integration
package loadbalancer

import (
	"testing"
)

func TestFailoverManager(t *testing.T) {
	// Create backend pool
	pool := NewBackendPool()

	// Create failover manager
	fm := NewFailoverManager(pool)

	// Create backend group
	group := fm.CreateBackendGroup("test-group", ActivePassiveMode)
	if group == nil {
		t.Fatal("Failed to create backend group")
	}

	// Check if group exists
	group = fm.GetBackendGroup("test-group")
	if group == nil {
		t.Fatal("Failed to retrieve backend group")
	}

	// Add backends to pool
	backend1 := &Backend{
		Address:  "192.168.1.1",
		Protocol: "http",
		Port:     80,
		Health:   true,
	}
	pool.AddBackend(backend1)

	backend2 := &Backend{
		Address:  "192.168.1.2",
		Protocol: "http",
		Port:     80,
		Health:   true,
	}
	pool.AddBackend(backend2)

	// Add backends to group
	fm.AddBackendToGroup("test-group", "192.168.1.1", true)  // Primary
	fm.AddBackendToGroup("test-group", "192.168.1.2", false) // Replica

	// Test getting primary backend
	result := fm.GetBackendForGroup("test-group")
	if result == nil {
		t.Fatal("Failed to get backend for group")
	}
	if result.Address != "192.168.1.1" {
		t.Errorf("Expected primary backend, got %s", result.Address)
	}

	// Test failover when primary is unhealthy
	backend1.Health = false
	result = fm.GetBackendForGroup("test-group")
	if result == nil {
		t.Fatal("Failed to get failover backend")
	}
	if result.Address != "192.168.1.2" {
		t.Errorf("Expected failover to replica, got %s", result.Address)
	}

	// Test removing backend from group
	fm.RemoveBackendFromGroup("test-group", "192.168.1.2")
	
	// Should have no available backends now
	result = fm.GetBackendForGroup("test-group")
	if result != nil {
		t.Errorf("Expected nil result after removing all healthy backends, got %s", result.Address)
	}

	// Test setting group mode
	fm.SetGroupMode("test-group", NormalMode)
	group = fm.GetBackendGroup("test-group")
	if group.Mode != NormalMode {
		t.Errorf("Expected NormalMode, got %v", group.Mode)
	}
}