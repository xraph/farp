package farp

import "strings"

// MatchPath checks whether a URL path matches a glob pattern.
//
// Pattern syntax:
//   - Literal segments match exactly: "/api/v1/users" matches "/api/v1/users"
//   - "*" matches exactly one path segment: "/api/*/status" matches "/api/users/status"
//   - "**" matches zero or more path segments: "/api/**" matches "/api", "/api/v1", "/api/v1/users/123"
func MatchPath(pattern, path string) bool {
	patternParts := splitPath(pattern)
	pathParts := splitPath(path)

	return matchParts(patternParts, pathParts)
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func matchParts(pattern, path []string) bool {
	pi, pa := 0, 0

	for pi < len(pattern) && pa < len(path) {
		seg := pattern[pi]

		switch {
		case seg == "**":
			// If ** is the last pattern segment, it matches everything remaining
			if pi == len(pattern)-1 {
				return true
			}

			// Try matching ** against zero or more path segments
			for tryPa := pa; tryPa <= len(path); tryPa++ {
				if matchParts(pattern[pi+1:], path[tryPa:]) {
					return true
				}
			}

			return false

		case seg == "*":
			// Matches exactly one segment — just advance both
			pi++
			pa++

		case strings.Contains(seg, "*"):
			// Segment contains inline wildcard (e.g., "openapi*" matches "openapi.json")
			if !matchSegment(seg, path[pa]) {
				return false
			}
			pi++
			pa++

		default:
			if seg != path[pa] {
				return false
			}
			pi++
			pa++
		}
	}

	// Handle trailing ** which can match zero segments
	for pi < len(pattern) && pattern[pi] == "**" {
		pi++
	}

	return pi == len(pattern) && pa == len(path)
}

// matchSegment matches a single path segment against a pattern segment
// containing inline wildcards (e.g., "openapi*" matches "openapi.json").
func matchSegment(pattern, segment string) bool {
	// Simple prefix/suffix matching for inline *
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(segment, prefix)
	}

	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(segment, suffix)
	}

	// * in the middle: split and check prefix + suffix
	if idx := strings.Index(pattern, "*"); idx >= 0 {
		prefix := pattern[:idx]
		suffix := pattern[idx+1:]
		return strings.HasPrefix(segment, prefix) && strings.HasSuffix(segment, suffix) && len(segment) >= len(prefix)+len(suffix)
	}

	return pattern == segment
}

// ShouldIncludePath evaluates path rules in order and returns whether the
// path should be included. First matching rule wins. If no rule matches,
// the path is included by default.
func ShouldIncludePath(path string, rules []PathRule) bool {
	for _, rule := range rules {
		if MatchPath(rule.Pattern, path) {
			return rule.Action == PathRuleInclude
		}
	}
	return true // default: include
}

// InternalPathRules returns the default rules for excluding framework
// introspection endpoints. Prepend these to user rules when
// ExcludeInternalPaths is enabled.
func InternalPathRules() []PathRule {
	return []PathRule{
		{Pattern: "/", Action: PathRuleExclude},
		{Pattern: "/_farp/**", Action: PathRuleExclude},
		{Pattern: "/_/**", Action: PathRuleExclude},
		{Pattern: "/docs/**", Action: PathRuleExclude},
		{Pattern: "/docs", Action: PathRuleExclude},
		{Pattern: "/openapi*", Action: PathRuleExclude},
		{Pattern: "/asyncapi*", Action: PathRuleExclude},
	}
}

// BuildPathRules combines internal rules (if enabled) with user-defined rules.
// Internal rules are prepended so they run first.
func BuildPathRules(excludeInternal bool, userRules []PathRule) []PathRule {
	if !excludeInternal {
		return userRules
	}
	return append(InternalPathRules(), userRules...)
}
