package handler

import "github.com/sorolens/sorolens/apps/api/internal/store"

type percentileValuesResponse struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

type resourcePercentilesResponse struct {
	CPUInsn percentileValuesResponse `json:"cpu_insn"`
	MemByte percentileValuesResponse `json:"mem_byte"`
	Fee     percentileValuesResponse `json:"fee"`
}

// contractStatsDetailResponse is the stats endpoint's payload. The percentiles
// stay off contractStatsResponse because the contract summary embeds that type
// and does not compute them.
type contractStatsDetailResponse struct {
	contractStatsResponse
	ResourcePercentiles resourcePercentilesResponse `json:"resource_percentiles"`
}

func percentileValuesFromStore(v store.PercentileValues) percentileValuesResponse {
	return percentileValuesResponse{P50: v.P50, P95: v.P95, P99: v.P99}
}

func resourcePercentilesFromStore(rp store.ResourcePercentiles) resourcePercentilesResponse {
	return resourcePercentilesResponse{
		CPUInsn: percentileValuesFromStore(rp.CPUInsn),
		MemByte: percentileValuesFromStore(rp.MemByte),
		Fee:     percentileValuesFromStore(rp.Fee),
	}
}
