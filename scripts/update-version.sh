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

# Portable in-place sed (BSD sed on macOS needs an explicit empty suffix).
sed_inplace() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "$1" "$2"
  else
    sed -i "$1" "$2"
  fi
}

# Patch the constants in version.go in place rather than regenerating the file,
# so comments and formatting stay whatever the linter requires.
sed_inplace "s|ProtocolVersion = \".*\"|ProtocolVersion = \"$VERSION\"|" version.go
sed_inplace "s|ProtocolMajor = .*|ProtocolMajor = $MAJOR|" version.go
sed_inplace "s|ProtocolMinor = .*|ProtocolMinor = $MINOR|" version.go
sed_inplace "s|ProtocolPatch = .*|ProtocolPatch = $PATCH|" version.go

# Fail loudly if the constants moved and the patterns above stopped matching.
grep -q "ProtocolVersion = \"$VERSION\"" version.go || {
  echo "error: failed to update ProtocolVersion in version.go" >&2
  exit 1
}
grep -q "ProtocolMajor = $MAJOR" version.go || {
  echo "error: failed to update ProtocolMajor in version.go" >&2
  exit 1
}
grep -q "ProtocolMinor = $MINOR" version.go || {
  echo "error: failed to update ProtocolMinor in version.go" >&2
  exit 1
}
grep -q "ProtocolPatch = $PATCH" version.go || {
  echo "error: failed to update ProtocolPatch in version.go" >&2
  exit 1
}

echo "Updated version.go to version $VERSION"

# Update Rust Cargo.toml if it exists
if [ -f "farp-rust/Cargo.toml" ]; then
  sed_inplace "s/^version = .*/version = \"$VERSION\"/" farp-rust/Cargo.toml
  echo "Updated farp-rust/Cargo.toml to version $VERSION"
fi

exit 0

