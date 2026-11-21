package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
)

// doctorSchemaVersion is the version of the `sorolens doctor --json` document.
// Bump it only for breaking changes (a removed or renamed field or check);
// adding a field or a check keeps the version.
const doctorSchemaVersion = 1

// Server versions this CLI works with: >= minServerVersion and < maxServerVersion.
const (
	minServerVersion = "0.1.0"
	maxServerVersion = "1.0.0"
)

// Thresholds for the clock and TLS checks.
const (
	clockSkewWarn  = 30 * time.Second
	clockSkewFail  = 5 * time.Minute
	certExpiryWarn = 14 * 24 * time.Hour
)

// Check statuses.
const (
	doctorPass = "pass"
	doctorWarn = "warn"
	doctorFail = "fail"
)

// Check names, in the order they run and are reported. They are part of the
// --json contract.
const (
	checkConfig  = "config"
	checkReach   = "reachability"
	checkTLS     = "tls"
	checkAuth    = "auth"
	checkVersion = "version"
	checkClock   = "clock"
)

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

type doctorSummary struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
}

type doctorReport struct {
	SchemaVersion int           `json:"schema_version"`
	OK            bool          `json:"ok"`
	CLIVersion    string        `json:"cli_version"`
	APIURL        string        `json:"api_url"`
	Profile       string        `json:"profile"`
	Summary       doctorSummary `json:"summary"`
	Checks        []doctorCheck `json:"checks"`
}

// errDoctorFailed makes `sorolens doctor` exit non-zero when a check fails.
type errDoctorFailed struct{ n int }

func (e errDoctorFailed) Error() string {
	if e.n == 1 {
		return "doctor: 1 check failed"
	}
	return fmt.Sprintf("doctor: %d checks failed", e.n)
}

// Test seams: the transport the checks use and the local clock.
var (
	doctorTransport http.RoundTripper
	doctorNow       = time.Now
)

// doctorConfigErr holds the error applyConfig returned, if any. doctor never
// stops on it: the config check reports it instead.
var doctorConfigErr error

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose configuration, connectivity, authentication, version and clock problems",
	Long: `Run a set of checks against the active configuration and the Sorolens API and
print pass, warn or fail for each, with a hint on how to fix anything that is
not passing:

  config        config file, active profile, API URL and where the API key comes from
  reachability  the API answers at --api-url
  tls           HTTPS is used and the server certificate is valid
  auth          the API key is accepted and has the read scopes the CLI needs
  version       the server version is in the range this CLI supports
  clock         local clock skew against the server's Date header

API keys and tokens never appear in the output, so the report is safe to
paste into an issue. Use --json for a stable machine-readable report. The
command exits with status 1 when any check fails.`,
	Example: `  sorolens doctor
  sorolens doctor --profile prod
  sorolens doctor --json`,
	Args: cobra.NoArgs,
	// doctor diagnoses a broken configuration, so it must not stop on the
	// error the root pre-run would return for it.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		doctorConfigErr = applyConfig(cmd, args)
		if doctorConfigErr != nil {
			applyDoctorFallbackConfig(cmd)
		}
		return nil
	},
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

// applyDoctorFallbackConfig resolves the API URL, key and output format the
// best it can when applyConfig failed (an unreadable config file, an unknown
// profile or a bad --output value).
func applyDoctorFallbackConfig(cmd *cobra.Command) {
	cfg, _ := loadUserConfig()
	if !cmd.Flags().Changed("api-url") {
		globalConfig.APIURL = firstNonEmpty(os.Getenv("SOROLENS_API_URL"), cfg.APIURL, "http://localhost:8080")
	}
	if !cmd.Flags().Changed("api-key") {
		globalConfig.APIKey = firstNonEmpty(os.Getenv("SOROLENS_API_KEY"), cfg.APIKey)
	}
	out := strings.ToLower(strings.TrimSpace(globalConfig.Output))
	switch {
	case globalConfig.JSON:
		out = format.FormatJSON
	case !format.ValidFormat(out):
		out = format.FormatTable
	}
	globalConfig.Output = out
	globalConfig.JSON = out != format.FormatTable
	format.SetOptions(format.Options{Format: out})
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	report := diagnose(cmd.Context())
	if globalConfig.JSON {
		if err := format.PrintJSON(report); err != nil {
			return err
		}
	} else {
		printDoctorReport(os.Stdout, report)
	}
	if report.Summary.Fail > 0 {
		return errDoctorFailed{n: report.Summary.Fail}
	}
	return nil
}

