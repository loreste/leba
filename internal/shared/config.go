package shared

import (
	"io"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config represents the overall configuration structure
type Config struct {
	FrontendAddress  string            `yaml:"frontend_address"`
	APIPort          string            `yaml:"api_port"`
	PeerPort         string            `yaml:"peer_port"`
	ClusterDNS       string            `yaml:"cluster_dns"`
	NodeID           string            `yaml:"node_id"`
	FrontendDNS      string            `yaml:"frontend_dns"`
	InitialPeers     []PeerConfig      `yaml:"initial_peers"` // Updated to use locally defined PeerConfig
	Backends         []BackendConfig   `yaml:"backends"`
	TLSConfig        TLSConfig         `yaml:"tls_config"`
	CacheConfig      CacheConfig       `yaml:"cache_config"`
	FrontendServices []FrontendService `yaml:"frontend_services"`
	AllowedPorts     map[string][]int  `yaml:"allowed_ports"`
	PeerSyncInterval time.Duration     `yaml:"peer_sync_interval"` // For peer synchronization interval
	LogFilePath      string            `yaml:"log_file_path"`      // For log file path
	SIPLogFilePath   string            `yaml:"sip_log_file_path"`  // For SIP log file path
}

// BackendConfig represents configuration for a backend server
type BackendConfig struct {
	Address            string `yaml:"address"`
	Protocol           string `yaml:"protocol"`
	Port               int    `yaml:"port"`
	Weight             int    `yaml:"weight"`
	MaxOpenConnections int    `yaml:"max_open_connections"`
	MaxIdleConnections int    `yaml:"max_idle_connections"`
	ConnMaxLifetime    int    `yaml:"conn_max_lifetime"`
	Role               string `yaml:"role"` // primary or replica
}

// TLSConfig holds TLS configuration details
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	HTTP2    bool   `yaml:"http2"`
	HTTP3    bool   `yaml:"http3"`
}

// CacheConfig holds configuration settings for caching
type CacheConfig struct {
	Enabled         bool `yaml:"enabled"`
	Expiration      int  `yaml:"expiration"`       // Expiration time for cached items in seconds
	CleanupInterval int  `yaml:"cleanup_interval"` // Interval for cleaning up expired items in seconds
}

// FrontendService represents a service configuration for the load balancer
type FrontendService struct {
	Protocol string `yaml:"protocol"`
	Port     int    `yaml:"port"`
}

// LoadConfig loads configuration from a given file path
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()

	byteValue, err := io.ReadAll(configFile)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(byteValue, config)
	if err != nil {
		return nil, err
	}

	// Set default values for PeerSyncInterval if not provided
	if config.PeerSyncInterval == 0 {
		config.PeerSyncInterval = 5 * time.Second
	}

	// Set default log file paths if not provided
	if config.LogFilePath == "" {
		config.LogFilePath = "/var/log/loadbalancer.log"
	}
	if config.SIPLogFilePath == "" {
		config.SIPLogFilePath = "/var/log/sip_calls.log"
	}

	return config, nil
}

// Example usage of LoadConfig function
func ExampleLoadConfig() {
	config, err := LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Config loaded: %+v", config)
}
