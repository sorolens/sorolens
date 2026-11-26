package resourcelimits_test

import (
	"context"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/resourcelimits"
)

func TestComputeUtilisation_FarFromLimits(t *testing.T) {
	limits := resourcelimits.DefaultLimits()
	usage := resourcelimits.ResourceUsage{
		CPUInstructions: 10_000_000, // 10%
		MemoryBytes:     4_000_000,  // ~9.5%
		LedgerReadByte:  20_000,     // 10%
		LedgerWriteByte: 10_000,     // 10%
	}

	u := resourcelimits.ComputeUtilisation(usage, limits, 0.85)
	if u.NearLimit {
		t.Errorf("expected NearLimit = false, got true (max ratio %.2f)", u.MaxRatio)
	}
	if u.MaxRatio > 0.15 {
		t.Errorf("expected MaxRatio ~ 0.10, got %.2f", u.MaxRatio)
	}
}

func TestComputeUtilisation_NearLimits(t *testing.T) {
	limits := resourcelimits.DefaultLimits()

	// 1. High CPU (90%)
	uCPU := resourcelimits.ComputeUtilisation(resourcelimits.ResourceUsage{
		CPUInstructions: 90_000_000,
		MemoryBytes:     10_000_000,
		LedgerReadByte:  10_000,
		LedgerWriteByte: 5_000,
	}, limits, 0.85)

	if !uCPU.NearLimit {
		t.Errorf("expected NearLimit = true for 90%% CPU, got false")
	}
	if uCPU.DominantResource != "cpu_instructions" {
		t.Errorf("expected dominant cpu_instructions, got %s", uCPU.DominantResource)
	}

	// 2. High Memory (88%)
	uMem := resourcelimits.ComputeUtilisation(resourcelimits.ResourceUsage{
		CPUInstructions: 10_000_000,
		MemoryBytes:     int64(float64(limits.MaxMemoryBytes) * 0.88),
		LedgerReadByte:  10_000,
		LedgerWriteByte: 5_000,
	}, limits, 0.85)

	if !uMem.NearLimit {
		t.Errorf("expected NearLimit = true for 88%% memory, got false")
	}
	if uMem.DominantResource != "memory_bytes" {
		t.Errorf("expected dominant memory_bytes, got %s", uMem.DominantResource)
	}

	// 3. High Ledger Write Byte (96% -> critical)
	uWrite := resourcelimits.ComputeUtilisation(resourcelimits.ResourceUsage{
		CPUInstructions: 10_000_000,
		MemoryBytes:     1_000_000,
		LedgerReadByte:  10_000,
		LedgerWriteByte: 96_000,
	}, limits, 0.85)

	if !uWrite.NearLimit {
		t.Errorf("expected NearLimit = true for 96%% write bytes, got false")
	}
	if uWrite.DominantResource != "ledger_write_byte" {
		t.Errorf("expected dominant ledger_write_byte, got %s", uWrite.DominantResource)
	}
}

func TestDetector_AlertTriggering(t *testing.T) {
	cache := resourcelimits.NewLimitsCache()
	det := resourcelimits.NewDetector(cache, 0.85)
	ctx := context.Background()

	// Far invocation: no alert
	farUsage := resourcelimits.ResourceUsage{
		CPUInstructions: 50_000_000, // 50%
	}
	alert, u := det.CheckInvocation(ctx, "CB1", "tx1", "testnet", farUsage, time.Now())
	if alert != nil {
		t.Errorf("expected no alert, got %v", alert)
	}
	if u.NearLimit {
		t.Errorf("expected NearLimit = false")
	}

	// Near invocation (88%): Warning alert
	nearUsage := resourcelimits.ResourceUsage{
		CPUInstructions: 88_000_000,
	}
	alert2, u2 := det.CheckInvocation(ctx, "CB1", "tx2", "testnet", nearUsage, time.Now())
	if alert2 == nil {
		t.Fatalf("expected alert, got nil")
	}
	if alert2.Severity != "Warning" {
		t.Errorf("expected Severity Warning, got %s", alert2.Severity)
	}
	if alert2.Rule != "resource_limit_near_miss" {
		t.Errorf("expected rule resource_limit_near_miss, got %s", alert2.Rule)
	}
	if !u2.NearLimit {
		t.Errorf("expected NearLimit = true")
	}

	// Critical invocation (98%): Critical alert
	critUsage := resourcelimits.ResourceUsage{
		CPUInstructions: 98_000_000,
	}
	alert3, _ := det.CheckInvocation(ctx, "CB1", "tx3", "testnet", critUsage, time.Now())
	if alert3 == nil {
		t.Fatalf("expected alert, got nil")
	}
	if alert3.Severity != "Critical" {
		t.Errorf("expected Severity Critical, got %s", alert3.Severity)
	}
}

func TestDynamicLimitUpdatesWithoutRestart(t *testing.T) {
	cache := resourcelimits.NewLimitsCache()
	det := resourcelimits.NewDetector(cache, 0.85)
	ctx := context.Background()

	usage := resourcelimits.ResourceUsage{
		CPUInstructions: 70_000_000, // 70% of 100M -> below 85%
	}

	// Initial check: below limit
	alert, u := det.CheckInvocation(ctx, "CB1", "tx1", "testnet", usage, time.Now())
	if alert != nil || u.NearLimit {
		t.Fatalf("expected no alert under baseline limits")
	}

	// Network upgrade reduces MaxCPUInstructions to 80M without restart
	newLimits := resourcelimits.NetworkLimits{
		MaxCPUInstructions: 80_000_000,
		MaxMemoryBytes:     resourcelimits.DefaultMaxMemoryBytes,
		MaxLedgerReadByte:  resourcelimits.DefaultMaxLedgerReadByte,
		MaxLedgerWriteByte: resourcelimits.DefaultMaxLedgerWriteByte,
	}
	cache.SetLimits("testnet", newLimits)

	// Now 70M / 80M = 87.5% -> crosses 85% threshold!
	alert2, u2 := det.CheckInvocation(ctx, "CB1", "tx2", "testnet", usage, time.Now())
	if alert2 == nil {
		t.Fatalf("expected alert after dynamic limit update, got nil")
	}
	if !u2.NearLimit {
		t.Errorf("expected NearLimit = true after dynamic limit update")
	}
	if alert2.Severity != "Warning" {
		t.Errorf("expected Warning, got %s", alert2.Severity)
	}
}
