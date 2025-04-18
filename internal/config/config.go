package config

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

// Config represents the overall configuration structure
type Config struct {
	FrontendAddress      string             `yaml:"frontend_address"`
	APIPort              string             `yaml:"api_port"`
	PeerPort             string             `yaml:"peer_port"`
	ClusterDNS           string             `yaml:"cluster_dns"`
	NodeID               string             `yaml:"node_id"`
	FrontendDNS          string             `yaml:"frontend_dns"`
	InitialPeers         []PeerConfig       `yaml:"initial_peers"`
	Backends             []BackendConfig    `yaml:"backends"`
	DNSBackends          []DNSBackendConfig `yaml:"dns_backends"`
	DNSBackendStrings    []string           `yaml:"dns_backend_strings"` // Simple format: "domain:port/protocol"
	FailoverGroups       []FailoverGroup    `yaml:"failover_groups"`
	TLSConfig            TLSConfig          `yaml:"tls_config"`
	CacheConfig          CacheConfig        `yaml:"cache_config"`
	FrontendServices     []FrontendService  `yaml:"frontend_services"`
	AllowedPorts         map[string][]int   `yaml:"allowed_ports"` // Added for dynamic allowed ports
}

// PeerConfig represents configuration for a peer node
type PeerConfig struct {
	Address string `yaml:"address"`
	NodeID  string `yaml:"node_id"`
}

// DNSBackendConfig represents a DNS-based backend configuration
type DNSBackendConfig struct {
	Domain              string        `yaml:"domain"`
	Protocol            string        `yaml:"protocol"`
	Port                int           `yaml:"port"`
	Weight              int           `yaml:"weight"`
	IsPrimary           bool          `yaml:"is_primary"`
	GroupName           string        `yaml:"group_name"`
	ResolutionTimeout   time.Duration `yaml:"resolution_timeout"`
	ResolutionInterval  time.Duration `yaml:"resolution_interval"`
	TTL                 time.Duration `yaml:"ttl"`
}

