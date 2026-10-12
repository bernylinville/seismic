package es

import (
	"regexp"
	"strconv"
	"strings"
)

// parseVersionParts mirrors parseVersionParts: numeric dot parts, junk → 0.
func parseVersionParts(version string) []int {
	if i := strings.IndexAny(version, "-+"); i >= 0 { // tolerate "7.17.0-SNAPSHOT"
		version = version[:i]
	}
	parts := strings.Split(version, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			n = 0
		}
		out[i] = n
	}
	return out
}

// VersionAtLeast mirrors versionAtLeast: pure semver >=, empty current → false.
func VersionAtLeast(current, required string) bool {
	if current == "" {
		return false
	}
	a, b := parseVersionParts(current), parseVersionParts(required)
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := range n {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x > y {
			return true
		}
		if x < y {
			return false
		}
	}
	return true
}

// MajorVersion returns the cluster's major version, or 0 when unknown.
func MajorVersion(version string) int {
	p := parseVersionParts(version)
	if len(p) == 0 {
		return 0
	}
	return p[0]
}

// SecurityBasePath mirrors currentSecurityApiBasePath/securityApiBasePath:
// prefer the live cluster version, default to the short _security path.
func SecurityBasePath(clusterVersion string) string {
	if clusterVersion == "" {
		return "_security"
	}
	if VersionAtLeast(clusterVersion, "6.6.0") {
		return "_security"
	}
	return "_xpack/security"
}

// SupportsNativeSecurityAPI mirrors supportsNativeSecurityApi: not on
// OpenSearch, requires ES >= 5 (unknown version counts as supported).
func SupportsNativeSecurityAPI(version, distribution string) bool {
	if distribution == "opensearch" {
		return false
	}
	if version == "" {
		return true
	}
	return MajorVersion(version) >= 5
}

// The remaining version gates, one per elasticvue helper (src/helpers/securitySupport.ts).
func SupportsUserEnabledInBody(major int) bool         { return major >= 6 }
func SupportsSecurityDeprecatedMetadata(v string) bool { return VersionAtLeast(v, "7.6.0") }
func SupportsRoleApplications(v string) bool           { return VersionAtLeast(v, "6.4.0") }
func SupportsAllowRestrictedIndices(v string) bool     { return VersionAtLeast(v, "6.7.0") }
func SupportsRoleDescription(v string) bool            { return VersionAtLeast(v, "8.15.0") }

// SupportsSLM mirrors slmSupport.ts: SLM exists from ES 7.4.0.
func SupportsSLM(version string) bool { return VersionAtLeast(version, "7.4.0") }

// SupportedMajorVersions mirrors consts.ts SUPPORTED_MAJOR_VERSIONS.
var SupportedMajorVersions = []string{"7", "8", "9"}

// IsServerless reports the serverless deployment flavor.
func IsServerless(buildFlavor string) bool { return buildFlavor == "serverless" }

func isHexByte(s string, i int) bool {
	if i+1 >= len(s) {
		return false
	}
	return isHexDigit(s[i]) && isHexDigit(s[i+1])
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'A' && c <= 'F' || c >= 'a' && c <= 'f'
}

// escapeLonePercent mirrors /%(?![0-9A-Fa-f]{2})/g: a % not followed by
// exactly two hex digits is escaped to %25.
func escapeLonePercent(s string) string {
	var b strings.Builder
	for i := range len(s) {
		if s[i] == '%' && !isHexByte(s, i+1) {
			b.WriteString("%25")
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

var datemathRe = regexp.MustCompile(`<([^>]*)>`)

// CleanIndexName mirrors cleanIndexName: escape % not part of an escape
// sequence, then URL-encode the inside of datemath <...> expressions.
func CleanIndexName(index string) string {
	result := escapeLonePercent(index)
	result = datemathRe.ReplaceAllStringFunc(result, func(m string) string {
		inner := m[1 : len(m)-1]
		repl := strings.NewReplacer(
			"<", "%3C", ">", "%3E", "/", "%2F", "{", "%7B", "}", "%7D",
			"|", "%7C", "+", "%2B", ":", "%3A", ",", "%2C",
		)
		return "%3C" + repl.Replace(inner) + "%3E"
	})
	return result
}
