package config

import (
	"os"
	"sync"

	"gopkg.in/yaml.v2"
)

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
	mu              sync.Mutex
	filePath        string
}

type PeerConfig struct {
	Address string `yaml:"address"`
	NodeID  string `yaml:"node_id"`
}

type BackendConfig struct {
	Address            string `yaml:"address"`
	Protocol           string `yaml:"protocol"`
	Port               int    `yaml:"port"`
	Weight             int    `yaml:"weight"`
	MaxOpenConnections int    `yaml:"max_open_connections"`
	MaxIdleConnections int    `yaml:"max_idle_connections"`
	ConnMaxLifetime    int    `yaml:"conn_max_lifetime"` // in seconds
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	HTTP2    bool   `yaml:"http2"`
	HTTP3    bool   `yaml:"http3"`
}

type CacheConfig struct {
	Enabled    bool `yaml:"enabled"`
	CacheSize  int  `yaml:"cache_size"`
	Expiration int  `yaml:"expiration"` // in seconds
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	config.filePath = path
	return &config, nil
}

func (c *Config) SaveConfig() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.filePath, data, 0644)
}