// doctorRun carries the state shared between checks.
type doctorRun struct {
	ctx     context.Context
	base    *url.URL
	apiKey  string
	client  *http.Client
	redact  func(string) string
	checks  []doctorCheck
	healthy *probeResult // the /health response, when the API answered
}

type probeResult struct {
	status int
	header http.Header
	body   []byte
	tls    *tls.ConnectionState
	sent   time.Time
	rtt    time.Duration
}

func diagnose(ctx context.Context) doctorReport {
	if ctx == nil {
		ctx = context.Background()
	}
	apiKey := resolvedAPIKey()
	run := &doctorRun{
		ctx:    ctx,
		apiKey: apiKey,
		redact: newRedactor(apiKey),
		client: &http.Client{Timeout: globalConfig.Timeout, Transport: doctorTransport},
	}
	if run.client.Timeout <= 0 {
		run.client.Timeout = 10 * time.Second
	}

	urlOK := run.checkConfig()
	if urlOK {
		run.checkReachability()
	} else {
		run.skip(checkReach, "API URL is invalid")
		run.skip(checkTLS, "API URL is invalid")
	}
	if run.healthy != nil {
		run.checkAuth()
		run.checkVersion()
		run.checkClock()
	} else {
		reason := "the API is not reachable"
		if !urlOK {
			reason = "API URL is invalid"
		}
		run.skip(checkAuth, reason)
		run.skip(checkVersion, reason)
		run.skip(checkClock, reason)
	}

	profile := globalConfig.Profile
	if profile == "" {
		profile = "(none)"
	}
	report := doctorReport{
		SchemaVersion: doctorSchemaVersion,
		CLIVersion:    Version,
		APIURL:        run.redact(redactURL(globalConfig.APIURL)),
		Profile:       profile,
		Checks:        run.checks,
	}
	for _, c := range report.Checks {
		switch c.Status {
		case doctorPass:
			report.Summary.Pass++
		case doctorWarn:
			report.Summary.Warn++
		case doctorFail:
			report.Summary.Fail++
		}
	}
	report.OK = report.Summary.Fail == 0
	return report
}

func (r *doctorRun) add(name, status, message, hint string) {
	r.checks = append(r.checks, doctorCheck{
		Name:    name,
		Status:  status,
		Message: r.redact(message),
		Hint:    r.redact(hint),
	})
}

func (r *doctorRun) skip(name, reason string) {
	r.add(name, doctorWarn, "Not checked: "+reason+".", "Fix the failing check above and run 'sorolens doctor' again.")
}

