package loadbalancer

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// TLSConfig holds the configuration for TLS
type TLSConfig struct {
	Enabled    bool   `yaml:"enabled"`
	CertFile   string `yaml:"cert_file"`
	KeyFile    string `yaml:"key_file"`
	CAFile     string `yaml:"ca_file"`
	ClientAuth string `yaml:"client_auth"`
}

// LoadTLSConfig creates a tls.Config object from the provided TLSConfig
func LoadTLSConfig(config TLSConfig) (*tls.Config, error) {
	if !config.Enabled {
		return nil, nil
	}

	// Load server certificate and key
	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate and key: %v", err)
	}

	// Create the base TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// If CAFile is provided, set up client authentication
	if config.CAFile != "" {
		caCert, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %v", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA certificates")
		}
		tlsConfig.ClientCAs = caCertPool

		// Set client auth type based on the provided configuration
		switch config.ClientAuth {
		case "NoClientCert":
			tlsConfig.ClientAuth = tls.NoClientCert
		case "RequestClientCert":
			tlsConfig.ClientAuth = tls.RequestClientCert
		case "RequireAnyClientCert":
			tlsConfig.ClientAuth = tls.RequireAnyClientCert
		case "VerifyClientCertIfGiven":
			tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
		case "RequireAndVerifyClientCert":
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		default:
			return nil, fmt.Errorf("invalid client auth type: %s", config.ClientAuth)
		}
	}

	return tlsConfig, nil
}
