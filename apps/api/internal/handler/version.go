package handler

import (
	"net/http"
	"runtime"
)

// APIVersion is the version of the public REST surface served under /api/v1.
const APIVersion = "v1"

// Defaults reported when the binary was built without the ldflags that inject
// real values (for example a plain `go build` during local development).
const (
	DefaultCommit    = "dev"
	DefaultBuildDate = "unknown"
)

// BuildInfo is the build metadata reported by GET /api/v1/version.
//
// Commit and BuildDate are injected at build time:
//
//	go build -ldflags "-X main.Commit=$(git rev-parse HEAD) -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
//
// The entrypoint copies the injected values into Handler.BuildInfo; GoVersion
// and APIVersion are known at runtime and are filled in by normalized, so
// callers only have to supply what cannot be derived.
type BuildInfo struct {
	Commit     string `json:"commit"`
	BuildDate  string `json:"build_date"`
	GoVersion  string `json:"go_version"`
	APIVersion string `json:"api_version"`
}

// normalized fills in the fields that do not need build-time injection so the
// endpoint always returns all four fields, even for a binary built with plain
// `go build`.
func (b BuildInfo) normalized() BuildInfo {
	if b.Commit == "" {
		b.Commit = DefaultCommit
	}
	if b.BuildDate == "" {
		b.BuildDate = DefaultBuildDate
	}
	if b.GoVersion == "" {
		b.GoVersion = runtime.Version()
	}
	if b.APIVersion == "" {
		b.APIVersion = APIVersion
	}
	return b
}

// Version handles GET /api/v1/version. It reports the commit, build date, Go
// version, and API version of the running binary so support and incident
// response can answer "which version is running?" without shell access to the
// host. The route is read-only and requires no API key scope.
func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.BuildInfo.normalized())
}
