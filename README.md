# LEBA - Load-Efficient Balancing Architecture

LEBA is a high-performance, production-ready distributed load balancer designed for modern cloud-native environments. It provides advanced service discovery, intelligent routing, and failover capabilities for HTTP, HTTPS, PostgreSQL, MySQL, and other TCP-based services.

## Features

### Core Functionality
- **Multi-Protocol Support**: HTTP, HTTPS, PostgreSQL, MySQL and other TCP protocols
- **Intelligent Load Balancing**: Least connections, round-robin, weighted, and IP hash algorithms
- **Health Checking**: Customizable health monitoring for all backend types
- **Connection Pooling**: Efficient database connection management
- **Circuit Breaker Pattern**: Prevents cascading failures
- **Graceful Shutdown**: Clean termination with connection draining

### Service Discovery
- **DNS-Based Discovery**: Dynamically discover backends via DNS
- **Kubernetes Integration**: Native service discovery in Kubernetes clusters
- **Peer Synchronization**: Distributed backend state across cluster nodes

### Advanced Routing
- **Content-Based Routing**: Route based on path, headers, host patterns
- **Sticky Sessions**: Support for stateful applications
- **Active/Passive Failover**: High-availability configurations
- **Weighted Distribution**: Priority-based traffic allocation

### Security
- **Authentication**: JWT-based API access control
- **Rate Limiting**: Prevent abuse and resource exhaustion
- **IP Allowlisting**: Restrict access by client IP
- **mTLS**: Mutual TLS for secure communication

### Monitoring & Management
- **Comprehensive API**: Full REST API for management and monitoring
- **Detailed Metrics**: Performance and health statistics
- **Dynamic Reconfiguration**: Update settings without restarts
- **Structured Logging**: Enhanced visibility into operations

## Getting Started

### Prerequisites
- Go 1.22 or higher
- YAML configuration file
- (Optional) TLS certificates for HTTPS

### Installation

#### From Source
```bash
git clone https://github.com/yourusername/leba.git
cd leba
go build -o leba ./cmd/server
```

#### Using Pre-built Binary
Download the latest release from the [releases page](https://github.com/yourusername/leba/releases).

### Basic Configuration

Create a `config.yaml` file:

```yaml
frontend_address: ":5000"
api_port: "8080"
peer_port: "8081"
node_id: "node1"

allowed_ports:
  http: [80]
  https: [443]
  postgres: [5432]
  mysql: [3306]

backends:
  - address: "192.168.1.10"
    protocol: "http"
    port: 80
    weight: 1

dns_backends:
  - domain: "api.example.com"
    protocol: "http"
    port: 80
    resolution_interval: 30s

tls_config:
  enabled: true
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"

frontend_services:
  - protocol: "http"
    port: 80
  - protocol: "https"
    port: 443
```

### Running LEBA

```bash
./leba --config config.yaml
```

Additional options:
- `--log`: Specify log file path
- `--watch-config`: Enable config file watching (default: true)
- `--watch-interval`: Config watch interval in seconds (default: 30)

## Architecture

LEBA is organized into several key components:

1. **Frontend Services**: Handle incoming connections
2. **Load Balancer Core**: Implements routing algorithms
3. **Backend Pool**: Manages backend connections and state
4. **Health Checker**: Monitors backend availability
5. **Peer Manager**: Synchronizes state across LEBA cluster
6. **Discovery Manager**: Handles dynamic service discovery
7. **Connection Pool**: Manages database connections
8. **Failover Manager**: Orchestrates high-availability behavior

## Advanced Usage

### DNS Discovery

Configure backends to be discovered via DNS:

```yaml
dns_backends:
  - domain: "app.example.com"
    protocol: "http"
    port: 80
    group_name: "app-servers"
    is_primary: true
```

[Full DNS Discovery Documentation](docs/DNS-DISCOVERY.md)

### Failover Groups

Set up active/passive failover groups:

```yaml
failover_groups:
  - name: "database"
    mode: "active-passive"
    backends: ["primary-db", "replica-db"]
    primary: "primary-db"
```

[Full Failover Documentation](docs/FAILOVER.md)

### Content-Based Routing

Route traffic based on content:

```yaml
routing_rules:
  - name: "api-version"
    path_pattern: "/api/v1/*"
    target_backend: "api-v1"
  - name: "static-content"
    path_pattern: "/static/*"
    target_backend: "cdn"
```

### API Management

Secure the management API:

```yaml
api_security:
  auth_enabled: true
  jwt_secret: "${JWT_SECRET}"
  rate_limit_enabled: true
  requests_per_minute: 60
  allowlist_enabled: true
  allowed_cidrs: ["10.0.0.0/8", "192.168.1.0/24"]
```

[Full API Documentation](docs/API.md)

## Production Best Practices

1. **Run Multiple Nodes**: Deploy at least 3 nodes for high availability
2. **Use TLS**: Always enable TLS in production environments
3. **Tune Connection Limits**: Adjust based on expected traffic volume
4. **Set Up Monitoring**: Integrate with your monitoring system
5. **Regular Backups**: Back up configuration regularly
6. **Circuit Breaker Tuning**: Adjust failure thresholds for your environment
7. **Security Hardening**: Use allowlists, authentication, and rate limiting

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.