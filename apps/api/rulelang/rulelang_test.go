package rulelang

import (
	"encoding/base32"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseValidRules(t *testing.T) {
	payload := make([]byte, 32)
	for i := range payload {
		payload[i] = 0x02
	}
	validID := encodeContractID(payload)

	tests := []struct {
		source   string
		contract string
		network  string
		window   time.Duration
	}{
		{"error_rate > 5%", "", "", 0},
		{"fee_per_invocation > 0.5 XLM for 5m", "", "", 5 * time.Minute},
		{"avg(cpu_insn_per_invocation) > 5000000 for 30m on contract " + validID, validID, "", 30 * time.Minute},
		{"rate(events) < 1 for 15m on network testnet", "", "testnet", 15 * time.Minute},
		{"health_score >= 80", "", "", 0},
		{"(error_rate) > 0.1", "", "", 0},
	}
	for _, tc := range tests {
		r, err := Validate(tc.source)
		if err != nil {
			t.Fatalf("Validate(%q) = %v", tc.source, err)
		}
		if r.ContractID != tc.contract {
			t.Errorf("%q contract = %q, want %q", tc.source, r.ContractID, tc.contract)
		}
		if r.Network != tc.network {
			t.Errorf("%q network = %q, want %q", tc.source, r.Network, tc.network)
		}
		if r.For != tc.window {
			t.Errorf("%q window = %v, want %v", tc.source, r.For, tc.window)
		}
	}
}

func TestParseErrorsAreFriendly(t *testing.T) {
	tests := []struct {
		source string
		want   string // substring of the message
		hint   string // substring of the hint
	}{
		{"error_rate", "expected a comparison operator", "error_rate > 0.05"},
		{"error_rate >", "expected an expression", ""},
		{"eror_rate > 0.1", "unknown metric", "did you mean"},
		{"error_rate > 0.1 for", "expected a duration", "30s, 5m"},
		{"error_rate > 0.1 for 5", "expected a duration", ""},
		{"error_rate > 0.1 for 1x", "unknown duration unit", ""},
		{"error_rate > 0.1 on widget", "expected `contract` or `network`", ""},
		{"error_rate > 0.1 xlm", "does not match", ""},
		{"avg > 1", "aggregation", "avg(metric)"},
		{"avg() > 1", "expected a metric name", ""},
		{"error_rate > 0.1 for 5m on contract notacontract", "invalid contract id", ""},
		{"error_rate > 0.1 on network solana", "unknown network", ""},
		{"error_rate = 0.1", "`=` is not a comparison", "=="},
		{"invocations > 1 for 1s", "window 1s is too short", ""},
		{"error_rate > (failed_invocations > 1)", "cannot compare against another comparison", ""},
	}
	for _, tc := range tests {
		_, err := Validate(tc.source)
		if err == nil {
			t.Errorf("Validate(%q) unexpectedly succeeded", tc.source)
			continue
		}
		var rerr *Error
		if !errors.As(err, &rerr) {
			t.Errorf("Validate(%q) error is %T, want *Error", tc.source, err)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Validate(%q) error = %q, want substring %q", tc.source, err.Error(), tc.want)
		}
		if tc.hint != "" && !strings.Contains(rerr.Hint, tc.hint) {
			t.Errorf("Validate(%q) hint = %q, want substring %q", tc.source, rerr.Hint, tc.hint)
		}
	}
}

func TestUnitMismatch(t *testing.T) {
	_, err := Validate("error_rate > 0.5 XLM")
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected unit mismatch, got %v", err)
	}
	if _, err := Validate("fee_per_invocation > 0.5 XLM"); err != nil {
		t.Fatalf("matching unit rejected: %v", err)
	}
}

func TestPercentLiteral(t *testing.T) {
	r, err := Validate("error_rate > 5%")
	if err != nil {
		t.Fatal(err)
	}
	cmp := r.Expr.(*Comparison)
	lit := cmp.Right.(*NumberLiteral)
	if lit.Value != 0.05 {
		t.Fatalf("5%% = %v, want 0.05", lit.Value)
	}
}

func TestFormatRoundTrip(t *testing.T) {
	source := "avg(cpu_insn_per_invocation) > 5000000 for 30m on network testnet"
	r, err := Validate(source)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Format(); got != source {
		t.Fatalf("Format() = %q, want %q", got, source)
	}
}

func evalWindow(values ...float64) Window {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Duration(len(values)) * time.Minute)
	w := Window{From: start, To: now}
	for i, v := range values {
		w.Samples = append(w.Samples, Sample{
			At:     start.Add(time.Duration(i) * time.Minute),
			Values: map[string]float64{"error_rate": v, "fee_per_invocation": v, "events": v},
		})
	}
	return w
}

func TestEvaluateBareMetricUsesLatestSample(t *testing.T) {
	r, _ := Validate("error_rate > 0.5")
	res, err := Evaluate(r, evalWindow(0.1, 0.2, 0.9))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Fired || res.Value != 0.9 {
		t.Fatalf("Fired=%v Value=%v, want true/0.9", res.Fired, res.Value)
	}
	if len(res.Points) != 1 {
		t.Fatalf("Points = %d, want 1", len(res.Points))
	}
}