// FailoverGroup represents a group of backends with failover capabilities
type FailoverGroup struct {
	Name      string   `yaml:"name"`
	Mode      string   `yaml:"mode"` // active-passive or normal
	Backends  []string `yaml:"backends"`
	Primary   string   `yaml:"primary"`
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

// configManager handles loading, validating, and watching configuration
type configManager struct {
	config     *Config
	configPath string
	lastMod    time.Time
	watchLock  sync.RWMutex
	listeners  []func(*Config)
}

var manager *configManager
var managerLock sync.Mutex

// GetConfigManager returns a singleton config manager
func GetConfigManager(configPath string) *configManager {
	managerLock.Lock()
	defer managerLock.Unlock()

	if manager == nil {
		manager = &configManager{
			configPath: configPath,
			listeners:  make([]func(*Config), 0),
		}
	}
	return manager
}

// LoadConfig loads configuration from a given file path
func LoadConfig(configPath string) (*Config, error) {
	configManager := GetConfigManager(configPath)
	return configManager.Load()
}

// Load loads the configuration from the file
func (cm *configManager) Load() (*Config, error) {
	config := &Config{}

	// Open the config file
	configFile, err := os.Open(cm.configPath)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()

	// Get file info for modification time
	fileInfo, err := configFile.Stat()
	if err != nil {
		return nil, err
	}
	cm.lastMod = fileInfo.ModTime()

	// Read file contents
	byteValue, err := io.ReadAll(configFile)
	if err != nil {
		return nil, err
	}

	// Apply environment variable overrides to the configuration
	byteValue = applyEnvOverrides(byteValue)

	// Unmarshal YAML
	err = yaml.Unmarshal(byteValue, config)
	if err != nil {
		return nil, err
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	cm.watchLock.Lock()
	cm.config = config
	cm.watchLock.Unlock()

	return config, nil
}

// applyEnvOverrides replaces environment variable placeholders in the config
func applyEnvOverrides(data []byte) []byte {
	config := string(data)

	// Replace ${ENV_VAR} and $ENV_VAR patterns with environment variable values
	configLines := strings.Split(config, "\n")
	for i, line := range configLines {
		if strings.Contains(line, "${") && strings.Contains(line, "}") {
			// Replace ${ENV_VAR} pattern
			for {
				start := strings.Index(line, "${")
				if start == -1 {
					break
				}
				end := strings.Index(line[start:], "}") + start
				if end > start {
					envName := line[start+2 : end]
					envValue := os.Getenv(envName)
					line = line[:start] + envValue + line[end+1:]
				} else {
					break
				}
			}
		} else if strings.Contains(line, "$") {
			// Replace $ENV_VAR pattern
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				if strings.HasPrefix(value, "$") {
					envName := value[1:]
					envValue := os.Getenv(envName)
					if envValue != "" {
						line = key + ": " + envValue
					}
				}
			}
		}
		configLines[i] = line
	}

	return []byte(strings.Join(configLines, "\n"))
}

// validateConfig performs validation checks on the loaded configuration
func validateConfig(config *Config) error {
	// Required fields
	if config.FrontendAddress == "" {
		return errors.New("frontend_address is required")
	}

	if config.NodeID == "" {
		return errors.New("node_id is required")
	}

	// Validate TLS config if enabled
	if config.TLSConfig.Enabled {
		if config.TLSConfig.CertFile == "" {
			return errors.New("cert_file is required when TLS is enabled")
		}
		if config.TLSConfig.KeyFile == "" {
			return errors.New("key_file is required when TLS is enabled")
		}

		// Check if cert and key files exist
		if _, err := os.Stat(config.TLSConfig.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("certificate file not found: %s", config.TLSConfig.CertFile)
		}
		if _, err := os.Stat(config.TLSConfig.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("key file not found: %s", config.TLSConfig.KeyFile)
		}
	}

	// Check allowed protocols
	validProtocols := map[string]bool{
		"http":     true,
		"https":    true,
		"postgres": true,
		"mysql":    true,
	}

	for proto := range config.AllowedPorts {
		if !validProtocols[proto] {
			return fmt.Errorf("invalid protocol in allowed_ports: %s", proto)
		}
	}

	// Validate frontend services
	for _, service := range config.FrontendServices {
		if !validProtocols[service.Protocol] {
			return fmt.Errorf("invalid protocol in frontend_services: %s", service.Protocol)
		}
		if service.Port < 1 || service.Port > 65535 {
			return fmt.Errorf("invalid port for service %s: %d", service.Protocol, service.Port)
		}
	}

	// Validate backends
	for i, backend := range config.Backends {
		if backend.Address == "" {
			return fmt.Errorf("backend #%d is missing address", i)
		}
		if !validProtocols[backend.Protocol] {
			return fmt.Errorf("invalid protocol for backend %s: %s", backend.Address, backend.Protocol)
		}
		if backend.Port < 1 || backend.Port > 65535 {
			return fmt.Errorf("invalid port for backend %s: %d", backend.Address, backend.Port)
		}
		if backend.Weight < 0 {
			return fmt.Errorf("invalid weight for backend %s: %d", backend.Address, backend.Weight)
		}
		if backend.Role != "" && backend.Role != "primary" && backend.Role != "replica" {
			return fmt.Errorf("invalid role for backend %s: %s (must be 'primary' or 'replica')", backend.Address, backend.Role)
		}
	}
	
	// Validate DNS backends
	for i, backend := range config.DNSBackends {
		if backend.Domain == "" {
			return fmt.Errorf("dns_backend #%d is missing domain", i)
		}
		if !validProtocols[backend.Protocol] {
			return fmt.Errorf("invalid protocol for dns_backend %s: %s", backend.Domain, backend.Protocol)
		}
		if backend.Port < 1 || backend.Port > 65535 {
			return fmt.Errorf("invalid port for dns_backend %s: %d", backend.Domain, backend.Port)
		}
		if backend.Weight < 0 {
			return fmt.Errorf("invalid weight for dns_backend %s: %d", backend.Domain, backend.Weight)
		}
		if backend.ResolutionTimeout < 0 {
			return fmt.Errorf("invalid resolution_timeout for dns_backend %s: %s", backend.Domain, backend.ResolutionTimeout)
		}
		if backend.ResolutionInterval < 0 {
			return fmt.Errorf("invalid resolution_interval for dns_backend %s: %s", backend.Domain, backend.ResolutionInterval)
		}
	}

	// Validate cache config
	if config.CacheConfig.Enabled {
		if config.CacheConfig.Expiration < 0 {
			return errors.New("cache expiration time cannot be negative")
		}
		if config.CacheConfig.CleanupInterval < 0 {
			return errors.New("cache cleanup interval cannot be negative")
		}
	}

	return nil
}

// StartConfigWatcher watches for changes to the config file and applies them
func (cm *configManager) StartConfigWatcher(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			configFile, err := os.Open(cm.configPath)
			if err != nil {
				log.Printf("Failed to open config file for watching: %v", err)
				continue
			}

			fileInfo, err := configFile.Stat()
			if err != nil {
				configFile.Close()
				log.Printf("Failed to stat config file: %v", err)
				continue
			}

			// Check if file has been modified
			if fileInfo.ModTime().After(cm.lastMod) {
				configFile.Close()
				log.Println("Config file changed, reloading...")
				
				newConfig, err := cm.Load()
				if err != nil {
					log.Printf("Failed to reload config: %v", err)
					continue
				}
				
				log.Println("Config reloaded successfully")
				
				// Notify listeners of config change
				cm.watchLock.RLock()
				for _, listener := range cm.listeners {
					go listener(newConfig)
				}
				cm.watchLock.RUnlock()
			} else {
				configFile.Close()
			}
		}
	}()
}