// checkConfig reports on the config file, the active profile, the API URL and
// the API key source. It returns false when the API URL is unusable.
func (r *doctorRun) checkConfig() bool {
	path, _ := userConfigPath()
	display := "~/.sorolens.yml"

	cfg, err := loadUserConfig()
	if err != nil {
		r.add(checkConfig, doctorFail,
			fmt.Sprintf("Cannot use %s: %v.", display, err),
			fmt.Sprintf("Fix the YAML in %s, or move it aside to fall back to defaults.", path))
		return r.checkURL(false)
	}

	profileName := firstNonEmpty(globalConfig.Profile, os.Getenv("SOROLENS_PROFILE"), cfg.DefaultProfile)
	prof, found := cfg.Profiles[profileName]
	if profileName != "" && !found {
		r.add(checkConfig, doctorFail,
			fmt.Sprintf("Profile %q is not defined in %s.", profileName, display),
			"Run 'sorolens config profile list' to see the profiles, or create it with 'sorolens config profile add "+profileName+"'.")
		return r.checkURL(false)
	}
	if doctorConfigErr != nil {
		// Something else in the configuration (e.g. --output, --query) is
		// invalid; report it rather than hide it.
		r.add(checkConfig, doctorFail, fmt.Sprintf("Invalid configuration: %v.", doctorConfigErr),
			"Fix the flag, environment variable or config entry named in the message.")
		return r.checkURL(false)
	}

	if !r.checkURL(true) {
		return false
	}

	_, statErr := os.Stat(path)
	desc := "Using " + display
	if errors.Is(statErr, os.ErrNotExist) {
		desc = "No config file; using flags, environment and defaults"
	}
	if profileName != "" {
		desc += fmt.Sprintf(", profile %q", profileName)
	}
	desc += ", API " + redactURL(globalConfig.APIURL)

	switch {
	case cfg.APIKey != "" && !flagOrEnvKey():
		r.add(checkConfig, doctorWarn,
			desc+". The API key is stored in plain text in "+display+".",
			"Move it to the OS keyring: 'sorolens config profile add <name> --api-key-stdin', then remove api_key from "+display+".")
	case prof.KeyRef != "" && r.apiKey == "":
		r.add(checkConfig, doctorWarn,
			desc+". The profile expects an API key in the "+prof.KeyRef+" store, but none was found.",
			"Store it again: 'sorolens config profile add "+profileName+" --api-key-stdin'.")
	default:
		r.add(checkConfig, doctorPass, desc+".", "")
	}
	return true
}

// flagOrEnvKey reports whether the key came from --api-key or SOROLENS_API_KEY.
func flagOrEnvKey() bool {
	if f := rootCmd.PersistentFlags().Lookup("api-key"); f != nil && f.Changed {
		return true
	}
	return strings.TrimSpace(os.Getenv("SOROLENS_API_KEY")) != ""
}

