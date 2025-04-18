package config

import (
	"os"
	"path/filepath"
	"testing"
)

func createTestConfig(t *testing.T) string {
	// Create a temporary file for the test config
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")

	configData := `
frontend_address: ":5000"
api_port: "8080"
peer_port: "8081"
cluster_dns: "lb-cluster.example.com"
node_id: "test_node"
frontend_dns: "loadbalancer.example.com"

allowed_ports:
  http: [80]
  https: [443]
  postgres: [5432]
  mysql: [3306]

initial_peers:
  - address: "peer1.example.com:8081"
    node_id: "peer1"

backends:
  - address: "192.168.1.10"
    protocol: "http"
    port: 80
    weight: 1
    role: "replica"

dns_backends:
  - domain: "app.example.com"
    protocol: "http"
    port: 80
    weight: 1
    is_primary: true
    group_name: "app-servers"
    resolution_timeout: 5s
    resolution_interval: 30s
    ttl: 60s

dns_backend_strings:
  - "api.example.com:80/http"
  - "mongo.example.com:27017/mongodb"

failover_groups:
  - name: "app-servers"
    mode: "active-passive"
    backends: ["app1.example.com", "app2.example.com"]
    primary: "app1.example.com"

tls_config:
  enabled: false

cache_config:
  enabled: true
  expiration: 300
  cleanup_interval: 60

frontend_services:
  - protocol: "http"
    port: 80
`

	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	return configPath
}

func TestConfigLoading(t *testing.T) {
	// Create a test config file
	configPath := createTestConfig(t)
	defer os.Remove(configPath)

	// Load the config file
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify some values
	if cfg.FrontendAddress != ":5000" {
		t.Errorf("Expected frontend_address to be ':5000', got '%s'", cfg.FrontendAddress)
	}

	if cfg.NodeID != "test_node" {
		t.Errorf("Expected node_id to be 'test_node', got '%s'", cfg.NodeID)
	}

	if len(cfg.Backends) != 1 {
		t.Errorf("Expected 1 backend, got %d", len(cfg.Backends))
	}

	if cfg.Backends[0].Protocol != "http" {
		t.Errorf("Expected backend protocol to be 'http', got '%s'", cfg.Backends[0].Protocol)
	}

	// Verify DNS backends
	if len(cfg.DNSBackends) != 1 {
		t.Errorf("Expected 1 DNS backend, got %d", len(cfg.DNSBackends))
	}

	if cfg.DNSBackends[0].Domain != "app.example.com" {
		t.Errorf("Expected DNS backend domain to be 'app.example.com', got '%s'", cfg.DNSBackends[0].Domain)
	}

	// Verify DNS backend strings
	if len(cfg.DNSBackendStrings) != 2 {
		t.Errorf("Expected 2 DNS backend strings, got %d", len(cfg.DNSBackendStrings))
	}

	// Verify failover groups
	if len(cfg.FailoverGroups) != 1 {
		t.Errorf("Expected 1 failover group, got %d", len(cfg.FailoverGroups))
	}

	if cfg.FailoverGroups[0].Name != "app-servers" {
		t.Errorf("Expected failover group name to be 'app-servers', got '%s'", cfg.FailoverGroups[0].Name)
	}

	if cfg.TLSConfig.Enabled {
		t.Errorf("Expected TLS to be disabled")
	}

	if !cfg.CacheConfig.Enabled {
		t.Errorf("Expected cache to be enabled")
	}

	if cfg.CacheConfig.Expiration != 300 {
		t.Errorf("Expected cache expiration to be 300, got %d", cfg.CacheConfig.Expiration)
	}
}

