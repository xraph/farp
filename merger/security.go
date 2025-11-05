package merger

import (
	"fmt"
	"sort"
)

// SecurityMergeStrategy defines how to handle security/auth during composition
type SecurityMergeStrategy string

const (
	// SecurityStrategyUnion: Combine all security schemes, require ANY to pass
	SecurityStrategyUnion SecurityMergeStrategy = "union"

	// SecurityStrategyIntersection: Only include common schemes, require ALL to pass
	SecurityStrategyIntersection SecurityMergeStrategy = "intersection"

	// SecurityStrategyPerService: Keep service-specific security, prefix scheme names
	SecurityStrategyPerService SecurityMergeStrategy = "per_service"

	// SecurityStrategyGlobal: Use a single global security definition
	SecurityStrategyGlobal SecurityMergeStrategy = "global"

	// SecurityStrategyMostStrict: Use the most restrictive security from any service
	SecurityStrategyMostStrict SecurityMergeStrategy = "most_strict"
)

// SecurityConfig configures security/auth merging behavior
type SecurityConfig struct {
	// Strategy for merging security schemes
	Strategy SecurityMergeStrategy

	// Global security to apply to all operations (overrides service-level)
	GlobalSecurity []map[string][]string

	// Whether to preserve operation-level security requirements
	PreserveOperationSecurity bool

	// Whether to prefix security scheme names with service name
	PrefixSchemeNames bool

	// Security schemes to exclude from merge
	ExcludeSchemes []string

	// Security schemes to always include
	RequiredSchemes []string
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		Strategy:                  SecurityStrategyUnion,
		PreserveOperationSecurity: true,
		PrefixSchemeNames:         false,
		ExcludeSchemes:            []string{},
		RequiredSchemes:           []string{},
	}
}

// SecurityMergeInfo contains information about merged security
type SecurityMergeInfo struct {
	// All security schemes found
	AllSchemes map[string]SecurityScheme

	// Security schemes by service
	SchemesByService map[string][]string

	// Common schemes across all services
	CommonSchemes []string

	// Conflicts found
	Conflicts []SecurityConflict

	// Applied strategy
	Strategy SecurityMergeStrategy
}

// SecurityConflict represents a security scheme conflict
type SecurityConflict struct {
	SchemeName      string
	Services        []string
	ConflictType    string // "definition", "type", "location"
	Resolution      string
	OriginalSchemes map[string]SecurityScheme
}

// MergeSecuritySchemes merges security schemes based on configuration
func MergeSecuritySchemes(
	serviceSchemas map[string]map[string]SecurityScheme,
	config SecurityConfig,
) (*SecurityMergeInfo, map[string]SecurityScheme, error) {
	info := &SecurityMergeInfo{
		AllSchemes:       make(map[string]SecurityScheme),
		SchemesByService: make(map[string][]string),
		CommonSchemes:    []string{},
		Conflicts:        []SecurityConflict{},
		Strategy:         config.Strategy,
	}

	// Track scheme definitions by name
	schemeDefinitions := make(map[string]map[string]SecurityScheme) // scheme name -> service -> definition

	// Collect all schemes
	for service, schemes := range serviceSchemas {
		for name, scheme := range schemes {
			// Skip excluded schemes
			if contains(config.ExcludeSchemes, name) {
				continue
			}

			if schemeDefinitions[name] == nil {
				schemeDefinitions[name] = make(map[string]SecurityScheme)
			}
			schemeDefinitions[name][service] = scheme

			info.SchemesByService[service] = append(info.SchemesByService[service], name)
		}
	}

	// Detect conflicts
	for schemeName, definitions := range schemeDefinitions {
		if len(definitions) > 1 {
			// Check if definitions are identical
			if !areSecuritySchemesIdentical(definitions) {
				conflict := SecurityConflict{
					SchemeName:      schemeName,
					Services:        make([]string, 0, len(definitions)),
					ConflictType:    "definition",
					OriginalSchemes: definitions,
				}
				for service := range definitions {
					conflict.Services = append(conflict.Services, service)
				}
				info.Conflicts = append(info.Conflicts, conflict)
			}
		}
	}

	// Merge based on strategy
	mergedSchemes := make(map[string]SecurityScheme)

	switch config.Strategy {
	case SecurityStrategyUnion:
		// Include all unique schemes
		for schemeName, definitions := range schemeDefinitions {
			if len(definitions) == 1 {
				// No conflict, add directly
				for _, scheme := range definitions {
					mergedSchemes[schemeName] = scheme
					break
				}
			} else {
				// Conflict - use first one or prefix
				if config.PrefixSchemeNames {
					// Prefix all conflicting schemes
					for service, scheme := range definitions {
						prefixedName := service + "_" + schemeName
						mergedSchemes[prefixedName] = scheme
					}
				} else {
					// Use first definition (arbitrary choice)
					for _, scheme := range definitions {
						mergedSchemes[schemeName] = scheme
						break
					}
				}
			}
		}

	case SecurityStrategyIntersection:
		// Only include schemes common to ALL services
		allServices := make(map[string]bool)
		for service := range serviceSchemas {
			allServices[service] = true
		}

		for schemeName, definitions := range schemeDefinitions {
			if len(definitions) == len(allServices) {
				// Scheme exists in all services
				info.CommonSchemes = append(info.CommonSchemes, schemeName)
				// Use first definition
				for _, scheme := range definitions {
					mergedSchemes[schemeName] = scheme
					break
				}
			}
		}

	case SecurityStrategyPerService:
		// Prefix all scheme names with service name
		for service, schemes := range serviceSchemas {
			for name, scheme := range schemes {
				if contains(config.ExcludeSchemes, name) {
					continue
				}
				prefixedName := service + "_" + name
				mergedSchemes[prefixedName] = scheme
			}
		}

	case SecurityStrategyGlobal:
		// Only include required schemes
		for schemeName := range schemeDefinitions {
			if contains(config.RequiredSchemes, schemeName) {
				for _, scheme := range schemeDefinitions[schemeName] {
					mergedSchemes[schemeName] = scheme
					break
				}
			}
		}

	case SecurityStrategyMostStrict:
		// Use the most restrictive version of each scheme
		for schemeName, definitions := range schemeDefinitions {
			mostStrict := findMostStrictScheme(definitions)
			mergedSchemes[schemeName] = mostStrict
		}
	}

	info.AllSchemes = mergedSchemes

	return info, mergedSchemes, nil
}

