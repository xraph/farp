package farp

import "testing"

func TestConflictStrategy_String(t *testing.T) {
	tests := []struct {
		strategy ConflictStrategy
		expected string
	}{
		{ConflictStrategyPrefix, "prefix"},
		{ConflictStrategyError, "error"},
		{ConflictStrategySkip, "skip"},
		{ConflictStrategyOverwrite, "overwrite"},
		{ConflictStrategyMerge, "merge"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.strategy.String(); got != tt.expected {
				t.Errorf("ConflictStrategy.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
