// Package buildinfo exposes build-time metadata for the API.
//
// The values are injected at build time with -ldflags, e.g.:
//
//	go build -ldflags "-X github.com/sorolens/sorolens/apps/api/internal/buildinfo.Version=1.2.3 \
//	  -X github.com/sorolens/sorolens/apps/api/internal/buildinfo.GitSHA=abc1234 \
//	  -X github.com/sorolens/sorolens/apps/api/internal/buildinfo.BuiltAt=2026-01-02T15:04:05Z" ./...
//
// When a value is not injected (local `go run`, plain `go build`, tests) Get
// falls back to "dev" so callers always receive a usable payload.
package buildinfo

import "time"

// var (Version, GitSHA, BuiltAt string) are overridden via -ldflags at build
// time. They stay empty when not injected, which Get turns into "dev".
var (
	Version string
	GitSHA  string
	BuiltAt string
)

// Info is the build metadata returned by GET /api/version.
type Info struct {
	Version string `json:"version"`
	GitSHA  string `json:"git_sha"`
	BuiltAt string `json:"built_at"`
}

// Get returns the build metadata with "dev" substituted for any value that was
// not injected at build time. BuiltAt is only returned when it is a valid
// RFC3339 timestamp; a missing or malformed value falls back to "dev" so the
// field is always parseable by clients.
func Get() Info {
	return Info{
		Version: orDev(Version),
		GitSHA:  orDev(GitSHA),
		BuiltAt: builtAt(),
	}
}

func orDev(v string) string {
	if v == "" {
		return "dev"
	}
	return v
}

func builtAt() string {
	if BuiltAt == "" {
		return "dev"
	}
	if _, err := time.Parse(time.RFC3339, BuiltAt); err != nil {
		return "dev"
	}
	return BuiltAt
}