func TestGetEnvHelpers(t *testing.T) {
	// Test GetEnvString
	os.Setenv("TEST_STRING", "test_value")
	value := GetEnvString("TEST_STRING", "default")
	if value != "test_value" {
		t.Errorf("GetEnvString failed to retrieve env var, got '%s'", value)
	}

	value = GetEnvString("NONEXISTENT_STRING", "default")
	if value != "default" {
		t.Errorf("GetEnvString didn't use default value, got '%s'", value)
	}

	// Test GetEnvInt
	os.Setenv("TEST_INT", "42")
	intValue := GetEnvInt("TEST_INT", 0)
	if intValue != 42 {
		t.Errorf("GetEnvInt failed to retrieve env var, got %d", intValue)
	}

	intValue = GetEnvInt("NONEXISTENT_INT", 123)
	if intValue != 123 {
		t.Errorf("GetEnvInt didn't use default value, got %d", intValue)
	}

	// Test GetEnvBool
	os.Setenv("TEST_BOOL", "true")
	boolValue := GetEnvBool("TEST_BOOL", false)
	if !boolValue {
		t.Errorf("GetEnvBool failed to retrieve env var")
	}

	boolValue = GetEnvBool("NONEXISTENT_BOOL", true)
	if !boolValue {
		t.Errorf("GetEnvBool didn't use default value")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	// Set environment variables for testing
	os.Setenv("TEST_PORT", "8888")
	os.Setenv("TEST_DOMAIN", "example.org")

	// Test config with environment variable placeholders
	configData := `
api_port: ${TEST_PORT}
frontend_dns: $TEST_DOMAIN
`

	// Apply env overrides
	result := string(applyEnvOverrides([]byte(configData)))

	// Check if values were replaced
	expected := `
api_port: 8888
frontend_dns: example.org
`
	if result != expected {
		t.Errorf("applyEnvOverrides didn't replace environment variables correctly: \nGot: %s\nExpected: %s", result, expected)
	}
}

func TestParseDNSBackendString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		groupName string
		isPrimary bool
		wantErr   bool
		domain    string
		protocol  string
		port      int
	}{
		{
			name:      "valid input",
			input:     "example.com:80/http",
			groupName: "test-group",
			isPrimary: true,
			wantErr:   false,
			domain:    "example.com",
			protocol:  "http",
			port:      80,
		},
		{
			name:      "invalid format",
			input:     "example.com:80",
			groupName: "",
			isPrimary: false,
			wantErr:   true,
		},
		{
			name:      "invalid port",
			input:     "example.com:invalid/http",
			groupName: "",
			isPrimary: false,
			wantErr:   true,
		},
		{
			name:      "invalid host format",
			input:     "example.com/http",
			groupName: "",
			isPrimary: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseDNSBackendString(tt.input, tt.groupName, tt.isPrimary)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDNSBackendString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if cfg.Domain != tt.domain {
				t.Errorf("ParseDNSBackendString() domain = %v, want %v", cfg.Domain, tt.domain)
			}
			if cfg.Protocol != tt.protocol {
				t.Errorf("ParseDNSBackendString() protocol = %v, want %v", cfg.Protocol, tt.protocol)
			}
			if cfg.Port != tt.port {
				t.Errorf("ParseDNSBackendString() port = %v, want %v", cfg.Port, tt.port)
			}
			if cfg.IsPrimary != tt.isPrimary {
				t.Errorf("ParseDNSBackendString() isPrimary = %v, want %v", cfg.IsPrimary, tt.isPrimary)
			}
			if cfg.GroupName != tt.groupName {
				t.Errorf("ParseDNSBackendString() groupName = %v, want %v", cfg.GroupName, tt.groupName)
			}
		})
	}
}

func TestConfigManager(t *testing.T) {
	// Create a test config file
	configPath := createTestConfig(t)
	defer os.Remove(configPath)

	// Get config manager
	manager := GetConfigManager(configPath)
	
	// Load config
	_, err := manager.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	// Test GetConfig
	cfg := manager.GetConfig()
	if cfg.NodeID != "test_node" {
		t.Errorf("GetConfig returned incorrect config, node_id: %s", cfg.NodeID)
	}
	
	// Test AddConfigChangeListener
	manager.AddConfigChangeListener(func(config *Config) {
		// Just a test listener that doesn't need to do anything
	})
	
	// Test listener is in the list
	if len(manager.listeners) != 1 {
		t.Errorf("Expected 1 listener, got %d", len(manager.listeners))
	}
}