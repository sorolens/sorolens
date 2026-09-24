package healthscore

import "testing"

func TestCompute_AllHealthy(t *testing.T) {
	in := Inputs{
		HealthyChecks:    100,
		TotalChecks:      100,
		TotalInvocations: 50,
		Activity: []Activity{
			{Invocations: 10, CPU: 100, Fees: 50},
			{Invocations: 10, CPU: 100, Fees: 50},
		},
		TotalStorage:    10,
		ExpiringStorage: 0,
	}
	s := Compute(in)
	if s.Overall != 100 {
		t.Fatalf("overall = %d, want 100", s.Overall)
	}
	for name, got := range map[string]int32{
		"uptime":      s.Uptime,
		"error_rate":  s.ErrorRate,
		"performance": s.Performance,
		"storage_ttl": s.StorageTTL,
	} {
		if got != 100 {
			t.Errorf("%s = %d, want 100", name, got)
		}
	}
}

func TestCompute_UptimeReflectsCheckHistory(t *testing.T) {
	in := Inputs{HealthyChecks: 25, TotalChecks: 50}
	s := Compute(in)
	if s.Uptime != 50 {
		t.Fatalf("uptime = %d, want 50 for 25/50 healthy checks", s.Uptime)
	}
	// 0.40*50 (uptime) + 0.25*100 + 0.20*100 + 0.15*100 = 20+25+20+15 = 80.
	if s.Overall != 80 {
		t.Fatalf("overall = %d, want 80 (uptime drops to 50, others default to 100)", s.Overall)
	}
}

func TestCompute_UptimeFallsBackToWatchdogStatus(t *testing.T) {
	base := Inputs{}
	cases := []struct {
		status string
		want   int32
	}{
		{"Healthy", 100},
		{"Degraded", 50},
		{"Unresponsive", 10},
		{"", 100},
	}
	for _, c := range cases {
		base.WatchdogStatus = c.status
		if got := uptimeScore(base); got != c.want {
			t.Errorf("uptimeScore(%q) = %d, want %d", c.status, got, c.want)
		}
	}
}

func TestCompute_ErrorRateReflectsFailures(t *testing.T) {
	in := Inputs{TotalInvocations: 20, FailedInvocations: 5}
	s := Compute(in)
	if s.ErrorRate != 75 {
		t.Fatalf("error rate = %d, want 75 (15/20 succeeded)", s.ErrorRate)
	}
	in = Inputs{TotalInvocations: 20, FailedInvocations: 20}
	if got := errorRateScore(in); got != 0 {
		t.Errorf("error rate = %d, want 0 when all fail", got)
	}
	in = Inputs{}
	if got := errorRateScore(in); got != 100 {
		t.Errorf("error rate = %d, want 100 with no invocations", got)
	}
}

func TestCompute_PerformanceReflectsCostTrend(t *testing.T) {
	// Recent hour costs 4x the baseline -> linear decay hits the floor.
	in := Inputs{Activity: []Activity{
		{Invocations: 10, CPU: 100, Fees: 50},  // baseline: 15/inv
		{Invocations: 10, CPU: 100, Fees: 50},  // baseline: 15/inv
		{Invocations: 10, CPU: 400, Fees: 200}, // recent: 60/inv, ratio 4
	}}
	s := Compute(in)
	if s.Performance != 0 {
		t.Fatalf("performance = %d, want 0 for 4x cost spike", s.Performance)
	}

	// Recent cost at/under baseline -> 100.
	in.Activity[2] = Activity{Invocations: 10, CPU: 100, Fees: 50}
	if got := performanceScore(in); got != 100 {
		t.Errorf("performance = %d, want 100 when cost is flat", got)
	}
}

func TestCompute_StorageTTLReflectsHeadroom(t *testing.T) {
	in := Inputs{TotalStorage: 10, ExpiringStorage: 4}
	s := Compute(in)
	if s.StorageTTL != 60 {
		t.Fatalf("storage ttl = %d, want 60 (6/10 healthy)", s.StorageTTL)
	}
	in = Inputs{TotalStorage: 10, ExpiringStorage: 10}
	if got := storageTTLScore(in); got != 0 {
		t.Errorf("storage ttl = %d, want 0 when everything expires", got)
	}
	in = Inputs{}
	if got := storageTTLScore(in); got != 100 {
		t.Errorf("storage ttl = %d, want 100 with no storage", got)
	}
}

func TestCompute_WeightsSumTo100(t *testing.T) {
	if got := WeightUptime + WeightErrorRate + WeightPerf + WeightStorageTTL; got != 100 {
		t.Fatalf("weights sum to %d, want 100", got)
	}
}

func TestClampQ(t *testing.T) {
	if got := clampQ(-5); got != 0 {
		t.Errorf("clampQ(-5) = %d, want 0", got)
	}
	if got := clampQ(150); got != 100 {
		t.Errorf("clampQ(150) = %d, want 100", got)
	}
	if got := clampQ(42.4); got != 42 {
		t.Errorf("clampQ(42.4) = %d, want 42", got)
	}
}