// AddConfigChangeListener adds a listener function to be called when config changes
func (cm *configManager) AddConfigChangeListener(listener func(*Config)) {
	cm.watchLock.Lock()
	defer cm.watchLock.Unlock()
	cm.listeners = append(cm.listeners, listener)
}

// GetConfig returns the current configuration
func (cm *configManager) GetConfig() *Config {
	cm.watchLock.RLock()
	defer cm.watchLock.RUnlock()
	return cm.config
}

// GetEnvInt gets an integer from environment or returns the default
func GetEnvInt(key string, defaultVal int) int {
	if value, exists := os.LookupEnv(key); exists {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultVal
}

// GetEnvString gets a string from environment or returns the default
func GetEnvString(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

// GetEnvBool gets a boolean from environment or returns the default
func GetEnvBool(key string, defaultVal bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return defaultVal
}

// ParseDNSBackendString parses a DNS backend string in the format "domain:port/protocol"
func ParseDNSBackendString(input string, groupName string, isPrimary bool) (*DNSBackendConfig, error) {
	// Parse the input
	parts := strings.Split(input, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid format, expected domain:port/protocol")
	}

	protocol := parts[1]
	hostPort := strings.Split(parts[0], ":")
	if len(hostPort) != 2 {
		return nil, fmt.Errorf("invalid host:port format")
	}

	domain := hostPort[0]
	port, err := strconv.Atoi(hostPort[1])
	if err != nil {
		return nil, fmt.Errorf("invalid port: %v", err)
	}

	return &DNSBackendConfig{
		Domain:              domain,
		Protocol:            protocol,
		Port:                port,
		Weight:              1, // Default weight
		IsPrimary:           isPrimary,
		GroupName:           groupName,
		ResolutionTimeout:   5 * time.Second,  // Default timeout
		ResolutionInterval:  30 * time.Second, // Default interval
		TTL:                 0,                // Use DNS TTL
	}, nil
}

// Example usage of LoadConfig function
func ExampleLoadConfig() {
	config, err := LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Config loaded: %+v", config)
}