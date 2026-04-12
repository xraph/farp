package farp

import "testing"

func TestMatchPath(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Exact matches
		{"/health", "/health", true},
		{"/health", "/healthz", false},
		{"/api/v1/users", "/api/v1/users", true},
		{"/api/v1/users", "/api/v2/users", false},

		// Single-segment wildcard (*)
		{"/api/*/users", "/api/v1/users", true},
		{"/api/*/users", "/api/v2/users", true},
		{"/api/*/users", "/api/v1/posts", false},
		{"/api/*/users", "/api/users", false},       // * requires exactly one segment
		{"/api/*/users", "/api/v1/v2/users", false}, // * matches only one segment
		{"/api/users/*", "/api/users/123", true},
		{"/api/users/*", "/api/users/123/edit", false},

		// Multi-segment wildcard (**)
		{"/api/**", "/api", true}, // ** matches zero segments
		{"/api/**", "/api/v1", true},
		{"/api/**", "/api/v1/users", true},
		{"/api/**", "/api/v1/users/123", true},
		{"/internal/**", "/internal", true},
		{"/internal/**", "/internal/foo/bar/baz", true},
		{"/**", "/anything/at/all", true},
		{"/**", "/", true},

		// ** in the middle
		{"/api/**/status", "/api/status", true},
		{"/api/**/status", "/api/v1/status", true},
		{"/api/**/status", "/api/v1/users/status", true},
		{"/api/**/status", "/api/v1/users/health", false},

		// Mixed wildcards
		{"/api/*/users/**", "/api/v1/users", true},
		{"/api/*/users/**", "/api/v1/users/123", true},
		{"/api/*/users/**", "/api/v1/users/123/edit", true},
		{"/api/*/users/**", "/api/users/123", false}, // * requires one segment

		// Root path
		{"/", "/", true},
		{"/", "/api", false},

		// Pattern with trailing slash normalization
		{"/_farp/**", "/_farp/manifest", true},
		{"/_farp/**", "/_farp/schemas/openapi", true},
		{"/_/**", "/_/health", true},
		{"/_/**", "/_/info", true},

		// OpenAPI/AsyncAPI prefix patterns
		{"/openapi*", "/openapi.json", true},
		{"/openapi*", "/openapi.yaml", true},
		{"/asyncapi*", "/asyncapi.json", true},
		{"/docs/**", "/docs", true},
		{"/docs/**", "/docs/index.html", true},
		{"/docs", "/docs", true},
		{"/docs", "/docs/index.html", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"→"+tt.path, func(t *testing.T) {
			got := MatchPath(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("MatchPath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestShouldIncludePath(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		rules []PathRule
		want  bool
	}{
		{
			name:  "no rules - include by default",
			path:  "/api/users",
			rules: nil,
			want:  true,
		},
		{
			name: "exclude rule matches",
			path: "/internal/debug",
			rules: []PathRule{
				{Pattern: "/internal/**", Action: PathRuleExclude},
			},
			want: false,
		},
		{
			name: "include rule matches",
			path: "/api/v1/users",
			rules: []PathRule{
				{Pattern: "/api/**", Action: PathRuleInclude},
			},
			want: true,
		},
		{
			name: "first match wins - exclude before include",
			path: "/api/internal/debug",
			rules: []PathRule{
				{Pattern: "/api/internal/**", Action: PathRuleExclude},
				{Pattern: "/api/**", Action: PathRuleInclude},
			},
			want: false,
		},
		{
			name: "first match wins - include before exclude",
			path: "/api/v1/users",
			rules: []PathRule{
				{Pattern: "/api/v1/**", Action: PathRuleInclude},
				{Pattern: "/api/**", Action: PathRuleExclude},
			},
			want: true,
		},
		{
			name: "no match - default include",
			path: "/other/path",
			rules: []PathRule{
				{Pattern: "/api/**", Action: PathRuleExclude},
			},
			want: true,
		},
		{
			name: "exact exclude",
			path: "/health",
			rules: []PathRule{
				{Pattern: "/health", Action: PathRuleExclude},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIncludePath(tt.path, tt.rules)
			if got != tt.want {
				t.Errorf("ShouldIncludePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestBuildPathRules(t *testing.T) {
	userRules := []PathRule{
		{Pattern: "/api/users/*", Action: PathRuleExclude},
	}

	// With internal exclusion enabled
	rules := BuildPathRules(true, userRules)
	if len(rules) <= len(userRules) {
		t.Error("expected internal rules to be prepended")
	}

	// Internal paths should be excluded
	if ShouldIncludePath("/_farp/manifest", rules) {
		t.Error("/_farp/manifest should be excluded with internal rules")
	}

	if ShouldIncludePath("/_/health", rules) {
		t.Error("/_/health should be excluded with internal rules")
	}

	// User rule should also work
	if ShouldIncludePath("/api/users/123", rules) {
		t.Error("/api/users/123 should be excluded by user rule")
	}

	// Non-matching paths should be included
	if !ShouldIncludePath("/api/v1/projects", rules) {
		t.Error("/api/v1/projects should be included")
	}

	// Without internal exclusion
	rulesNoInternal := BuildPathRules(false, userRules)
	if len(rulesNoInternal) != len(userRules) {
		t.Error("expected only user rules when internal exclusion disabled")
	}

	// Internal paths should be included when disabled
	if !ShouldIncludePath("/_farp/manifest", rulesNoInternal) {
		t.Error("/_farp/manifest should be included when internal exclusion disabled")
	}
}

func TestInternalPathRules(t *testing.T) {
	rules := InternalPathRules()

	excluded := []string{"/", "/_farp/manifest", "/_farp/schemas/openapi", "/_/health", "/_/info", "/docs", "/docs/index.html"}
	for _, path := range excluded {
		if ShouldIncludePath(path, rules) {
			t.Errorf("%q should be excluded by internal rules", path)
		}
	}

	included := []string{"/api/v1/users", "/api/v1/projects", "/graphql"}
	for _, path := range included {
		if !ShouldIncludePath(path, rules) {
			t.Errorf("%q should be included (not internal)", path)
		}
	}
}