// checkURL validates globalConfig.APIURL. When report is true a problem is
// added as a failing config check; otherwise the config check already failed
// and the URL is only validated.
func (r *doctorRun) checkURL(report bool) bool {
	u, err := url.Parse(strings.TrimRight(globalConfig.APIURL, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		if report {
			r.add(checkConfig, doctorFail,
				fmt.Sprintf("API URL %q is not a valid http(s) URL.", redactURL(globalConfig.APIURL)),
				"Set --api-url, SOROLENS_API_URL or api_url to something like https://api.sorolens.xyz.")
		}
		return false
	}
	u.RawQuery, u.Fragment = "", ""
	r.base = u
	return true
}

func (r *doctorRun) get(path string, withKey bool) (*probeResult, error) {
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, r.base.String()+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sorolens-cli/"+Version+" (doctor)")
	req.Header.Set("Accept", "application/json")
	if withKey && r.apiKey != "" {
		req.Header.Set("X-API-Key", r.apiKey)
		req.Header.Set("Authorization", "Bearer "+r.apiKey)
	}
	sent := doctorNow()
	start := time.Now()
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return &probeResult{
		status: resp.StatusCode,
		header: resp.Header,
		body:   body,
		tls:    resp.TLS,
		sent:   sent,
		rtt:    time.Since(start),
	}, nil
}

func (r *doctorRun) checkReachability() {
	res, err := r.get("/health", false)
	if err != nil {
		r.add(checkReach, doctorFail, "Cannot reach "+redactURL(r.base.String())+": "+describeNetErr(err)+".", reachHint(err))
		if isTLSError(err) {
			r.add(checkTLS, doctorFail, "TLS handshake failed: "+describeNetErr(err)+".",
				"Check that the server certificate is valid for "+r.base.Hostname()+" and issued by a trusted CA; on a private CA, add it to the system trust store.")
		} else {
			r.skip(checkTLS, "the API is not reachable")
		}
		return
	}
	r.healthy = res
	latency := res.rtt.Round(time.Millisecond)
	if res.status >= 500 {
		r.add(checkReach, doctorWarn,
			fmt.Sprintf("API answered GET /health with HTTP %d in %s.", res.status, latency),
			"The server is up but unhealthy; check its logs and its database and Redis connections.")
	} else {
		r.add(checkReach, doctorPass, fmt.Sprintf("API answered GET /health with HTTP %d in %s.", res.status, latency), "")
	}
	r.checkTLS(res)
}

func (r *doctorRun) checkTLS(res *probeResult) {
	if r.base.Scheme == "http" {
		if isLocalHost(r.base.Hostname()) {
			r.add(checkTLS, doctorPass, "Plain HTTP to a local API.", "")
			return
		}
		r.add(checkTLS, doctorWarn, "The API URL uses plain HTTP, so the API key is sent unencrypted.",
			"Use an https:// API URL.")
		return
	}
	if res.tls == nil || len(res.tls.PeerCertificates) == 0 {
		r.add(checkTLS, doctorWarn, "HTTPS was requested but no TLS connection details were available.", "")
		return
	}
	cert := res.tls.PeerCertificates[0]
	version := tls.VersionName(res.tls.Version)
	left := cert.NotAfter.Sub(doctorNow())
	expiry := cert.NotAfter.UTC().Format("2006-01-02")
	if left < certExpiryWarn {
		r.add(checkTLS, doctorWarn,
			fmt.Sprintf("%s; the server certificate expires on %s (in %s).", version, expiry, humanDuration(left)),
			"Renew the server certificate before it expires.")
		return
	}
	r.add(checkTLS, doctorPass, fmt.Sprintf("%s; certificate valid until %s.", version, expiry), "")
}

// checkAuth verifies the API key against a read:contracts route and, when
// that passes, a read:watchdog route.
func (r *doctorRun) checkAuth() {
	if r.apiKey == "" {
		r.add(checkAuth, doctorWarn, "No API key is configured; only anonymous read access is available.",
			"Create a key in the dashboard and store it with 'sorolens config profile add <name> --api-key-stdin'.")
		return
	}
	res, err := r.get("/api/v1/contracts?limit=1", true)
	if err != nil {
		r.add(checkAuth, doctorWarn, "Could not verify the API key: "+describeNetErr(err)+".", reachHint(err))
		return
	}
	switch {
	case res.status == http.StatusUnauthorized:
		r.add(checkAuth, doctorFail, "The API key was rejected (HTTP 401): it is unknown, revoked or expired.",
			"Create a new key and store it with 'sorolens config profile add <name> --api-key-stdin'.")
		return
	case res.status == http.StatusForbidden:
		scope := requiredScope(res.body, "read:contracts")
		r.add(checkAuth, doctorFail, fmt.Sprintf("The API key is valid but lacks the %s scope (HTTP 403).", scope),
			fmt.Sprintf("Create a key with the %s scope; most commands need it.", scope))
		return
	case res.status == http.StatusTooManyRequests:
		r.add(checkAuth, doctorWarn, "Could not verify the API key: rate limited (HTTP 429).",
			"Wait "+retryAfter(res.header)+" and run 'sorolens doctor' again.")
		return
	case res.status < 200 || res.status >= 300:
		r.add(checkAuth, doctorWarn, fmt.Sprintf("Could not verify the API key: GET /api/v1/contracts answered HTTP %d.", res.status),
			"Check the server logs, then run 'sorolens doctor' again.")
		return
	}

	wd, err := r.get("/api/v1/watchdog/stats", true)
	if err == nil && wd.status == http.StatusForbidden {
		r.add(checkAuth, doctorWarn, "The API key is accepted and has read:contracts, but lacks read:watchdog.",
			"Watchdog and alerts commands will fail; create a key with the read:watchdog scope if you need them.")
		return
	}
	if err != nil || wd.status < 200 || wd.status >= 300 {
		r.add(checkAuth, doctorPass, "The API key is accepted and has read:contracts (read:watchdog not verified).", "")
		return
	}
	r.add(checkAuth, doctorPass, "The API key is accepted and has read:contracts and read:watchdog.", "")
}

func (r *doctorRun) checkVersion() {
	supported := fmt.Sprintf(">=%s <%s", minServerVersion, maxServerVersion)
	res, err := r.get("/api/version", false)
	if err != nil {
		r.add(checkVersion, doctorWarn, "Could not read the server version: "+describeNetErr(err)+".", reachHint(err))
		return
	}
	if res.status == http.StatusNotFound {
		r.add(checkVersion, doctorWarn, "The server does not report its version (GET /api/version returned 404).",
			fmt.Sprintf("The server is probably older than %s; upgrade it to a supported version (%s).", minServerVersion, supported))
		return
	}
	var info struct {
		Version string `json:"version"`
	}
	if res.status != http.StatusOK || json.Unmarshal(res.body, &info) != nil || info.Version == "" {
		r.add(checkVersion, doctorWarn, fmt.Sprintf("Could not read the server version (GET /api/version answered HTTP %d).", res.status),
			"Check that --api-url points at the Sorolens API and not at the dashboard.")
		return
	}
	if info.Version == "dev" {
		r.add(checkVersion, doctorWarn, "The server is a development build, so compatibility cannot be checked.",
			fmt.Sprintf("This CLI supports server versions %s.", supported))
		return
	}
	v, ok := parseSemver(info.Version)
	if !ok {
		r.add(checkVersion, doctorWarn, fmt.Sprintf("The server reports an unrecognised version %q.", info.Version),
			fmt.Sprintf("This CLI supports server versions %s.", supported))
		return
	}
	minV, _ := parseSemver(minServerVersion)
	maxV, _ := parseSemver(maxServerVersion)
	switch {
	case compareSemver(v, minV) < 0:
		r.add(checkVersion, doctorFail,
			fmt.Sprintf("Server %s is older than this CLI supports (%s).", info.Version, supported),
			"Upgrade the server, or install a CLI release that matches it.")
	case compareSemver(v, maxV) >= 0:
		r.add(checkVersion, doctorFail,
			fmt.Sprintf("Server %s is newer than this CLI supports (%s).", info.Version, supported),
			"Upgrade the sorolens CLI.")
	default:
		r.add(checkVersion, doctorPass, fmt.Sprintf("Server %s is supported (%s).", info.Version, supported), "")
	}
}

func (r *doctorRun) checkClock() {
	res := r.healthy
	date := res.header.Get("Date")
	if date == "" {
		r.add(checkClock, doctorWarn, "The server sent no Date header, so clock skew cannot be measured.", "")
		return
	}
	server, err := http.ParseTime(date)
	if err != nil {
		r.add(checkClock, doctorWarn, fmt.Sprintf("The server sent an unreadable Date header %q.", date), "")
		return
	}
	// The server stamped the response roughly half-way through the round trip.
	local := res.sent.Add(res.rtt / 2)
	skew := local.Sub(server)
	abs := skew
	if abs < 0 {
		abs = -abs
	}
	dir := "ahead of"
	if skew < 0 {
		dir = "behind"
	}
	msg := fmt.Sprintf("Local clock is %s %s the server.", humanDuration(abs), dir)
	hint := "Sync your system clock (enable NTP / automatic time)."
	switch {
	case abs >= clockSkewFail:
		r.add(checkClock, doctorFail, msg+" Signed requests, token expiry and time filters will misbehave.", hint)
	case abs >= clockSkewWarn:
		r.add(checkClock, doctorWarn, msg, hint)
	default:
		r.add(checkClock, doctorPass, fmt.Sprintf("Local clock is within %s of the server.", clockSkewWarn), "")
	}
}

// --- output -----------------------------------------------------------------

func printDoctorReport(w io.Writer, rep doctorReport) {
	color := format.IsColorEnabled() && !globalConfig.NoColor
	styles := map[string]lipgloss.Style{
		doctorPass: lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true),
		doctorWarn: lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),
		doctorFail: lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
	}
	fmt.Fprintf(w, "sorolens doctor (CLI %s, API %s, profile %s)\n\n", rep.CLIVersion, rep.APIURL, rep.Profile)
	for _, c := range rep.Checks {
		label := fmt.Sprintf("%-4s", strings.ToUpper(c.Status))
		if color {
			label = styles[c.Status].Render(label)
		}
		fmt.Fprintf(w, "%s  %-12s  %s\n", label, c.Name, c.Message)
		if c.Hint != "" {
			fmt.Fprintf(w, "%s  %-12s  hint: %s\n", "    ", "", c.Hint)
		}
	}
	fmt.Fprintf(w, "\n%d passed, %d warnings, %d failed\n", rep.Summary.Pass, rep.Summary.Warn, rep.Summary.Fail)
}

