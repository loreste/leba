# DNS-Based Service Discovery

## Overview

LEBA's DNS-based service discovery enables automatic, real-time backend discovery through DNS resolution. This enterprise-grade feature seamlessly integrates with modern infrastructure platforms including Kubernetes, AWS Route 53, Google Cloud DNS, and traditional DNS servers, providing zero-touch backend management in dynamic environments.

## Key Features

### Automatic Discovery
- **Real-time Resolution**: Continuously discover and update backend endpoints
- **Multi-Record Support**: Handle A, AAAA, and SRV records
- **Intelligent Caching**: Respect DNS TTL with configurable overrides
- **Health-Aware Updates**: Only add healthy backends to the pool

### Advanced Capabilities
- **Graceful Updates**: Zero-downtime backend additions and removals
- **Failover Integration**: Seamless integration with active/passive failover groups
- **Load Distribution**: Automatic weight distribution based on SRV priorities
- **Resolution Strategies**: Support for round-robin, weighted, and geographic DNS
- **Error Resilience**: Automatic retry with exponential backoff

### Monitoring & Observability
- **Resolution Metrics**: Track DNS query performance and failures
- **Change Detection**: Log all backend additions and removals
- **Health Correlation**: Correlate DNS changes with backend health

## Configuration

DNS backends can be defined in two ways in the `config.yaml` file:

### 1. Structured Configuration

The `dns_backends` section allows detailed configuration:

```yaml
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
```

**Parameters:**
- `domain`: The domain name to resolve (required)
- `protocol`: Protocol to use (http, https, postgres, mysql, etc.) (required)
- `port`: Port to connect to (required)
- `weight`: Weight for load balancing (default: 1)
- `is_primary`: Whether this backend is primary in its group (default: false)
- `group_name`: Name of the failover group (optional)
- `resolution_timeout`: Timeout for DNS resolution (default: 5s)
- `resolution_interval`: Interval between DNS resolutions (default: 30s)
- `ttl`: TTL for DNS entries; overrides DNS TTL if set (default: 0, which means use DNS TTL)

### 2. Simple String Format

For simpler configuration, use the `dns_backend_strings` section:

```yaml
dns_backend_strings:
  - "api.example.com:80/http"
  - "db.example.com:5432/postgres"
  - "cache.example.com:6379/redis"
```

String format: `domain:port/protocol`

### 3. Failover Group Configuration

To configure backend groups for failover:

```yaml
failover_groups:
  - name: "app-servers"
    mode: "active-passive"
    backends: ["app1.example.com", "app2.example.com", "app3.example.com"]
    primary: "app1.example.com"
```

**Parameters:**
- `name`: Group name (required)
- `mode`: Either "active-passive" or "normal" (required)
- `backends`: List of backend IDs in the group (required)
- `primary`: Primary backend ID (required for active-passive mode)

## How It Works

1. **DNS Resolution**: The DNS Discovery Manager periodically resolves domain names to IP addresses.
2. **Backend Management**: When a new IP is discovered, it's added to the backend pool. When an IP disappears from DNS, the corresponding backend is removed.
3. **Integration with Failover**: DNS backends can be part of failover groups, allowing automatic failover if a primary backend becomes unavailable.
4. **Health Checking**: Backend health is monitored through regular health checks.

## Usage Examples

### Basic HTTP Load Balancing with DNS Discovery

```yaml
dns_backends:
  - domain: "web-service.example.com"
    protocol: "http"
    port: 80
    resolution_interval: 30s
```

### Database Failover with DNS

```yaml
dns_backends:
  - domain: "primary-db.example.com"
    protocol: "postgres"
    port: 5432
    is_primary: true
    group_name: "postgres-cluster"

  - domain: "replica-db.example.com"
    protocol: "postgres"
    port: 5432
    is_primary: false
    group_name: "postgres-cluster"
```

### Mixed Static and DNS-Based Backends

```yaml
backends:
  - address: "192.168.1.10"
    protocol: "http"
    port: 80

dns_backend_strings:
  - "api.example.com:80/http"
```

## Health Check Integration

DNS-discovered backends are automatically subjected to the same health checking mechanisms as statically defined backends. This includes:

- Protocol-specific health checks
- Circuit breaker functionality
- Connection pooling for database backends

## Advanced Configuration

### Custom Resolver Settings

In environments where you need to customize DNS resolution, you can specify:

```yaml
dns_resolver:
  timeout: 5s
  cache_ttl: 60s
  retry_attempts: 3
```

### Kubernetes Integration

For Kubernetes environments, DNS discovery works well with headless services:

```yaml
dns_backend_strings:
  - "headless-service.namespace.svc.cluster.local:80/http"
```

## Troubleshooting

### Common Issues

1. **No Backends Discovered**
   - Verify DNS records exist using `dig` or `nslookup`
   - Check resolution_timeout is sufficient
   - Ensure DNS server is accessible from the load balancer

2. **Backends Disconnect Frequently**
   - Increase resolution_interval
   - Adjust TTL settings
   - Check for network issues

3. **Slow Backend Discovery**
   - Decrease resolution_timeout
   - Check DNS server performance
   - Consider using a local DNS cache

### Diagnostic Commands

To view the current state of DNS discovery:

```bash
curl http://localhost:8080/api/v1/dns/status
```

To manually trigger DNS resolution:

```bash
curl -X POST http://localhost:8080/api/v1/dns/refresh
```

## Performance Considerations

- **Resolution Interval**: Setting too low may cause excessive DNS queries
- **Backend Pool Size**: Large number of backends may require more resources
- **TTL Settings**: Honor DNS TTL when possible to reduce unnecessary updates

## Security Best Practices

- Use DNSSEC-enabled DNS servers when possible
- Consider using private DNS zones for internal services
- Implement proper mTLS for secure communication with discovered backends
- Regularly audit DNS backends for unexpected changes

## Metrics and Monitoring

The DNS discovery system exposes the following metrics:

- DNS resolution success rate
- Resolution time
- Backend addition/removal counts
- Failover events

These can be accessed via the metrics endpoint or through the management API.