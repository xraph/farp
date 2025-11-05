#!/bin/bash
set -e

# Update version script for semantic-release
# Usage: ./scripts/update-version.sh VERSION

VERSION=$1

if [ -z "$VERSION" ]; then
  echo "Usage: $0 VERSION"
  exit 1
fi

# Remove 'v' prefix if present
VERSION=${VERSION#v}

# Parse semantic version
IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"

# Update Go version.go file
cat > version.go << EOF
package farp

import "fmt"

// Protocol version constants
const (
	// ProtocolVersion is the current FARP protocol version (semver)
	ProtocolVersion = "$VERSION"

	// ProtocolMajor is the major version
	ProtocolMajor = $MAJOR

	// ProtocolMinor is the minor version
	ProtocolMinor = $MINOR

	// ProtocolPatch is the patch version
	ProtocolPatch = $PATCH
)

// VersionInfo provides version information about the protocol
type VersionInfo struct {
	// Version is the full semver string
	Version string \`json:"version"\`

	// Major version number
	Major int \`json:"major"\`

	// Minor version number
	Minor int \`json:"minor"\`

	// Patch version number
	Patch int \`json:"patch"\`
}

// GetVersion returns the current protocol version information
func GetVersion() VersionInfo {
	return VersionInfo{
		Version: ProtocolVersion,
		Major:   ProtocolMajor,
		Minor:   ProtocolMinor,
		Patch:   ProtocolPatch,
	}
}

// IsCompatible checks if a manifest version is compatible with this protocol version
// Compatible means the major version matches and the manifest's minor version
// is less than or equal to the protocol's minor version
func IsCompatible(manifestVersion string) bool {
	// Parse manifest version (simple parsing for semver)
	var major, minor, patch int
	_, err := fmt.Sscanf(manifestVersion, "%d.%d.%d", &major, &minor, &patch)
	if err != nil {
		return false
	}

	// Major version must match
	if major != ProtocolMajor {
		return false
	}

	// Protocol must support manifest's minor version or higher
	return minor <= ProtocolMinor
}

EOF

echo "Updated version.go to version $VERSION"

# Update Rust Cargo.toml if it exists
if [ -f "farp-rust/Cargo.toml" ]; then
  # Use sed to update version in Cargo.toml (portable across macOS and Linux)
  if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    sed -i '' "s/^version = .*/version = \"$VERSION\"/" farp-rust/Cargo.toml
  else
    # Linux
    sed -i "s/^version = .*/version = \"$VERSION\"/" farp-rust/Cargo.toml
  fi
  echo "Updated farp-rust/Cargo.toml to version $VERSION"
fi

# Update README.md version badge if needed
if [ -f "README.md" ]; then
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s/\*\*Version\*\*: [0-9]\+\.[0-9]\+\.[0-9]\+/**Version**: $VERSION/" README.md
  else
    sed -i "s/\*\*Version\*\*: [0-9]\+\.[0-9]\+\.[0-9]\+/**Version**: $VERSION/" README.md
  fi
  echo "Updated README.md to version $VERSION"
fi

exit 0