// --- redaction --------------------------------------------------------------

var (
	reBearer   = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`)
	reKeyParam = regexp.MustCompile(`(?i)((?:api[_-]?key|token|access_token)=)[^&\s"']+`)
	reKeyLike  = regexp.MustCompile(`\b(?:sl|whsec)_[A-Za-z0-9_-]{16,}`)
)

// newRedactor returns a function that removes the API key and anything that
// looks like a credential from text before it is printed.
func newRedactor(key string) func(string) string {
	key = strings.TrimSpace(key)
	return func(s string) string {
		if key != "" {
			s = strings.ReplaceAll(s, key, "[REDACTED]")
		}
		s = reBearer.ReplaceAllString(s, "${1}[REDACTED]")
		s = reKeyParam.ReplaceAllString(s, "${1}[REDACTED]")
		s = reKeyLike.ReplaceAllString(s, "[REDACTED]")
		return s
	}
}

// redactURL drops user info and the query string (where a key could be passed
// as ?api_key=) from a URL before it is printed.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return reKeyParam.ReplaceAllString(raw, "${1}[REDACTED]")
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	return u.String()
}

// --- helpers ----------------------------------------------------------------

func describeNetErr(err error) string {
	var dnsErr *net.DNSError
	var urlErr *url.Error
	switch {
	case errors.As(err, &dnsErr):
		return "cannot resolve host " + dnsErr.Name
	case isTLSError(err):
		var certErr *tls.CertificateVerificationError
		if errors.As(err, &certErr) && certErr.Err != nil {
			return certErr.Err.Error()
		}
	case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
		return "timed out after " + globalConfig.Timeout.String()
	}
	if errors.As(err, &urlErr) {
		return urlErr.Err.Error()
	}
	return err.Error()
}

