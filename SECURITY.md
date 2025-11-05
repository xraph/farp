# Security Policy

## Supported Versions

We actively support the following versions with security updates:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please follow responsible disclosure practices:

### Do NOT:

- Open a public issue
- Post in discussions
- Share details publicly before patch is released

### DO:

1. **Email Security Team**: security@xraph.com (or through GitHub Security Advisory)
2. **Provide Details**:
   - Description of vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)
   - Your contact information

### What to Expect:

- **Initial Response**: Within 48 hours
- **Status Update**: Within 5 business days
- **Fix Timeline**: Depends on severity
  - Critical: 1-7 days
  - High: 7-14 days
  - Medium: 14-30 days
  - Low: 30-90 days

### Disclosure Timeline:

1. Vulnerability reported and confirmed
2. Patch developed and tested
3. Security advisory drafted
4. Patch released
5. Public disclosure (coordinated)

We follow a 90-day disclosure timeline unless:
- Critical vulnerabilities (faster)
- Coordination with other projects needed
- Exceptional circumstances

## Security Best Practices

When using FARP:

### 1. Authentication & Authorization

```go
// Always validate and authenticate service registrations
registry.RegisterManifest(ctx, manifest, WithAuth(token))

// Implement authorization checks
if !isAuthorized(ctx, service) {
    return ErrUnauthorized
}
```

### 2. Input Validation

```go
// Validate all schema manifests
if err := manifest.Validate(); err != nil {
    return fmt.Errorf("invalid manifest: %w", err)
}

// Sanitize URLs and paths
cleanURL := sanitizeURL(schemaLocation.URL)
```

### 3. Transport Security

- Use TLS/HTTPS for all schema URLs
- Verify TLS certificates
- Use mTLS for service-to-gateway communication
- Encrypt sensitive data at rest and in transit

### 4. Schema Verification

```go
// Verify schema checksums
manifest.Schemas[0].Checksum = &SchemaChecksum{
    Algorithm: "sha256",
    Value:     computeSHA256(schemaContent),
}

// Validate before use
if !manifest.VerifyChecksum(schemaContent) {
    return ErrChecksumMismatch
}
```

### 5. Rate Limiting

```go
// Implement rate limiting for registrations
limiter := rate.NewLimiter(rate.Limit(10), 100)
if !limiter.Allow() {
    return ErrRateLimitExceeded
}
```

### 6. Access Control

- Restrict registry access to authorized services
- Use network policies/firewalls
- Implement RBAC for multi-tenant scenarios
- Audit all registration and query operations

### 7. Secret Management

```go
// Never log sensitive data
logger.Info("registration", 
    "service", manifest.ServiceName,
    // DO NOT log tokens, keys, passwords
)

// Use secret management systems
token := os.Getenv("FARP_TOKEN")
if token == "" {
    // Fetch from secrets manager
    token = fetchFromVault(ctx, "farp/token")
}
```

### 8. Denial of Service Prevention

- Limit manifest size (e.g., 10MB max)
- Implement request timeouts
- Use circuit breakers
- Monitor resource usage

```go
// Limit manifest size
if len(manifestBytes) > MaxManifestSize {
    return ErrManifestTooLarge
}

// Timeout contexts
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

### 9. Dependency Management

- Keep dependencies up to date
- Monitor security advisories
- Use Dependabot (already configured)
- Run security scanners (Gosec, Semgrep)

### 10. Error Handling

```go
// Don't leak sensitive information in errors
if err != nil {
    logger.Error("registration failed",
        "service", manifest.ServiceName,
        "error", err)
    // Return generic error to client
    return ErrRegistrationFailed
}
```

## Known Security Considerations

### Schema URLs

When using HTTP-based schema locations:

- **Risk**: Man-in-the-middle attacks
- **Mitigation**: Always use HTTPS with certificate validation
- **Alternative**: Use inline schemas for sensitive services

### mDNS/Bonjour Discovery

When using mDNS for local discovery:

- **Risk**: Broadcast information visible on network
- **Mitigation**: Use on trusted networks only
- **Alternative**: Use encrypted service discovery backends (Consul with TLS)

### Schema Content

Schemas may contain sensitive information:

- **Risk**: API structure exposure
- **Mitigation**: Use access control on schema endpoints
- **Best Practice**: Separate public and internal schemas

## Security Testing

We run multiple security checks:

- **Gosec**: Go security scanner
- **CodeQL**: Semantic code analysis
- **Dependency scanning**: Dependabot and Snyk
- **SAST**: Static analysis in CI
- **Fuzzing**: For critical parsers (future)

## Security Updates

Security patches are released as:

- Patch versions (e.g., 1.2.4)
- Tagged with `security` label
- Announced via GitHub Security Advisories
- Documented in CHANGELOG.md

Subscribe to releases and advisories:
- Watch this repository
- Enable security alerts
- Subscribe to GitHub Security Advisories

## Compliance

FARP is designed to work in regulated environments:

- **No data collection**: FARP doesn't collect or transmit telemetry
- **Audit trails**: Enable comprehensive logging
- **Encryption**: Support for encrypted storage backends
- **Access control**: Pluggable authentication/authorization

## Questions?

For security-related questions (non-vulnerability):
- Open a discussion on GitHub
- Email: security@xraph.com

---

**Last Updated**: 2025-11-01  
**Security Policy Version**: 1.0

