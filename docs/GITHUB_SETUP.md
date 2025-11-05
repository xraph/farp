# GitHub Setup Guide

This guide explains how to configure GitHub repository settings and secrets for FARP's CI/CD pipeline.

## Repository Settings

### 1. General Settings

Navigate to **Settings** → **General**:

- **Default branch**: Set to `main`
- **Allow merge commits**: Enabled
- **Allow squash merging**: Enabled (recommended)
- **Allow rebase merging**: Enabled
- **Automatically delete head branches**: Enabled
- **Allow auto-merge**: Enabled

### 2. Branch Protection Rules

Navigate to **Settings** → **Branches** → **Add rule**:

**For `main` branch**:

- **Branch name pattern**: `main`
- **Require a pull request before merging**: ✓
  - **Require approvals**: 1 (or more for critical projects)
  - **Dismiss stale pull request approvals**: ✓
  - **Require review from Code Owners**: ✓ (if using CODEOWNERS)
- **Require status checks to pass**: ✓
  - **Require branches to be up to date**: ✓
  - **Status checks**:
    - `Go Tests (1.25)`
    - `Go Lint`
    - `Go Security Scan`
    - `Examples`
    - `Code Quality Checks`
- **Require conversation resolution**: ✓
- **Require signed commits**: ✓ (recommended)
- **Include administrators**: ✓
- **Restrict pushes**: Optional (for very strict workflows)
- **Allow force pushes**: ✗
- **Allow deletions**: ✗

**For `develop` branch** (if using):

Similar rules but potentially less strict for development.

## Required Secrets

Navigate to **Settings** → **Secrets and variables** → **Actions**:

### 1. GitHub Token (Automatic)

**Secret Name**: `GITHUB_TOKEN`  
**Description**: Automatically provided by GitHub Actions  
**Usage**: Creating releases, pushing tags, commenting on PRs  
**Action Required**: None - automatically available

### 2. Codecov Token (Optional)

