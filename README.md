# LEBA - Load-Efficient Balancing Architecture

LEBA is a high-performance, production-ready distributed load balancer designed for modern cloud-native environments. Built with Go, it delivers enterprise-grade reliability, advanced service discovery, intelligent routing algorithms, and automatic failover capabilities across multiple protocols including HTTP/HTTPS, PostgreSQL, MySQL, and TCP-based services.

## Features

### Core Functionality
- **Multi-Protocol Support**: Native support for HTTP/HTTPS, PostgreSQL, MySQL, gRPC, SIP, and generic TCP protocols
- **Advanced Load Balancing Algorithms**: Least connections, round-robin, weighted round-robin, IP hash, and consistent hashing
- **Comprehensive Health Monitoring**: Protocol-aware health checks with customizable intervals and thresholds
- **Intelligent Connection Pooling**: Optimized database connection management with configurable pool sizes
- **Circuit Breaker Implementation**: Automatic failure detection and recovery with configurable thresholds
- **Zero-Downtime Operations**: Graceful shutdown with active connection draining and state preservation

### Service Discovery & High Availability
- **Dynamic DNS Discovery**: Real-time backend discovery with automatic registration and deregistration
- **Native Kubernetes Integration**: Seamless service discovery using Kubernetes endpoints and services
- **Distributed State Synchronization**: Peer-to-peer state replication ensuring consistency across cluster nodes
- **Automatic Failover**: Intelligent failover with configurable active-passive and active-active modes

### Intelligent Traffic Management
- **Content-Based Routing**: Advanced routing rules based on paths, headers, hosts, and custom patterns
- **Session Persistence**: Multiple sticky session methods including cookie-based, IP-based, and header-based
- **Request Coalescing**: Automatic deduplication of identical concurrent requests
- **Weighted Traffic Distribution**: Fine-grained control over traffic allocation with dynamic weight adjustment
- **Smart Caching**: Integrated caching layer with configurable TTL and invalidation strategies

### Enterprise Security Features
- **JWT Authentication**: Industry-standard JWT-based API authentication with configurable expiration
- **Advanced Rate Limiting**: Distributed rate limiting with per-client, per-endpoint, and global limits
- **Access Control Lists**: Comprehensive IP-based access control with CIDR notation support
- **Mutual TLS (mTLS)**: End-to-end encryption with client certificate validation
- **SSL/TLS Termination**: High-performance SSL termination with SNI support
- **Security Headers**: Automatic injection of security headers (HSTS, CSP, X-Frame-Options)

### Observability & Operations
- **RESTful Management API**: Comprehensive API for configuration, monitoring, and runtime management
- **Prometheus Metrics**: Native Prometheus integration with detailed performance and health metrics
- **Real-time Statistics**: Live dashboard with connection counts, latency percentiles, and error rates
- **Hot Configuration Reload**: Dynamic configuration updates without service interruption
- **Structured Logging**: JSON-formatted logs with contextual metadata for enhanced debugging
- **Health Check Dashboard**: Real-time visualization of backend health status and performance

## Getting Started

### Prerequisites
- Go 1.22 or higher
- Linux, macOS, or Windows operating system
- Basic understanding of load balancing concepts
- (Optional) TLS certificates for HTTPS endpoints
- (Optional) Prometheus for metrics collection

### Installation

#### Building from Source
```bash
git clone https://github.com/loreste/leba.git
cd leba
make build  # Builds with embedded version info
```

#### Using Pre-built Binaries
Download the appropriate binary for your platform from the [releases page](https://github.com/loreste/leba/releases).

#### Docker Installation
```bash
docker pull loreste/leba:latest
docker run -d -p 5000:5000 -p 8080:8080 -v /path/to/config.yaml:/etc/leba/config.yaml loreste/leba:latest
```

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

## Architecture Overview

LEBA employs a modular architecture designed for scalability and reliability:

### Core Components

1. **Frontend Services Layer**: Accepts incoming connections and performs protocol detection
2. **Load Balancer Engine**: Implements various load balancing algorithms with O(1) selection performance
3. **Backend Pool Manager**: Maintains backend state with lock-free data structures for high concurrency
4. **Health Monitoring System**: Performs protocol-aware health checks with configurable strategies
5. **Cluster Coordination**: Peer-to-peer state synchronization using gossip protocol
6. **Service Discovery Engine**: Integrates with DNS, Kubernetes, and custom discovery mechanisms
7. **Connection Pool Manager**: Efficient connection pooling with automatic cleanup and recycling
8. **Failover Controller**: Manages high-availability configurations and automatic failover

### Performance Optimizations

- **Lock-free Data Structures**: Minimize contention in hot paths
- **Memory Pooling**: Reduce GC pressure through object reuse
- **Zero-copy Proxying**: Efficient data transfer using splice/sendfile
- **Adaptive Algorithms**: Dynamic adjustment based on real-time metrics

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

## Production Deployment Guidelines

### Infrastructure Requirements

1. **High Availability Setup**: Deploy a minimum of 3 nodes across different availability zones
2. **Resource Allocation**: 
   - CPU: 2-4 cores per node (adjust based on traffic)
   - Memory: 4-8GB RAM (varies with connection count)
   - Network: 10Gbps network interface recommended for high-traffic deployments

### Security Best Practices

1. **Enable TLS/mTLS**: Mandatory for production environments
2. **Implement Defense in Depth**:
   - Configure IP allowlists for management APIs
   - Enable rate limiting on all endpoints
   - Use JWT tokens with short expiration times
3. **Regular Security Audits**: Review access logs and update security policies

### Performance Tuning

1. **Connection Pool Sizing**: Set based on backend capacity and expected concurrency
2. **Circuit Breaker Configuration**:
   - Failure threshold: 5-10 failures (adjust per service SLA)
   - Recovery timeout: 10-30 seconds
3. **Health Check Intervals**: Balance between detection speed and backend load
4. **Memory Optimization**: Enable memory pooling for high-traffic scenarios

### Operational Excellence

1. **Monitoring Integration**: 
   - Export metrics to Prometheus/Grafana
   - Set up alerts for error rates, latency, and availability
2. **Configuration Management**:
   - Use version control for all configuration files
   - Implement staged rollouts for configuration changes
3. **Backup Strategy**:
   - Automated daily configuration backups
   - Replicate configurations across all nodes
4. **Incident Response**:
   - Document runbooks for common scenarios
   - Practice failover procedures regularly

## Contributing

We welcome contributions from the community! Please read our [Contributing Guidelines](CONTRIBUTING.md) before submitting a pull request.

### Development Process

1. Fork the repository and create your feature branch from `main`
2. Write tests for any new functionality
3. Ensure all tests pass with `make check`
4. Update documentation as needed
5. Submit a pull request with a clear description of changes

### Code Standards

- Follow Go idioms and best practices
- Maintain test coverage above 80%
- Add benchmarks for performance-critical code
- Document all exported functions and types

## License

This project is licensed under the MIT License - see the LICENSE file for details.