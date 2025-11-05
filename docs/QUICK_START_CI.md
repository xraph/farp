# Quick Start: CI/CD Pipeline

5-minute guide to understanding FARP's automated release system.

## How It Works

```
Developer → Commit → Push → PR → Merge to main → 🎉 Auto-release!
```

### The Magic Formula

**Commit message** = **Version bump** + **Release notes**

## Commit Message Cheat Sheet

| Want to... | Use this format | Example |
|------------|-----------------|---------|
| Add new feature | `feat: description` | `feat: add PostgreSQL provider` |
| Fix a bug | `fix: description` | `fix: handle nil pointer in registry` |
| Improve performance | `perf: description` | `perf: optimize schema validation` |
| Update docs | `docs: description` | `docs: add migration guide` |
| Refactor code | `refactor: description` | `refactor: simplify provider interface` |
| Breaking change | `feat!: description` | `feat!: redesign manifest structure` |

## What Gets Released

### Automatic Version Bumping

- `fix:` → 1.0.**X** (patch)
- `feat:` → 1.**X**.0 (minor)  
- `BREAKING CHANGE:` → **X**.0.0 (major)

### Generated Artifacts

✅ Git tag (e.g., `v1.2.3`)  
✅ GitHub release with notes  
✅ Updated CHANGELOG.md  
✅ Updated version.go  
✅ Example binaries  

## Daily Workflow

### 1. Start Feature

```bash
git checkout -b feat/my-feature
```

### 2. Make Changes

```bash
# Edit files
go test ./...
golangci-lint run
```

### 3. Commit (Use Conventional Format!)

```bash
git commit -m "feat(providers): add Kafka provider

Implements Apache Kafka schema provider with support for
Schema Registry integration.

Closes #123"
```

### 4. Push and Create PR

```bash
git push origin feat/my-feature
# Create PR on GitHub
```

### 5. CI Runs Automatically

GitHub Actions will:
- Run tests on Go 1.23, 1.24, 1.25
- Run linters
- Security scan
- Build examples

### 6. After Merge → Release!

Once merged to `main`:
1. Semantic-release analyzes commits
2. Determines version bump
3. Updates version files
4. Generates CHANGELOG
5. Creates GitHub release
6. Tags repository

## Checking Status

### View CI Status

```
https://github.com/xraph/farp/actions
```

### View Releases

```
https://github.com/xraph/farp/releases
```

### Check Latest Version

```bash
# In code
git describe --tags --abbrev=0

# Go module
go list -m github.com/xraph/farp@latest

# Rust crate (future)
cargo search farp
```

## Common Scenarios

### Multiple Commits in PR

Each commit message matters:

```bash
git commit -m "feat: add feature A"
git commit -m "fix: resolve bug B"
git commit -m "docs: update readme"
```

Result after merge:
- Feature A → minor bump
- Bug fix → already covered by minor bump
- Docs → already covered by minor bump
- **Final**: Minor version bump (e.g., 1.2.0 → 1.3.0)

### Hotfix

```bash
git checkout -b hotfix/critical-bug main
git commit -m "fix: resolve critical security issue"
git push origin hotfix/critical-bug
# PR → merge → patch release (e.g., 1.2.3 → 1.2.4)
```

### Breaking Change

```bash
git commit -m "feat!: redesign provider interface

BREAKING CHANGE: Provider.Generate now requires context.Context

Migration:
- Old: provider.Generate(app)
- New: provider.Generate(ctx, app)"
```

Result: Major version bump (e.g., 1.2.3 → 2.0.0)

## Troubleshooting

### "No release created"

**Cause**: Commits don't trigger a release

**Fix**: Use conventional commit format (`feat:`, `fix:`, etc.)

### "CI failing"

**Cause**: Tests or linting errors

**Fix**: 
```bash
go test ./...
golangci-lint run
```

### "Version conflict"

**Cause**: Local version out of sync

**Fix**:
```bash
git fetch --tags
git pull origin main
```

## Best Practices

### ✅ DO

- Use meaningful commit messages
- One logical change per commit
- Test before pushing
- Keep changes focused
- Reference issues (`Closes #123`)

### ❌ DON'T

- Use vague messages ("fix stuff", "update")
- Skip conventional commit format
- Push failing tests
- Combine unrelated changes
- Forget to pull before starting

## Quick Commands

```bash
# Check your commit messages
git log --oneline

# Test locally
go test -v -race ./...

# Lint locally
golangci-lint run

# Simulate release (no actual release)
npx semantic-release --dry-run

# View what would be released
git log $(git describe --tags --abbrev=0)..HEAD --oneline
```

## Getting Help

- 📖 Full docs: [RELEASE_PROCESS.md](RELEASE_PROCESS.md)
- 🛠️ Setup guide: [GITHUB_SETUP.md](GITHUB_SETUP.md)
- 🤝 Contributing: [../CONTRIBUTING.md](../CONTRIBUTING.md)
- 💬 Questions: Open a discussion

## Pro Tips

1. **Preview release notes**: Check what commits will be released before merging
2. **Batch related features**: Group related changes in one PR for cleaner releases
3. **Use scopes**: `feat(providers):` helps organize changelog
4. **Add breaking change footer**: Always document migration path
5. **Reference issues**: Auto-close issues with `Closes #123`

---

**Remember**: Good commit messages = Good releases = Happy users! 🎉