**Secret Name**: `CODECOV_TOKEN`  
**Description**: Token for uploading code coverage to Codecov  
**How to Get**:
1. Sign up at [codecov.io](https://codecov.io)
2. Add your repository
3. Copy the repository token
4. Add as GitHub secret

**To Add**:
```
Settings → Secrets → New repository secret
Name: CODECOV_TOKEN
Value: [your-token-here]
```

### 3. Cargo Registry Token (For Rust - Future)

**Secret Name**: `CARGO_REGISTRY_TOKEN`  
**Description**: Token for publishing to crates.io  
**How to Get**:
1. Create account on [crates.io](https://crates.io)
2. Go to Account Settings → API Tokens
3. Create new token
4. Copy token

**To Add**:
```
Settings → Secrets → New repository secret
Name: CARGO_REGISTRY_TOKEN
Value: [your-token-here]
```

**Note**: Only needed when ready to publish Rust crate.

## Repository Permissions

### GitHub Actions Permissions

Navigate to **Settings** → **Actions** → **General**:

**Workflow permissions**:
- Select: **Read and write permissions**
- Enable: **Allow GitHub Actions to create and approve pull requests**

This allows semantic-release to:
- Create releases
- Push tags
- Update CHANGELOG.md
- Create commits

### Dependabot Permissions

Navigate to **Settings** → **Code security and analysis**:

- **Dependency graph**: Enable
- **Dependabot alerts**: Enable
- **Dependabot security updates**: Enable
- **Dependabot version updates**: Enable

### CodeQL Analysis

Navigate to **Settings** → **Code security and analysis**:

- **CodeQL analysis**: Enable
- **Default setup**: Use workflow (our custom workflow)

## Environment Variables

For different environments, navigate to **Settings** → **Environments**:

### Production Environment

Create environment named `production`:

**Deployment branches**: Selected branches → `main`

**Environment secrets** (if needed):
- `REGISTRY_URL`: Production registry URL
- `NOTIFICATION_WEBHOOK`: Slack/Discord webhook for release notifications

**Reviewers**: Add team members who must approve production deployments

## GitHub Apps (Optional)

Consider installing these GitHub Apps:

### 1. Renovate

Alternative to Dependabot with more features:
- [github.com/apps/renovate](https://github.com/apps/renovate)

### 2. SonarCloud

Additional code quality and security analysis:
- [github.com/apps/sonarcloud](https://github.com/apps/sonarcloud)

### 3. Codecov

Code coverage tracking:
- [github.com/apps/codecov](https://github.com/apps/codecov)

## Webhooks (Optional)

Navigate to **Settings** → **Webhooks** → **Add webhook**:

Useful for notifications:

**Release Notifications**:
```
Payload URL: https://hooks.slack.com/services/YOUR/WEBHOOK/URL
Content type: application/json
Events: releases
```

**CI Status**:
```
Payload URL: https://your-monitoring.com/webhook
Content type: application/json
Events: workflow_run
```

## Team Configuration

### CODEOWNERS File

Create `.github/CODEOWNERS`:

```
# Default owners for everything
*       @xraph/farp-maintainers

# Go code
*.go    @xraph/go-experts

# Rust code
/farp-rust/  @xraph/rust-experts

# CI/CD
/.github/  @xraph/devops

# Documentation
/docs/  @xraph/documentation
*.md    @xraph/documentation
```

### Issue Labels

Navigate to **Issues** → **Labels**:

Create these labels:

| Label | Color | Description |
|-------|-------|-------------|
| `bug` | #d73a4a | Something isn't working |
| `enhancement` | #a2eeef | New feature or request |
| `documentation` | #0075ca | Improvements to documentation |
| `dependencies` | #0366d6 | Dependency updates |
| `security` | #ee0701 | Security vulnerability or concern |
| `performance` | #fbca04 | Performance improvement |
| `breaking-change` | #b60205 | Breaking change |
| `good-first-issue` | #7057ff | Good for newcomers |
| `help-wanted` | #008672 | Extra attention needed |
| `wontfix` | #ffffff | This will not be worked on |

## Notifications

### Watch Settings

Recommend team members:
- Watch repository for all activity
- Custom: Releases, Security alerts

### Release Notifications

Team members should:
1. Go to repository → **Watch**
2. Select **Custom**
3. Enable **Releases**
4. Enable **Security alerts**

## Verifying Setup

### 1. Test CI Pipeline

Create a test branch and PR:

```bash
git checkout -b test/ci-setup
echo "# Testing CI" >> TEST.md
git add TEST.md
git commit -m "test: verify CI pipeline"
git push origin test/ci-setup
```

Create PR and verify all checks pass.

### 2. Test Release (Dry Run)

You can test semantic-release locally:

```bash
npm install -g semantic-release @semantic-release/changelog @semantic-release/git

# Dry run (doesn't actually release)
npx semantic-release --dry-run
```

### 3. First Real Release

After verifying setup:

```bash
git checkout main
git pull
# Make a feature commit
git commit -m "feat: initial release setup"
git push origin main
```

Watch Actions tab to see release workflow execute.

## Troubleshooting

### Release Not Creating

**Issue**: Commits to main but no release created

**Check**:
1. Verify commit messages follow conventional commits
2. Check if commits since last tag warrant a release
3. Review semantic-release logs in Actions
4. Ensure GITHUB_TOKEN has write permissions

### Permission Denied on Push

**Issue**: Release workflow can't push tags/commits

**Fix**:
```
Settings → Actions → General → Workflow permissions
Select: Read and write permissions
```

### Codecov Upload Failing

**Issue**: Coverage upload fails

**Fix**:
1. Verify `CODECOV_TOKEN` secret is set correctly
2. Check Codecov service status
3. Token might be expired - regenerate

### Dependabot PRs Not Creating

**Issue**: No automatic dependency update PRs

**Check**:
1. Verify Dependabot is enabled in settings
2. Check `.github/dependabot.yml` syntax
3. For Rust: Ensure `farp-rust/Cargo.toml` exists

## Support

For issues with this setup:
1. Check [GitHub Actions documentation](https://docs.github.com/en/actions)
2. Review workflow run logs
3. Open discussion in repository
4. Check existing issues for similar problems

---

**Last Updated**: 2025-11-01