func reachHint(err error) string {
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &dnsErr):
		return "Check the host name in --api-url / SOROLENS_API_URL and your DNS settings."
	case isTLSError(err):
		return "Fix the server certificate, or check that --api-url uses the right scheme and host."
	case isTimeout(err):
		return "Check your network, VPN or proxy, or raise --timeout."
	case strings.Contains(strings.ToLower(err.Error()), "connection refused"):
		return "Nothing is listening there: start the API or fix the port in --api-url."
	}
	return "Check --api-url / SOROLENS_API_URL and your network connection."
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func isTLSError(err error) bool {
	var certErr *tls.CertificateVerificationError
	var unknownAuth x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	var invalid x509.CertificateInvalidError
	var recErr tls.RecordHeaderError
	return errors.As(err, &certErr) || errors.As(err, &unknownAuth) || errors.As(err, &hostErr) ||
		errors.As(err, &invalid) || errors.As(err, &recErr)
}

func isLocalHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func requiredScope(body []byte, fallback string) string {
	var e struct {
		Required string `json:"required"`
	}
	if json.Unmarshal(body, &e) == nil && e.Required != "" {
		return e.Required
	}
	return fallback
}

func retryAfter(h http.Header) string {
	if s, err := strconv.Atoi(h.Get("Retry-After")); err == nil && s > 0 {
		return humanDuration(time.Duration(s) * time.Second)
	}
	return "a minute"
}

func humanDuration(d time.Duration) string {
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	case d >= time.Hour:
		return d.Round(time.Minute).String()
	case d >= time.Second:
		return d.Round(time.Second).String()
	}
	return d.Round(time.Millisecond).String()
}

type semver [3]int

// parseSemver reads MAJOR.MINOR.PATCH with an optional leading v and ignores
// pre-release and build suffixes.
func parseSemver(s string) (semver, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var v semver
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return semver{}, false
		}
		v[i] = n
	}
	return v, true
}

func compareSemver(a, b semver) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}
