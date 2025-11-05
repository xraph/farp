# Contributing to FARP

Thank you for your interest in contributing to FARP! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, professional, and inclusive. We're all here to build better software.

## Development Setup

### Go Development

```bash
# Clone the repository
git clone https://github.com/xraph/farp.git
cd farp

# Install dependencies
go mod download

# Run tests
go test -v -race ./...

# Run linter
golangci-lint run
```

### Rust Development (Future)

```bash
cd farp-rust

# Build
cargo build

# Run tests
cargo test

# Run clippy
cargo clippy --all-targets --all-features -- -D warnings

# Format code
cargo fmt
```

## Commit Message Convention

We use [Conventional Commits](https://www.conventionalcommits.org/) for semantic versioning:

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: A new feature (triggers minor version bump)
- `fix`: A bug fix (triggers patch version bump)
- `docs`: Documentation only changes
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement (triggers patch version bump)
- `test`: Adding or updating tests
- `build`: Changes to build system or dependencies
- `ci`: Changes to CI configuration
- `chore`: Other changes that don't modify src or test files
- `revert`: Reverts a previous commit

### Breaking Changes

Add `BREAKING CHANGE:` in the footer or append `!` after type:

```
feat!: change API interface for providers

BREAKING CHANGE: Provider interface now requires context parameter
```

### Examples

```
feat(providers): add Avro schema provider

Implements Apache Avro schema provider with support for
schema registry integration.

Closes #123
```

```
fix(registry): handle race condition in concurrent registration

Fixes a race condition that could occur when multiple
services register simultaneously.

Fixes #456
```

```
docs(readme): update installation instructions

Updates the README with clearer installation steps for
both Go and Rust implementations.
```

## Pull Request Process

1. **Fork and Branch**
   ```bash
   git checkout -b feat/my-new-feature
   ```

2. **Write Code**
   - Follow Go/Rust style guidelines
   - Add tests for new functionality
   - Ensure all tests pass
   - Run linters and fix issues
   - Update documentation

3. **Commit**
   - Use conventional commit messages
   - Keep commits focused and atomic
   - Reference issues in commit messages

4. **Push and Create PR**
   ```bash
   git push origin feat/my-new-feature
   ```
   - Fill out the PR template completely
   - Link related issues
   - Request review from maintainers

5. **Address Review Comments**
   - Make requested changes
   - Push updates to the same branch
   - Respond to comments

6. **Merge**
   - Maintainers will merge after approval
   - Squash commits if requested

## Code Quality Standards

### Go

- **Tests**: 80%+ coverage, 95%+ for critical paths
- **Linting**: Pass all `golangci-lint` checks
- **Race Detector**: Pass `go test -race`
- **Function Size**: Keep functions under 50 lines
- **Cyclomatic Complexity**: Keep below 15
- **Documentation**: Document all exported functions
- **Error Handling**: Always handle errors explicitly
- **Context**: Use `context.Context` for cancellation

### Rust

- **Tests**: Comprehensive unit and integration tests
- **Clippy**: Address all warnings
- **Format**: Run `cargo fmt` before committing
- **Documentation**: Doc comments on public APIs
- **Error Handling**: Use `Result<T, E>`, avoid panics in library code

## Testing Guidelines

### Unit Tests

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name: "valid input",
            input: InputType{...},
            want: OutputType{...},
            wantErr: false,
        },
        {
            name: "invalid input",
            input: InputType{...},
            want: OutputType{},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Tests

- Use `_test.go` suffix
- Use build tags for integration tests: `//go:build integration`
- Clean up resources in test teardown

## Documentation

- Update README.md for user-facing changes
- Update SPECIFICATION.md for protocol changes
- Add inline documentation for complex logic
- Include examples for new features
- Update CHANGELOG.md (handled automatically by CI)

## Release Process

Releases are automated via semantic-release:

1. Merge PR to `main` branch
2. CI analyzes commit messages
3. Version is bumped automatically
4. CHANGELOG is generated
5. GitHub release is created
6. Artifacts are published

### Version Bumping

- `fix:` → patch version (1.0.x)
- `feat:` → minor version (1.x.0)
- `BREAKING CHANGE:` → major version (x.0.0)

## Getting Help

- Open an issue for bugs or feature requests
- Ask questions in discussions
- Review existing documentation
- Check examples directory

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

