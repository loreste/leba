# Release Process

This document describes the release process for LEBA.

## Versioning

LEBA follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version when you make incompatible API changes
- **MINOR** version when you add functionality in a backward compatible manner
- **PATCH** version when you make backward compatible bug fixes

## Release Checklist

1. **Update Version**
   - Update the version number in `internal/version/version.go`
   - Update CHANGELOG.md with the new version and release date

2. **Run Tests**
   ```bash
   go test -tags basic ./internal/...
   ```

3. **Build and Test Locally**
   ```bash
   go build -o leba ./cmd/server
   ./leba --version
   ```

4. **Create a Release Branch**
   ```bash
   git checkout -b release/v{version}
   git add .
   git commit -m "Release v{version}"
   git push origin release/v{version}
   ```

5. **Create a Pull Request**
   - Create a PR from the release branch to main
   - Ensure CI passes
   - Get approval from at least one maintainer

6. **Tag the Release**
   ```bash
   git tag -a v{version} -m "Release v{version}"
   git push origin v{version}
   ```

7. **Build Release Binaries**
   The GitHub Actions workflow will automatically build and create the release when a tag is pushed.

## Post-Release Tasks

1. Update version in `internal/version/version.go` to the next development version (e.g., "0.1.0-dev")

2. Announce the release in:
   - GitHub discussions
   - Project website/blog (if applicable)
   - Relevant community channels

## Emergency Fixes

For critical bug fixes that need to be released outside the normal cycle:

1. Create a hotfix branch from the latest release tag
   ```bash
   git checkout -b hotfix/issue-{number} v{latest-version}
   ```

2. Fix the issue, commit, and push
   ```bash
   git add .
   git commit -m "Fix critical issue #{number}"
   git push origin hotfix/issue-{number}
   ```

3. Follow steps 5-7 from the regular release process, but increment only the patch version