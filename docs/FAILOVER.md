# High Availability & Failover Configuration

## Overview

LEBA's enterprise-grade failover system delivers mission-critical high availability through intelligent traffic management, automatic failure detection, and seamless failover orchestration. Designed for zero-downtime operations, it ensures continuous service availability even during backend failures, maintenance windows, or infrastructure events.

## Architecture & Capabilities

### Failover Strategies

#### Active-Passive Mode
- **Primary-First Routing**: All traffic directed to primary backend during normal operations
- **Automatic Failover**: Instant traffic redirection upon primary failure detection
- **Failback Options**: Configurable automatic or manual failback to primary
- **State Preservation**: Maintain session state during failover events

#### Active-Active Mode
- **Load Distribution**: Traffic distributed across all healthy backends
- **Dynamic Rebalancing**: Automatic traffic redistribution on backend changes
- **Weighted Routing**: Fine-grained traffic control based on backend capacity
- **Geographic Affinity**: Route based on client proximity to backends

### Advanced Features

#### Intelligent Health Monitoring
- **Multi-Layer Health Checks**: Application, network, and protocol-specific checks
- **Predictive Failure Detection**: Identify degrading backends before complete failure
- **Custom Health Criteria**: Define complex health evaluation logic
- **Health Score Tracking**: Continuous backend performance scoring

#### Failover Orchestration
- **Sub-Second Detection**: Rapid failure identification and response
- **Graceful Degradation**: Progressive traffic reduction for failing backends
- **Connection Draining**: Safe migration of active connections
- **Rollback Protection**: Prevent flapping between unstable backends

#### Operational Control
- **Manual Override**: Force failover for maintenance or testing
- **Scheduled Maintenance**: Plan failover windows with automatic execution
- **API-Driven Control**: Full programmatic failover management
- **Audit Logging**: Complete failover event history and analysis

## Configuration

### Defining Failover Groups

Failover groups can be defined in the `config.yaml` file:

```yaml
failover_groups:
  - name: "app-servers"
    mode: "active-passive"
    backends: ["backend1", "backend2", "backend3"]
    primary: "backend1"

  - name: "db-servers"
    mode: "normal"
    backends: ["db1", "db2"]
    primary: "db1"
```

**Parameters:**
- `name`: Unique identifier for the group (required)
- `mode`: Either "active-passive" or "normal" (required)
- `backends`: List of backend identifiers (required)
- `primary`: Primary backend identifier (required for active-passive mode)

### Integrating with DNS Discovery

To use failover with DNS-discovered backends:

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

## How It Works

1. **Group Creation**: Failover groups are created from configuration or via API
2. **Health Monitoring**: The health checker continuously monitors all backends
3. **State Management**: Group state is maintained with primary and backup information
4. **Failover Process**:
   - When a primary backend fails, the system selects a healthy backup
   - Traffic is automatically directed to the new primary
   - The original primary is demoted to backup status
   - When the original primary recovers, it remains as backup by default

## Failover Strategies

### Active/Passive Mode

In active/passive mode, traffic is always directed to the primary backend as long as it's healthy. If the primary fails, traffic switches to a backup backend. This mode is recommended for:

- Stateful applications
- Database servers
- Services requiring strict consistency

### Normal Mode

In normal mode, traffic is distributed across all healthy backends using the configured load balancing algorithm. This mode offers better resource utilization and is recommended for:

- Stateless web applications
- Content delivery
- API services

## API Integration

The failover system can be managed through the LEBA API:

- `GET /api/v1/groups` - List all failover groups
- `POST /api/v1/groups` - Create a new failover group
- `GET /api/v1/groups/{name}` - Get group details
- `POST /api/v1/groups/{name}/primary` - Set a new primary
- `DELETE /api/v1/groups/{name}` - Delete a failover group

See the [API Documentation](API.md) for complete details.

## Best Practices

### Group Design

- **Define Clear Group Boundaries**: Group related backends that serve the same function
- **Balance Group Size**: 3-5 backends per group is typically optimal
- **Use Meaningful Names**: Name groups by function (e.g., "postgres-primary", "web-frontend")

### Monitoring and Alerts

- Set up alerts for failover events
- Monitor primary changes
- Track time spent in failover state

### Testing

- Regularly test failover by manually taking down primary backends
- Validate recovery procedures
- Measure failover time

## Advanced Configurations

### Custom Health Checks

Configure custom health checks for failover groups:

```yaml
failover_groups:
  - name: "custom-check-group"
    mode: "active-passive"
    backends: ["backend1", "backend2"]
    primary: "backend1"
    health_check:
      path: "/custom-health"
      interval: "5s"
      timeout: "2s"
      healthy_threshold: 3
      unhealthy_threshold: 2
```

### Sticky Sessions with Failover

When using sticky sessions with failover groups, session data may be lost during failover events. Options to handle this:

1. **Session Replication**: Ensure sessions are replicated across backends
2. **External Session Store**: Use Redis or similar for centralized session storage
3. **Graceful Degradation**: Design applications to handle session loss gracefully

### Multiple Failover Groups

Complex applications may require multiple failover groups:

```yaml
failover_groups:
  - name: "web-tier"
    mode: "active-passive"
    backends: ["web1", "web2", "web3"]
    primary: "web1"

  - name: "app-tier"
    mode: "active-passive"
    backends: ["app1", "app2"]
    primary: "app1"

  - name: "db-tier"
    mode: "active-passive"
    backends: ["db1", "db2"]
    primary: "db1"
```

## Troubleshooting

### Common Issues

1. **Frequent Failovers**
   - Check network stability
   - Increase health check thresholds
   - Verify backend resources are sufficient

2. **Failed Primary Election**
   - Check if any backends are healthy
   - Verify group configuration
   - Check for network partitions

3. **Traffic Not Switching**
   - Verify health check configuration
   - Check client caching (DNS TTL, etc.)
   - Inspect backend connectivity

### Diagnostic Commands

Get failover group status:
```bash
curl http://localhost:8080/api/v1/groups/app-servers
```

Force failover to a specific backend:
```bash
curl -X POST -H "Content-Type: application/json" \
  -d '{"backend_id": "backend2"}' \
  http://localhost:8080/api/v1/groups/app-servers/primary
```

## Metrics and Monitoring

The failover system exposes the following metrics:

- Number of failover events
- Time since last failover
- Current primary backend for each group
- Healthy backend count per group
- Failed backend count per group

These metrics are available through the metrics endpoint and can be integrated with Prometheus, Grafana, or other monitoring systems.