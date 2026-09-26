package rules

import (
	"strings"
	"testing"
	"time"
)

func TestParseValid(t *testing.T) {
	cases := []struct {
		expr string
		want Rule
	}{
		{
			expr: "avg(fee) > 0.5 XLM for 5m on CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
			want: Rule{
				Aggregation: AggAvg, Metric: MetricFee, Comparison: CmpGt,
				Threshold: 0.5, Unit: UnitXLM, Window: 5 * time.Minute,
				Contract: "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
			},
		},
		{
			expr: "rate(events) > 100 for 1h",
			want: Rule{
				Aggregation: AggRate, Metric: MetricEvents, Comparison: CmpGt,
				Threshold: 100, Unit: "", Window: time.Hour,
			},
		},
		{
			expr: "fee > 0.5 XLM for 5m",
			want: Rule{
				Aggregation: AggAvg, Metric: MetricFee, Comparison: CmpGt,
				Threshold: 0.5, Unit: UnitXLM, Window: 5 * time.Minute,
			},
		},
		{
			expr: "events > 10",
			want: Rule{
				Aggregation: AggRate, Metric: MetricEvents, Comparison: CmpGt,
				Threshold: 10, Unit: "", Window: DefaultWindow,
			},
		},
		{
			expr: "max(cpu) >= 200000000 for 24h",
			want: Rule{
				Aggregation: AggMax, Metric: MetricCPU, Comparison: CmpGte,
				Threshold: 200000000, Unit: UnitInsn, Window: 24 * time.Hour,
			},
		},
		{
			expr: "max(cpu) >= 200000000 insn",
			want: Rule{
				Aggregation: AggMax, Metric: MetricCPU, Comparison: CmpGte,
				Threshold: 200000000, Unit: UnitInsn, Window: DefaultWindow,
			},
		},
		{
			expr: "rate(failures) > 0.05",
			want: Rule{
				Aggregation: AggRate, Metric: MetricFailures, Comparison: CmpGt,
				Threshold: 0.05, Unit: "", Window: DefaultWindow,
			},
		},
		{
			expr: "avg(ledger_bytes) > 1048576 bytes for 1h30m",
			want: Rule{
				Aggregation: AggAvg, Metric: MetricLedgerBytes, Comparison: CmpGt,
				Threshold: 1048576, Unit: UnitBytes, Window: 90 * time.Minute,
			},
		},
		{
			expr: "MAX(FEE) <= 1000000 STROOPS for 5m on *",
			want: Rule{
				Aggregation: AggMax, Metric: MetricFee, Comparison: CmpLte,
				Threshold: 1000000, Unit: UnitStroops, Window: 5 * time.Minute,
			},
		},
		{
			expr: "avg(fee) = 5",
			want: Rule{
				Aggregation: AggAvg, Metric: MetricFee, Comparison: CmpEq,
				Threshold: 5, Unit: UnitStroops, Window: DefaultWindow,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := ParseExpression(tc.expr)
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			if got != tc.want {
				t.Errorf("rule mismatch:\n got: %+v\nwant: %+v", got, tc.want)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	cases := []struct {
		name string
		expr string
		want string
	}{
		{"empty", "", "expected"},
		{"garbage", "hello world", "unknown metric"},
		{"unknown agg", "sum(fee) > 5", "unknown aggregation"},
		{"missing paren metric", "avg fee > 5", "expected '('"},
		{"unclosed paren", "avg(fee > 5", "expected ')'"},
		{"missing threshold", "avg(fee) >", "expected a numeric threshold"},
		{"trailing garbage", "avg(fee) > 5 banana", "unknown unit"},
		{"double condition", "avg(fee) > 5 and max(cpu) > 3", "only one condition"},
		{"bad unit", "rate(events) > 5 XLM", "unit"},
		{"bad contract", "avg(fee) > 5 on C123", "not a valid contract"},
		{"no data window", "avg(fee) > 5 for 0s", "between 1s and 90d"},
		{"max invocation", "max(invocations) > 5", "not meaningful"},
		{"avg invocation", "avg(invocations) > 5", "meaningless"},
		{"empty agg args", "avg() > 5", "expected a metric"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseExpression(tc.expr)
			if err == nil {
				t.Fatalf("expected error for %q, got rule %+v", tc.expr, got)
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestParseErrorHasPosition(t *testing.T) {
	_, err := ParseExpression("avg(fee) > 5 for 3x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "line 1, column") {
		t.Errorf("error missing position: %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	exprs := []string{
		"avg(fee) > 0.5 XLM for 5m",
		"rate(events) > 100 for 1h",
		"max(cpu) > 200000000 for 24h",
		"rate(failures) > 0.05 for 5m",
		"avg(ledger_bytes) > 1048576 for 1h",
	}
	for _, expr := range exprs {
		r, err := ParseExpression(expr)
		if err != nil {
			t.Fatalf("parse %q: %v", expr, err)
		}
		canonical := r.String()
		r2, err := ParseExpression(canonical)
		if err != nil {
			t.Fatalf("reparse %q: %v", canonical, err)
		}
		if r != r2 {
			t.Errorf("round-trip mismatch for %q:\n1: %+v\n2: %+v", expr, r, r2)
		}
	}
}

func TestEvaluateFires(t *testing.T) {
	stats := WindowStats{
		Duration: 5 * time.Minute,
		Invocations: []InvocationSample{
			{Status: "SUCCESS", FeeStroops: 6_000_000, CPUInsn: 1_000_000, MemBytes: 100, LedgerBytes: 10},
			{Status: "SUCCESS", FeeStroops: 4_000_000, CPUInsn: 2_000_000, MemBytes: 100, LedgerBytes: 10},
			{Status: "FAILED", FeeStroops: 8_000_000, CPUInsn: 4_000_000, MemBytes: 100, LedgerBytes: 10},
		},
		Events: 9,
	}

	r := Rule{Aggregation: AggAvg, Metric: MetricFee, Comparison: CmpGt, Threshold: 0.5, Unit: UnitXLM, Window: 5 * time.Minute}
	res := Evaluate(r, stats)
	if !res.Fired {
		t.Fatalf("expected avg fee (0.6 XLM) > 0.5 XLM to fire, got %+v", res)
	}
	if res.Value != 0.6 {
		t.Errorf("value = %v, want 0.6", res.Value)
	}
	if res.Samples != 3 {
		t.Errorf("samples = %d, want 3", res.Samples)
	}

	// max fee in stroops
	rm := Rule{Aggregation: AggMax, Metric: MetricFee, Comparison: CmpGt, Threshold: 7_000_000, Unit: UnitStroops, Window: 5 * time.Minute}
	resm := Evaluate(rm, stats)
	if !resm.Fired || resm.Value != 8_000_000 {
		t.Errorf("max fee: got %+v, want fired with value 8000000", resm)
	}

	// rate events: 9 events / 300s = 0.03/sec
	re := Rule{Aggregation: AggRate, Metric: MetricEvents, Comparison: CmpGt, Threshold: 0.02, Window: 5 * time.Minute}
	rese := Evaluate(re, stats)
	if !rese.Fired || rese.Value != 0.03 {
		t.Errorf("rate events: got %+v, want fired value 0.03", rese)
	}

	// failure rate: 1/3 = 0.333 above 0.05
	rf := Rule{Aggregation: AggRate, Metric: MetricFailures, Comparison: CmpGt, Threshold: 0.05, Window: 5 * time.Minute}
	resf := Evaluate(rf, stats)
	if !resf.Fired || resf.Value != 1.0/3.0 {
		t.Errorf("failure rate: got %+v", resf)
	}
}

func TestEvaluateEmptyWindowNeverFires(t *testing.T) {
	r := Rule{Aggregation: AggAvg, Metric: MetricFee, Comparison: CmpGte, Threshold: 0, Unit: UnitStroops, Window: 5 * time.Minute}
	res := Evaluate(r, WindowStats{Duration: 5 * time.Minute})
	if res.Fired {
		t.Errorf("empty window must not fire, got %+v", res)
	}
	if !res.NoData {
		t.Errorf("empty window should be flagged NoData")
	}
}

func TestEvaluateNotFired(t *testing.T) {
	stats := WindowStats{
		Duration:    5 * time.Minute,
		Invocations: []InvocationSample{{Status: "SUCCESS", FeeStroops: 1_000_000}},
		Events:      1,
	}
	r := Rule{Aggregation: AggAvg, Metric: MetricFee, Comparison: CmpGt, Threshold: 0.5, Unit: UnitXLM, Window: 5 * time.Minute}
	res := Evaluate(r, stats)
	if res.Fired {
		t.Errorf("0.1 XLM fee should not exceed 0.5 XLM: %+v", res)
	}
	if res.Value != 0.1 {
		t.Errorf("value = %v, want 0.1", res.Value)
	}
}

func TestSamplesAllParse(t *testing.T) {
	for _, s := range Samples {
		if _, err := ParseExpression(s.Expression); err != nil {
			t.Errorf("sample %q does not parse: %v", s.Name, err)
		}
	}
}