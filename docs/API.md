# LEBA API Documentation

## Management API

LEBA provides a comprehensive HTTP API for configuration, monitoring, and runtime management.

### API Authentication

All API requests require JWT authentication:

```bash
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" http://localhost:8080/api/v1/status
```

### Backends Management

#### List All Backends

```
GET /api/v1/backends
```

Response:
```json
{
  "backends": [
    {
      "address": "192.168.1.10",
      "protocol": "http", 
      "port": 80,
      "health": true,
      "active_connections": 5,
      "weight": 1
    }
  ],
  "count": 1
}
```

#### Add Backend

```
POST /api/v1/backends
```

Request body:
```json
{
  "address": "192.168.1.11",
  "protocol": "http",
  "port": 80,
  "weight": 1
}
```

#### Remove Backend

```
DELETE /api/v1/backends/{backend_id}
```

#### Get Backend Health

```
GET /api/v1/backends/{backend_id}/health
```

Response:
```json
{
  "backend_id": "192.168.1.10:80",
  "health": true,
  "last_check": "2025-04-17T15:30:45Z",
  "response_time_ms": 25
}
```

### DNS Discovery Management

#### List DNS Backends

```
GET /api/v1/dns/backends
```

Response:
```json
{
  "domains": [
    {
      "domain": "app.example.com",
      "protocol": "http",
      "port": 80,
      "addresses": ["192.168.1.10", "192.168.1.11"],
      "resolution_interval": "30s",
      "last_resolution": "2025-04-17T15:30:45Z"
    }
  ],
  "count": 1
}
```

#### Add DNS Backend

```
POST /api/v1/dns/backends
```

Request body:
```json
{
  "domain": "db.example.com",
  "protocol": "postgres",
  "port": 5432,
  "is_primary": true,
  "group_name": "db-group",
  "resolution_interval": "30s"
}
```

Alternative simple format:
```json
{
  "backend_string": "api.example.com:80/http"
}
```

#### Remove DNS Backend

```
DELETE /api/v1/dns/backends/{domain}
```

#### Get DNS Status

```
GET /api/v1/dns/status
```

Response:
```json
{
  "domains": ["app.example.com", "db.example.com"],
  "domain_count": 2,
  "backend_count": 5,
  "domain_status": {
    "app.example.com": {
      "protocol": "http",
      "port": 80,
      "addresses": ["192.168.1.10", "192.168.1.11"],
      "address_count": 2,
      "last_resolution": "2025-04-17T15:30:45Z",
      "resolution_interval": "30s"
    }
  }
}
```

#### Force DNS Refresh

```
POST /api/v1/dns/refresh
```

### Failover Group Management

#### List Failover Groups

```
GET /api/v1/groups
```

Response:
```json
{
  "groups": [
    {
      "name": "app-servers",
      "mode": "active-passive",
      "primary_key": "app1.example.com",
      "primary_healthy": true,
      "total_backends": 3,
      "healthy_backends": 3,
      "last_failover": "2025-04-17T10:30:45Z"
    }
  ],
  "count": 1
}
```

#### Create Failover Group

```
POST /api/v1/groups
```

Request body:
```json
{
  "name": "db-servers",
  "mode": "active-passive"
}
```

#### Add Backend to Group

```
POST /api/v1/groups/{group_name}/backends
```

Request body:
```json
{
  "backend_id": "192.168.1.10:5432",
  "is_primary": true
}
```

#### Set Primary Backend

```
POST /api/v1/groups/{group_name}/primary
```

Request body:
```json
{
  "backend_id": "192.168.1.10:5432"
}
```

#### Remove Backend from Group

```
DELETE /api/v1/groups/{group_name}/backends/{backend_id}
```

#### Delete Group

```
DELETE /api/v1/groups/{group_name}
```

### Statistics and Monitoring

#### Get Load Balancer Status

```
GET /api/v1/status
```

Response:
```json
{
  "version": "1.0.0",
  "uptime": "24h15m5s",
  "total_backends": 10,
  "healthy_backends": 9,
  "total_connections": 1520,
  "active_connections": 42,
  "connection_rate": 15.3
}
```

#### Get Backend Statistics

```
GET /api/v1/stats/backends
```

Response:
```json
{
  "backends": {
    "192.168.1.10:80": {
      "connections": 1025,
      "active_connections": 10,
      "bytes_in": 15728640,
      "bytes_out": 1048576,
      "connection_rate": 3.5,
      "error_count": 5,
      "avg_response_time_ms": 45
    }
  }
}
```

#### Get Connection Pool Status

```
GET /api/v1/stats/pools
```

Response:
```json
{
  "pools": {
    "postgres": {
      "open_connections": 5,
      "max_open_connections": 10,
      "in_use": 2,
      "idle": 3,
      "wait_count": 0,
      "max_idle_closed": 0,
      "max_lifetime_closed": 0
    }
  }
}
```

### IP Allowlist Management

#### Get IP Allowlist

```
GET /api/v1/security/allowlist
```

Response:
```json
{
  "enabled": true,
  "rules": [
    {
      "id": "admin-ips",
      "cidr": "192.168.1.0/24",
      "description": "Admin network"
    }
  ]
}
```

#### Add IP to Allowlist

```
POST /api/v1/security/allowlist
```

Request body:
```json
{
  "cidr": "10.0.0.0/8",
  "description": "Corporate network"
}
```

#### Remove IP from Allowlist

```
DELETE /api/v1/security/allowlist/{rule_id}
```

### Configuration Management

#### Get Current Configuration

```
GET /api/v1/config
```

Response:
```json
{
  "frontend_address": ":5000",
  "api_port": "8080",
  "allowed_ports": {
    "http": [80],
    "https": [443]
  },
  "peer_port": "8081"
}
```

#### Reload Configuration

```
POST /api/v1/config/reload
```

## API Return Codes

- `200 OK` - Success
- `201 Created` - Resource successfully created
- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - IP not in allowlist
- `404 Not Found` - Resource not found
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error

## Rate Limiting

API requests are subject to rate limiting. The default limits are:

- 60 requests per minute for GET operations
- 10 requests per minute for POST/PUT/DELETE operations

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 58
X-RateLimit-Reset: 1618676545
```

## API Versioning

The API uses URL versioning (v1, v2, etc.). Always use the latest stable version.