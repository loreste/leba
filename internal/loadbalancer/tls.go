package loadbalancer

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
)

// ConfigureTLS initializes the TLS configuration based on the provided settings
func ConfigureTLS(config *TLSConfig) (*tls.Config, error) {
	if !config.Enabled {
		log.Println("TLS is disabled in the configuration.")
		return nil, nil
	}

	log.Println("Configuring TLS...")
	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate and key: %v", err)
	}

	var rootCAs *x509.CertPool
	if config.CAFile != "" {
		rootCAs = x509.NewCertPool()
		caCert, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %v", err)
		}

		if ok := rootCAs.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to append CA certificates")
		}
		log.Println("Loaded CA certificates.")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	if rootCAs != nil {
		tlsConfig.RootCAs = rootCAs
	}

	log.Println("TLS configuration successfully loaded.")
	return tlsConfig, nil
}
