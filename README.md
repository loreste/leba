# Leba
## Leba - Distributed Load Balancer
leba is a distributed TCP load balancer that supports various backend protocols such as HTTP, HTTPS, MySQL, and PostgreSQL. It is designed to be deployed across multiple servers, using gRPC for inter-node communication and configuration management. The load balancer supports least-connections load balancing and dynamic backend management.

Features
Distributed Setup: Deployable across multiple servers with DNS-based discovery.
Dynamic Backend Management: Add and remove backends on the fly.
Load Balancing Algorithms: Supports least-connections strategy.
Protocol Support: HTTP, HTTPS, MySQL, PostgreSQL.
TLS Support: Secure communication with backend servers using TLS.
Caching: Built-in query caching for database requests.
Health Checking: Regular health checks for backend servers.