func TestEvaluateWindowRequiresEverySample(t *testing.T) {
	r, _ := Validate("error_rate > 0.5 for 3m")
	// Three buckets in the window, one of which is below the threshold.
	res, err := Evaluate(r, evalWindow(0.9, 0.1, 0.9))
	if err != nil {
		t.Fatal(err)
	}
	if res.Fired {
		t.Fatalf("expected not fired: %s", res.Reason)
	}
	if len(res.Points) != 3 {
		t.Fatalf("Points = %d, want 3", len(res.Points))
	}

	r2, _ := Validate("error_rate > 0.5 for 3m")
	res2, _ := Evaluate(r2, evalWindow(0.9, 0.8, 0.7))
	if !res2.Fired {
		t.Fatalf("expected fired: %s", res2.Reason)
	}
}

func TestAggregations(t *testing.T) {
	tests := []struct {
		source string
		values []float64
		fired  bool
		value  float64
	}{
		{"avg(error_rate) > 0.5 for 3m", []float64{0.1, 0.2, 0.9}, false, 0.4},
		{"max(error_rate) > 0.5 for 3m", []float64{0.1, 0.2, 0.9}, true, 0.9},
		{"min(error_rate) < 0.15 for 3m", []float64{0.1, 0.2, 0.9}, true, 0.1},
		{"count(error_rate) > 2 for 3m", []float64{0.1, 0.2, 0.9}, true, 3},
	}
	for _, tc := range tests {
		r, err := Validate(tc.source)
		if err != nil {
			t.Fatalf("Validate(%q): %v", tc.source, err)
		}
		res, err := Evaluate(r, evalWindow(tc.values...))
		if err != nil {
			t.Fatalf("Evaluate(%q): %v", tc.source, err)
		}
		if res.Fired != tc.fired {
			t.Errorf("%q Fired = %v, want %v", tc.source, res.Fired, tc.fired)
			continue
		}
		if diff := res.Value - tc.value; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("%q Value = %v, want %v", tc.source, res.Value, tc.value)
		}
	}
}

func TestRateAggregation(t *testing.T) {
	// 4 samples spanning 3 minutes, values summing to 120 -> 120/180 = 0.667/s.
	r, _ := Validate("rate(events) > 0.5 for 4m")
	res, err := Evaluate(r, evalWindow(10, 20, 30, 60))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Fired {
		t.Fatalf("expected fired, got %s (value %v)", res.Reason, res.Value)
	}
}

func TestMissingMetricIsReported(t *testing.T) {
	r, _ := Validate("health_score > 90 for 5m")
	res, err := Evaluate(r, evalWindow(1, 2, 3))
	if err != nil {
		t.Fatal(err)
	}
	if res.Fired || res.HasValue {
		t.Fatalf("expected no verdict, got %+v", res)
	}
	if !strings.Contains(res.Reason, "health_score") {
		t.Fatalf("reason = %q, want it to mention health_score", res.Reason)
	}
}

func TestEvaluateSource(t *testing.T) {
	res, rule, err := EvaluateSource("error_rate > 0.5", evalWindow(0.9))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Fired {
		t.Fatal("expected fired")
	}
	if rule.For != 0 {
		t.Fatalf("window = %v, want 0", rule.For)
	}
}

func TestLibraryRulesAreValid(t *testing.T) {
	lib := Library()
	if len(lib) == 0 {
		t.Fatal("library is empty")
	}
	for _, lr := range lib {
		r, err := Validate(lr.Source)
		if err != nil {
			t.Errorf("library rule %q (%q) does not validate: %v", lr.Name, lr.Source, err)
			continue
		}
		switch lr.Severity {
		case "Info", "Warning", "Critical":
		default:
			t.Errorf("library rule %q has unknown severity %q", lr.Name, lr.Severity)
		}
		// Every library rule must survive a format round trip.
		if got := r.Format(); got != lr.Source {
			t.Errorf("library rule %q formats as %q, want %q", lr.Name, got, lr.Source)
		}
	}
}

func TestCatalogIsSortedAndUnique(t *testing.T) {
	names := Metrics()
	seen := map[string]bool{}
	for i, n := range names {
		if seen[n] {
			t.Fatalf("duplicate metric %q", n)
		}
		seen[n] = true
		if i > 0 && names[i-1] > n {
			t.Fatalf("metrics not sorted: %q before %q", names[i-1], n)
		}
	}
}

// encodeContractID builds a valid Soroban StrKey from a 32-byte payload,
// mirroring stellar-core's encoding: version byte, payload, CRC16-XModem
// little-endian, base32 without padding.
func encodeContractID(payload []byte) string {
	raw := append([]byte{contractVersionByte}, payload...)
	crc := crc16XModem(raw)
	raw = append(raw, byte(crc), byte(crc>>8))
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
}

func TestValidContractID(t *testing.T) {
	payload := make([]byte, 32)
	for i := range payload {
		payload[i] = 0x01
	}
	good := encodeContractID(payload)
	if !ValidContractID(good) {
		t.Fatalf("%s should be valid", good)
	}
	// Flip the first character to the account version byte, and corrupt the
	// final checksum character.
	badVersion := "G" + good[1:]
	last := good[len(good)-1]
	repl := byte('A')
	if last == 'A' {
		repl = 'B'
	}
	badChecksum := good[:len(good)-1] + string(repl)
	for _, bad := range []string{"", badVersion, badChecksum, good[:55]} {
		if ValidContractID(bad) {
			t.Fatalf("%q should be invalid", bad)
		}
	}
}
