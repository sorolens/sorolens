package middleware

import (
	"regexp"
	"strings"
)

// bearerRE matches authorization credentials embedded in a user agent string
// (e.g. some SDKs append the API key to the UA field). The capture group
// preserves the credential prefix so the replacement is "[prefix][redacted]".
var bearerRE = regexp.MustCompile(
	`(?i)(bearer\s+|basic\s+|api[_-]?key[=:\s]+|token[=:\s]+)[A-Za-z0-9\-._~+/]{16,}=*`,
)

// sensitiveHeaders lists header names whose values must always be redacted.
var sensitiveHeaders = map[string]bool{
	"authorization":        true,
	"x-api-key":            true,
	"x-auth-token":         true,
	"cookie":               true,
	"set-cookie":           true,
	"proxy-authorization":  true,
}

// RedactUserAgent redacts authorization credentials that may accidentally
// appear in a user agent string (issue #796). Normal browser or SDK user
// agents that contain no credentials are returned unchanged.
func RedactUserAgent(ua string) string {
	return bearerRE.ReplaceAllStringFunc(ua, func(match string) string {
		// Keep the credential prefix (e.g. "Bearer "), replace the token.
		idx := strings.IndexAny(match, " =:")
		if idx < 0 {
			return "[redacted]"
		}
		return match[:idx+1] + "[redacted]"
	})
}

// RedactHeader returns the header value with the value fully replaced by
// "[redacted]" for headers known to carry credentials, and unchanged otherwise.
func RedactHeader(name, value string) string {
	if sensitiveHeaders[strings.ToLower(name)] {
		return "[redacted]"
	}
	return value
}
