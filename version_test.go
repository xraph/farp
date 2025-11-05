package farp

import "testing"

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
	tests := []struct {
		name            string
		manifestVersion string
		want            bool
	}{
		{
			name:            "exact match",
			manifestVersion: "1.0.0",
			want:            true,
		},
		{
			name:            "same major, lower minor",
			manifestVersion: "1.0.0",
			want:            true,
		},
		{
			name:            "same major, higher minor",
			manifestVersion: "1.1.0",
			want:            false,
		},
		{
			name:            "different major (higher)",
			manifestVersion: "2.0.0",
			want:            false,
		},
		{
			name:            "different major (lower)",
			manifestVersion: "0.9.0",
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
			manifestVersion: "1.0",
			want:            false,
		},
		{
			name:            "version with extra parts",
			manifestVersion: "1.0.0.0",
			want:            true, // Sscanf will ignore extra parts
		},
		{
			name:            "version with patch difference",
			manifestVersion: "1.0.5",
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
	// Verify protocol version constants are set correctly
	if ProtocolMajor != 1 {
		t.Errorf("ProtocolMajor = %v, want 1", ProtocolMajor)
	}

	if ProtocolMinor != 0 {
		t.Errorf("ProtocolMinor = %v, want 0", ProtocolMinor)
	}

	if ProtocolPatch != 1 {
		t.Errorf("ProtocolPatch = %v, want 1", ProtocolPatch)
	}

	expectedVersion := "1.0.1"
	if ProtocolVersion != expectedVersion {
		t.Errorf("ProtocolVersion = %v, want %v", ProtocolVersion, expectedVersion)
	}
}
