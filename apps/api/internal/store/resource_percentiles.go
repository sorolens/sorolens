package store

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// PercentileValues holds the p50, p95 and p99 of one resource metric.
type PercentileValues struct {
	P50 float64
	P95 float64
	P99 float64
}

// ResourcePercentiles summarises per-invocation resource usage over a time
// window. CPU is in instructions, memory in bytes and fee in stroops. Every
// value is zero when the window holds no invocations.
type ResourcePercentiles struct {
	CPUInsn PercentileValues
	MemByte PercentileValues
	Fee     PercentileValues
}

// GetContractResourcePercentiles computes every percentile in one aggregate
// query over the same window GetContractStats uses. percentile_cont ignores
// NULL metrics, so invocations missing a resource value do not skew the result.
func (s *postgresStore) GetContractResourcePercentiles(ctx context.Context, contractID, window string) (ResourcePercentiles, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY cpu_insn::float8), 0)::float8,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY cpu_insn::float8), 0)::float8,
			COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY cpu_insn::float8), 0)::float8,
			COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY mem_byte::float8), 0)::float8,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY mem_byte::float8), 0)::float8,
			COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY mem_byte::float8), 0)::float8,
			COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY resource_fee_charged::float8), 0)::float8,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY resource_fee_charged::float8), 0)::float8,
			COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY resource_fee_charged::float8), 0)::float8
		FROM invocations
		WHERE contract_id = $1 AND ledger_closed_at >= NOW() - $2::interval`,
		contractID, windowToInterval(window),
	)
	var rp ResourcePercentiles
	if err := row.Scan(
		&rp.CPUInsn.P50, &rp.CPUInsn.P95, &rp.CPUInsn.P99,
		&rp.MemByte.P50, &rp.MemByte.P95, &rp.MemByte.P99,
		&rp.Fee.P50, &rp.Fee.P95, &rp.Fee.P99,
	); err != nil {
		return ResourcePercentiles{}, fmt.Errorf("contract resource percentiles: %w", err)
	}
	return rp, nil
}

// GetContractResourcePercentiles mirrors the Postgres query, including
// percentile_cont's linear interpolation between neighbouring values.
func (m *MockStore) GetContractResourcePercentiles(_ context.Context, contractID, window string) (ResourcePercentiles, error) {
	if m.GetContractResourcePercentilesErr != nil {
		return ResourcePercentiles{}, m.GetContractResourcePercentilesErr
	}
	cutoff := time.Now().Add(-mockWindowDuration(window))
	var cpu, mem, fee []float64
	for _, inv := range m.invocations {
		if inv.ContractID != contractID || inv.LedgerClosedAt.Before(cutoff) {
			continue
		}
		cpu = append(cpu, float64(inv.CPUInsn))
		mem = append(mem, float64(inv.MemByte))
		fee = append(fee, float64(inv.ResourceFeeCharged))
	}
	return ResourcePercentiles{
		CPUInsn: percentileValues(cpu),
		MemByte: percentileValues(mem),
		Fee:     percentileValues(fee),
	}, nil
}

func mockWindowDuration(window string) time.Duration {
	switch window {
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

func percentileValues(values []float64) PercentileValues {
	if len(values) == 0 {
		return PercentileValues{}
	}
	sort.Float64s(values)
	return PercentileValues{
		P50: interpolatedPercentile(values, 0.50),
		P95: interpolatedPercentile(values, 0.95),
		P99: interpolatedPercentile(values, 0.99),
	}
}

// interpolatedPercentile expects sorted, non-empty values.
func interpolatedPercentile(sorted []float64, p float64) float64 {
	position := float64(len(sorted)-1) * p
	lower := int(position)
	if lower+1 >= len(sorted) {
		return sorted[lower]
	}
	fraction := position - float64(lower)
	return sorted[lower] + (sorted[lower+1]-sorted[lower])*fraction
}
