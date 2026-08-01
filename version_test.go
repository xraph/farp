package farp

import (
	"fmt"
	"testing"
)

func TestGetVersion(t *testing.T) {
	version := GetVersion()

	if version.Version != ProtocolVersion {
		t.Errorf("GetVersion().Version = %v, want %v", version.Version, ProtocolVersion)
	}

	if version.Major != ProtocolMajor {
		t.Errorf("GetVersion().Major = %v, want %v", version.Major, ProtocolMajor)
	}

	if version.Minor != ProtocolMinor {
		t.Errorf("GetVersion().Minor = %v, want %v", version.Minor, ProtocolMinor)
	}

	if version.Patch != ProtocolPatch {
		t.Errorf("GetVersion().Patch = %v, want %v", version.Patch, ProtocolPatch)
	}
}

func TestIsCompatible(t *testing.T) {
	// Cases are expressed relative to the protocol constants so a release bump
	// does not invalidate them.
	tests := []struct {
		name            string
		manifestVersion string
		want            bool
	}{
		{
			name:            "exact match",
			manifestVersion: ProtocolVersion,
			want:            true,
		},
		{
			name:            "same major, lower minor",
			manifestVersion: fmt.Sprintf("%d.0.0", ProtocolMajor),
			want:            true,
		},
		{
			name:            "same major, same minor",
			manifestVersion: fmt.Sprintf("%d.%d.0", ProtocolMajor, ProtocolMinor),
			want:            true,
		},
		{
			name:            "same major, higher minor",
			manifestVersion: fmt.Sprintf("%d.%d.0", ProtocolMajor, ProtocolMinor+1),
			want:            false,
		},
		{
			name:            "different major (higher)",
			manifestVersion: fmt.Sprintf("%d.0.0", ProtocolMajor+1),
			want:            false,
		},
		{
			name:            "different major (lower)",
			manifestVersion: fmt.Sprintf("%d.9.0", ProtocolMajor-1),
			want:            false,
		},
		{
			name:            "invalid version format",
			manifestVersion: "invalid",
			want:            false,
		},
		{
			name:            "empty version",
			manifestVersion: "",
			want:            false,
		},
		{
			name:            "partial version",
			manifestVersion: fmt.Sprintf("%d.0", ProtocolMajor),
			want:            false,
		},
		{
			name:            "version with extra parts",
			manifestVersion: fmt.Sprintf("%d.0.0.0", ProtocolMajor),
			want:            true, // Sscanf will ignore extra parts
		},
		{
			name:            "version with patch difference",
			manifestVersion: fmt.Sprintf("%d.0.5", ProtocolMajor),
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCompatible(tt.manifestVersion); got != tt.want {
				t.Errorf("IsCompatible(%v) = %v, want %v", tt.manifestVersion, got, tt.want)
			}
		})
	}
}

func TestProtocolConstants(t *testing.T) {
	// The protocol major version is a deliberate, breaking-change-only decision,
	// so it is pinned. Minor and patch are moved by release automation
	// (scripts/update-version.sh) and are only checked for self-consistency.
	if ProtocolMajor != 1 {
		t.Errorf("ProtocolMajor = %v, want 1", ProtocolMajor)
	}

	if ProtocolMinor < 0 {
		t.Errorf("ProtocolMinor = %v, want >= 0", ProtocolMinor)
	}

	if ProtocolPatch < 0 {
		t.Errorf("ProtocolPatch = %v, want >= 0", ProtocolPatch)
	}

	expectedVersion := fmt.Sprintf("%d.%d.%d", ProtocolMajor, ProtocolMinor, ProtocolPatch)
	if ProtocolVersion != expectedVersion {
		t.Errorf("ProtocolVersion = %v, want %v", ProtocolVersion, expectedVersion)
	}
}
