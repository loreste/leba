# Contributing to LEBA

Thank you for your interest in contributing to LEBA! This document provides guidelines and instructions for contributing to the project. We value all contributions, from bug reports to feature implementations.

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct, which promotes a respectful and inclusive environment for all contributors.

## How to Contribute

### Reporting Issues

- **Bug Reports**: Include detailed steps to reproduce, expected vs. actual behavior, and system information
- **Feature Requests**: Describe the use case, proposed solution, and potential alternatives
- **Security Issues**: Please report security vulnerabilities privately to security@leba-project.org

### Development Workflow

We follow a standard GitHub flow for all contributions:

### Pull Requests

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Commit Message Guidelines

We follow the Conventional Commits specification for clear and consistent commit history:

#### Format
```
<type>(<scope>): <subject>

<body>

<footer>
```

#### Types
- **feat**: New feature implementation
- **fix**: Bug fix
- **docs**: Documentation changes
- **style**: Code style changes (formatting, missing semicolons, etc.)
- **refactor**: Code restructuring without changing functionality
- **perf**: Performance improvements
- **test**: Test additions or modifications
- **chore**: Build process, dependencies, or tooling changes

#### Examples
```
feat(backend): add circuit breaker implementation

Implement circuit breaker pattern for backend connections to prevent
cascading failures. The circuit breaker has three states: closed,
open, and half-open.

Closes #123
```

```
fix(health): correct race condition in health checker

Use atomic operations instead of mutex locks for performance-critical
health status updates.
```

## Code Quality Standards

### Go Code Guidelines

- **Formatting**: Use `gofmt` and `goimports` for consistent formatting
- **Linting**: Pass `golangci-lint` checks (run `make lint`)
- **Testing**: Maintain test coverage above 80% for new code
- **Documentation**: Document all exported functions, types, and packages
- **Error Handling**: Always handle errors explicitly, wrap with context when appropriate

### Testing Requirements

- **Unit Tests**: Write comprehensive unit tests for all new functionality
- **Integration Tests**: Add integration tests for API endpoints and complex workflows
- **Benchmarks**: Include benchmarks for performance-critical code paths
- **Race Detection**: Ensure all tests pass with `-race` flag

### Performance Considerations

- **Memory Management**: Use object pools for frequently allocated objects
- **Concurrency**: Prefer atomic operations over mutex locks where possible
- **Profiling**: Profile code for CPU and memory usage in performance-critical sections

### Security Guidelines

- **Input Validation**: Validate all external inputs
- **Error Messages**: Don't expose sensitive information in error messages
- **Dependencies**: Keep dependencies minimal and up-to-date
- **Secrets**: Never commit secrets or credentials

## License

By contributing, you agree that your contributions will be licensed under the project's MIT License.

## Code of Conduct

This project follows a Code of Conduct to ensure a welcoming community for all. Please read and follow it.

## References

This document was adapted from the open-source contribution guidelines templates.