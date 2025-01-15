package config

import (
	"io"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

// Config represents the overall configuration structure
type Config struct {
	FrontendAddress string          `yaml:"frontend_address"`
	APIPort         string          `yaml:"api_port"`
	PeerPort        string          `yaml:"peer_port"`
	ClusterDNS      string          `yaml:"cluster_dns"`
	NodeID          string          `yaml:"node_id"`
	FrontendDNS     string          `yaml:"frontend_dns"`
	InitialPeers    []PeerConfig    `yaml:"initial_peers"`
	Backends        []BackendConfig `yaml:"backends"`
	TLSConfig       TLSConfig       `yaml:"tls_config"`
	CacheConfig     CacheConfig     `yaml:"cache_config"`
}

// PeerConfig represents configuration for a peer node
type PeerConfig struct {
	Address string `yaml:"address"`
	NodeID  string `yaml:"node_id"`
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
	Role               string `yaml:"role"` // Added this field
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
