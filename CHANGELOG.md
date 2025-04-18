# Changelog

All notable changes to LEBA will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.2] - 2025-04-17

### Added
- DNS-based backend discovery for dynamic service endpoints
- Automatic failover with active/passive mode support
- Connection pooling for database backends (MySQL, PostgreSQL)
- Circuit breaker pattern for preventing cascading failures
- Graceful shutdown with connection draining
- Configuration file watching for live updates
- Enhanced health checking for all supported protocols
- Retry logic with exponential backoff
- Management API with authentication and rate limiting
- Mutual TLS (mTLS) for secure backend communication

### Changed
- Improved error handling with context propagation
- Enhanced TLS configuration with modern cipher suites
- Better logging with additional context
- More robust peer synchronization

### Fixed
- Connection leaks in database connection handling
- Race conditions in backend health status updates
- Timeout issues in health checking

## [0.0.1] - 2025-03-15

### Added
- Initial release with basic load balancing
- Support for HTTP, HTTPS, MySQL, PostgreSQL
- Simple health checking
- Basic configuration via YAML
- Least connections load balancing algorithm