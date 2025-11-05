# Release Process

FARP uses automated semantic versioning based on [Conventional Commits](https://www.conventionalcommits.org/).

## Overview

Releases are fully automated through GitHub Actions:

1. Developer merges PR to `main` with conventional commit messages
2. CI analyzes commit history since last release
3. Version is automatically bumped based on commit types
4. CHANGELOG is generated
5. Git tag is created
6. GitHub release is published
7. Go module version is tagged
8. Rust crate is published to crates.io (when available)

## Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Version Bumping Rules

| Commit Type | Version Bump | Example |
|-------------|--------------|---------|
| `fix:` | Patch (1.0.x) | `fix(registry): handle nil pointer` |
| `feat:` | Minor (1.x.0) | `feat(providers): add Kafka support` |
| `BREAKING CHANGE:` | Major (x.0.0) | `feat!: redesign provider interface` |
| `perf:` | Patch (1.0.x) | `perf(manifest): optimize validation` |
| `docs:` | Patch (1.0.x) | `docs(readme): update examples` |
| `chore:`, `ci:`, `test:` | No release | `chore: update dependencies` |

### Commit Types

- **feat**: New feature (minor bump)
- **fix**: Bug fix (patch bump)
- **perf**: Performance improvement (patch bump)
- **docs**: Documentation changes (patch bump)
- **refactor**: Code refactoring (patch bump)
- **test**: Test updates (no release)
- **build**: Build system changes (no release)
- **ci**: CI configuration changes (no release)
- **chore**: Other maintenance (no release)

### Breaking Changes

For breaking changes, use either:

1. Add `!` after type:
   ```
   feat!: change provider interface signature
   
   BREAKING CHANGE: Provider.Generate now requires context.Context
   ```

2. Add `BREAKING CHANGE:` in footer:
   ```
   feat(api): redesign schema manifest
   
   BREAKING CHANGE: SchemaManifest field types have changed
   Migration guide: ...
   ```

## Release Workflow

### Standard Release

1. Create a branch for your feature/fix:
   ```bash
   git checkout -b feat/my-feature
   ```

2. Make your changes and commit using conventional commits:
   ```bash
   git commit -m "feat(providers): add PostgreSQL provider"
   ```

3. Push and create pull request:
   ```bash
   git push origin feat/my-feature
   ```

4. After PR approval and merge to `main`:
   - CI automatically runs tests
   - Semantic-release analyzes commits
   - New version is determined
   - Release is published automatically

### Pre-release / Beta

For pre-releases, create branches:

- `alpha` - for alpha releases (v1.2.3-alpha.1)
- `beta` - for beta releases (v1.2.3-beta.1)
- `rc` - for release candidates (v1.2.3-rc.1)

Configure in `.releaserc.json` branches section.

### Hotfix Release

For urgent fixes to production:

1. Create hotfix branch from latest release tag:
   ```bash
   git checkout -b hotfix/critical-bug v1.2.3
   ```

2. Make fix with conventional commit:
   ```bash
   git commit -m "fix: resolve critical security issue"
   ```

3. Merge to `main` and release will be triggered

## What Gets Released

### Go Library

- Git tag (e.g., `v1.2.3`)
- GitHub release with notes
- Updated `version.go`
- Updated `CHANGELOG.md`
- Example binaries

### Rust Library (Future)

- Git tag (e.g., `v1.2.3`)
- Published to crates.io
- Updated `Cargo.toml`
- Compiled library artifacts

## Release Assets

Each release includes:

- **Source code** (zip and tar.gz)
- **Example binary** (`farp-basic-example`)
- **CHANGELOG** (embedded in release notes)
- **Documentation** links

## Version Constraints

### Go Module

```go
// Require specific version
require github.com/xraph/farp v1.2.3

// Require minimum version
require github.com/xraph/farp v1.2.0

// Latest minor version
require github.com/xraph/farp v1
```

### Rust Crate

```toml
[dependencies]
# Exact version
farp = "1.2.3"

# Compatible versions (>= 1.2.3, < 2.0.0)
farp = "^1.2.3"

# Minimum version
farp = ">= 1.2.3"
```

## Manual Release (Emergency)

If automated release fails:

1. **Update version manually:**
   ```bash
   ./scripts/update-version.sh 1.2.4
   ```

2. **Update CHANGELOG.md** with changes

3. **Commit and tag:**
   ```bash
   git add version.go CHANGELOG.md
   git commit -m "chore(release): 1.2.4"
   git tag -a v1.2.4 -m "Release v1.2.4"
   git push origin main --tags
   ```

4. **Create GitHub release** manually with notes

## Troubleshooting

### Release Not Triggering

- Verify commits follow conventional format
- Check CI logs in GitHub Actions
- Ensure semantic-release configuration is correct
- Confirm GITHUB_TOKEN has sufficient permissions

### Version Conflicts

- Ensure no duplicate tags exist
- Verify `version.go` is in sync with latest tag
- Check for merge conflicts in CHANGELOG.md

### Rust Release Failing

- Verify `CARGO_REGISTRY_TOKEN` secret is set
- Ensure Cargo.toml version matches tag
- Check crates.io for existing version conflicts

## Monitoring Releases

- **GitHub Releases**: https://github.com/xraph/farp/releases
- **Go Module Proxy**: https://proxy.golang.org/github.com/xraph/farp/@v/list
- **Crates.io** (future): https://crates.io/crates/farp

## Security Releases

For security vulnerabilities:

1. **Do not** create public issue or PR
2. Report to security team
3. Fix will be released as patch with advisory
4. Follow responsible disclosure timeline

## Best Practices

1. **Write meaningful commit messages** - they become CHANGELOG
2. **Group related changes** - one logical change per commit
3. **Test before merging** - CI must pass
4. **Review release notes** - verify generated CHANGELOG makes sense
5. **Document breaking changes** - provide migration guide
6. **Maintain backward compatibility** - minimize breaking changes

## CI/CD Configuration Files

- `.github/workflows/ci.yml` - Tests and quality checks
- `.github/workflows/release.yml` - Release automation
- `.releaserc.json` - Semantic-release configuration
- `scripts/update-version.sh` - Version update script

## Release Schedule

- **Patch releases**: As needed for bug fixes
- **Minor releases**: Monthly feature releases
- **Major releases**: Yearly or for breaking changes
- **Security patches**: Immediate as needed

---

For questions about releases, see [CONTRIBUTING.md](../CONTRIBUTING.md) or open a discussion.

