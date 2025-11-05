//! Protocol version constants and compatibility checking.

use serde::{Deserialize, Serialize};

/// Current FARP protocol version (semver)
pub const PROTOCOL_VERSION: &str = "1.0.0";

/// Protocol major version
pub const PROTOCOL_MAJOR: u32 = 1;

/// Protocol minor version
pub const PROTOCOL_MINOR: u32 = 0;

/// Protocol patch version
pub const PROTOCOL_PATCH: u32 = 0;

/// Version information about the protocol
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct VersionInfo {
    /// Full semver string
    pub version: String,
    /// Major version number
    pub major: u32,
    /// Minor version number
    pub minor: u32,
    /// Patch version number
    pub patch: u32,
}

/// Returns the current protocol version information
pub fn get_version() -> VersionInfo {
    VersionInfo {
        version: PROTOCOL_VERSION.to_string(),
        major: PROTOCOL_MAJOR,
        minor: PROTOCOL_MINOR,
        patch: PROTOCOL_PATCH,
    }
}

/// Checks if a manifest version is compatible with this protocol version.
///
/// Compatible means the major version matches and the manifest's minor version
/// is less than or equal to the protocol's minor version.
///
/// # Arguments
///
/// * `manifest_version` - The version string from the manifest (e.g., "1.0.0")
///
/// # Returns
///
/// `true` if the versions are compatible, `false` otherwise
///
/// # Examples
///
/// ```
/// use farp::version::is_compatible;
///
/// assert!(is_compatible("1.0.0"));
/// assert!(is_compatible("1.0.1"));
/// assert!(!is_compatible("2.0.0"));
/// assert!(!is_compatible("0.9.0"));
/// ```
pub fn is_compatible(manifest_version: &str) -> bool {
    let parts: Vec<&str> = manifest_version.split('.').collect();
    if parts.len() != 3 {
        return false;
    }

    let major = parts[0].parse::<u32>().ok();
    let minor = parts[1].parse::<u32>().ok();

    match (major, minor) {
        (Some(major), Some(minor)) => {
            // Major version must match
            if major != PROTOCOL_MAJOR {
                return false;
            }
            // Protocol must support manifest's minor version or higher
            minor <= PROTOCOL_MINOR
        }
        _ => false,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_get_version() {
        let version = get_version();
        assert_eq!(version.version, "1.0.0");
        assert_eq!(version.major, 1);
        assert_eq!(version.minor, 0);
        assert_eq!(version.patch, 0);
    }

    #[test]
    fn test_is_compatible() {
        // Same version - compatible
        assert!(is_compatible("1.0.0"));

        // Same major, same minor, different patch - compatible
        assert!(is_compatible("1.0.1"));
        assert!(is_compatible("1.0.99"));

        // Different major - not compatible
        assert!(!is_compatible("2.0.0"));
        assert!(!is_compatible("0.9.0"));

        // Same major, higher minor - not compatible
        assert!(!is_compatible("1.1.0"));

        // Invalid version strings
        assert!(!is_compatible("1.0"));
        assert!(!is_compatible("1"));
        assert!(!is_compatible("invalid"));
        assert!(!is_compatible(""));
    }
}