// MergeOperationSecurity merges operation-level security requirements
func MergeOperationSecurity(
	existingSecurity []map[string][]string,
	newSecurity []map[string][]string,
	strategy SecurityMergeStrategy,
) []map[string][]string {
	switch strategy {
	case SecurityStrategyUnion:
		// Combine both (either can satisfy)
		result := append([]map[string][]string{}, existingSecurity...)
		result = append(result, newSecurity...)
		return result

	case SecurityStrategyIntersection:
		// Require both (AND logic)
		if len(existingSecurity) == 0 {
			return newSecurity
		}
		if len(newSecurity) == 0 {
			return existingSecurity
		}
		// Combine into single requirement
		combined := make(map[string][]string)
		for _, req := range existingSecurity {
			for scheme, scopes := range req {
				combined[scheme] = append(combined[scheme], scopes...)
			}
		}
		for _, req := range newSecurity {
			for scheme, scopes := range req {
				combined[scheme] = append(combined[scheme], scopes...)
			}
		}
		return []map[string][]string{combined}

	case SecurityStrategyMostStrict:
		// Use the one with more requirements
		if len(newSecurity) > len(existingSecurity) {
			return newSecurity
		}
		return existingSecurity

	default:
		// Default: overwrite with new
		return newSecurity
	}
}

// Helper functions

func areSecuritySchemesIdentical(schemes map[string]SecurityScheme) bool {
	if len(schemes) <= 1 {
		return true
	}

	var first SecurityScheme
	firstSet := false

	for _, scheme := range schemes {
		if !firstSet {
			first = scheme
			firstSet = true
			continue
		}

		// Compare key fields
		if first.Type != scheme.Type ||
			first.Name != scheme.Name ||
			first.In != scheme.In ||
			first.Scheme != scheme.Scheme {
			return false
		}
	}

	return true
}

func findMostStrictScheme(schemes map[string]SecurityScheme) SecurityScheme {
	// Simple heuristic: oauth2 > openIdConnect > http > apiKey
	strictness := map[string]int{
		"oauth2":        4,
		"openIdConnect": 3,
		"http":          2,
		"apiKey":        1,
		"":              0,
	}

	var mostStrict SecurityScheme
	maxStrictness := -1

	for _, scheme := range schemes {
		if s, ok := strictness[scheme.Type]; ok && s > maxStrictness {
			maxStrictness = s
			mostStrict = scheme
		}
	}

	return mostStrict
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetSecuritySchemeSummary returns a summary of security schemes
func GetSecuritySchemeSummary(schemes map[string]SecurityScheme) string {
	if len(schemes) == 0 {
		return "No security schemes defined"
	}

	byType := make(map[string][]string)
	for name, scheme := range schemes {
		byType[scheme.Type] = append(byType[scheme.Type], name)
	}

	summary := fmt.Sprintf("Security schemes (%d):\n", len(schemes))
	for schemeType, names := range byType {
		sort.Strings(names)
		summary += fmt.Sprintf("  %s: %v\n", schemeType, names)
	}

	return summary
}

// ValidateSecurityScheme validates a security scheme definition
func ValidateSecurityScheme(name string, scheme SecurityScheme) error {
	if name == "" {
		return fmt.Errorf("security scheme name cannot be empty")
	}

	switch scheme.Type {
	case "apiKey":
		if scheme.Name == "" {
			return fmt.Errorf("apiKey scheme %s missing 'name' field", name)
		}
		if scheme.In == "" {
			return fmt.Errorf("apiKey scheme %s missing 'in' field", name)
		}
		if scheme.In != "query" && scheme.In != "header" && scheme.In != "cookie" {
			return fmt.Errorf("apiKey scheme %s has invalid 'in' value: %s", name, scheme.In)
		}

	case "http":
		if scheme.Scheme == "" {
			return fmt.Errorf("http scheme %s missing 'scheme' field", name)
		}

	case "oauth2":
		// OAuth2 has flows, but we're not validating those in this simplified version

	case "openIdConnect":
		if scheme.OpenIdConnectURL == "" {
			return fmt.Errorf("openIdConnect scheme %s missing 'openIdConnectUrl' field", name)
		}

	case "":
		return fmt.Errorf("security scheme %s missing 'type' field", name)

	default:
		return fmt.Errorf("security scheme %s has unknown type: %s", name, scheme.Type)
	}

	return nil
}
