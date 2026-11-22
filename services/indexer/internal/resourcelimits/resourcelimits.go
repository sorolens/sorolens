package resourcelimits

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Default limits for Soroban network resources.
const (
	DefaultMaxCPUInstructions int64 = 100_000_000
	DefaultMaxMemoryBytes     int64 = 41_943_040 // 40 MB
	DefaultMaxLedgerReadByte  int64 = 200_000    // 200 KB
	DefaultMaxLedgerWriteByte int64 = 100_000    // 100 KB

	DefaultThreshold float64 = 0.85 // 85% utilisation threshold
)

// NetworkLimits holds the resource budget limits for a network.
type NetworkLimits struct {
	MaxCPUInstructions int64 `json:"max_cpu_instructions"`
	MaxMemoryBytes     int64 `json:"max_memory_bytes"`
	MaxLedgerReadByte  int64 `json:"max_ledger_read_byte"`
	MaxLedgerWriteByte int64 `json:"max_ledger_write_byte"`
}

// DefaultLimits returns the baseline Soroban limits.
func DefaultLimits() NetworkLimits {
	return NetworkLimits{
		MaxCPUInstructions: DefaultMaxCPUInstructions,
		MaxMemoryBytes:     DefaultMaxMemoryBytes,
		MaxLedgerReadByte:  DefaultMaxLedgerReadByte,
		MaxLedgerWriteByte: DefaultMaxLedgerWriteByte,
	}
}

// LimitsCache is a thread-safe cache allowing dynamic limit updates without restart.
type LimitsCache struct {
	mu     sync.RWMutex
	limits map[string]NetworkLimits
}

// NewLimitsCache returns a new LimitsCache initialized with default limits.
func NewLimitsCache() *LimitsCache {
	return &LimitsCache{
		limits: make(map[string]NetworkLimits),
	}
}

// GetLimits returns limits for the given network, falling back to defaults if not set.
func (c *LimitsCache) GetLimits(network string) NetworkLimits {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if lim, ok := c.limits[network]; ok {
		return lim
	}
	if def, ok := c.limits["default"]; ok {
		return def
	}
	return DefaultLimits()
}

// SetLimits updates the limits for a network dynamically without restarting.
func (c *LimitsCache) SetLimits(network string, limits NetworkLimits) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.limits[network] = limits
}

// ResourceUsage represents the resource footprint of an invocation.
type ResourceUsage struct {
	CPUInstructions int64
	MemoryBytes     int64
	LedgerReadByte  int64
	LedgerWriteByte int64
}

// Utilisation represents calculated resource consumption ratios.
type Utilisation struct {
	CPUInstructionsRatio float64 `json:"cpu_instructions_ratio"`
	MemoryBytesRatio     float64 `json:"memory_bytes_ratio"`
	LedgerReadByteRatio  float64 `json:"ledger_read_byte_ratio"`
	LedgerWriteByteRatio float64 `json:"ledger_write_byte_ratio"`
	MaxRatio             float64 `json:"max_ratio"`
	NearLimit            bool    `json:"near_limit"`
	DominantResource     string  `json:"dominant_resource"`
}

// ComputeUtilisation evaluates resource usage against the given network limits.
func ComputeUtilisation(usage ResourceUsage, limits NetworkLimits, threshold float64) Utilisation {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}

	maxCPU := limits.MaxCPUInstructions
	if maxCPU <= 0 {
		maxCPU = DefaultMaxCPUInstructions
	}
	maxMem := limits.MaxMemoryBytes
	if maxMem <= 0 {
		maxMem = DefaultMaxMemoryBytes
	}
	maxRead := limits.MaxLedgerReadByte
	if maxRead <= 0 {
		maxRead = DefaultMaxLedgerReadByte
	}
	maxWrite := limits.MaxLedgerWriteByte
	if maxWrite <= 0 {
		maxWrite = DefaultMaxLedgerWriteByte
	}

	cpuRatio := float64(usage.CPUInstructions) / float64(maxCPU)
	memRatio := float64(usage.MemoryBytes) / float64(maxMem)
	readRatio := float64(usage.LedgerReadByte) / float64(maxRead)
	writeRatio := float64(usage.LedgerWriteByte) / float64(maxWrite)

	maxRatio := cpuRatio
	dominant := "cpu_instructions"

	if memRatio > maxRatio {
		maxRatio = memRatio
		dominant = "memory_bytes"
	}
	if readRatio > maxRatio {
		maxRatio = readRatio
		dominant = "ledger_read_byte"
	}
	if writeRatio > maxRatio {
		maxRatio = writeRatio
		dominant = "ledger_write_byte"
	}

	return Utilisation{
		CPUInstructionsRatio: cpuRatio,
		MemoryBytesRatio:     memRatio,
		LedgerReadByteRatio:  readRatio,
		LedgerWriteByteRatio: writeRatio,
		MaxRatio:             maxRatio,
		NearLimit:            maxRatio >= threshold,
		DominantResource:     dominant,
	}
}

// Alert is emitted when an invocation comes near the resource limit.
type Alert struct {
	ContractID string
	TxHash     string
	Severity   string // Warning or Critical
	Rule       string // "resource_limit_near_miss"
	Message    string
	Timestamp  time.Time
}

// Detector detects invocations nearing limits.
type Detector struct {
	cache     *LimitsCache
	threshold float64
}

// NewDetector creates a detector with the given cache and threshold.
func NewDetector(cache *LimitsCache, threshold float64) *Detector {
	if cache == nil {
		cache = NewLimitsCache()
	}
	if threshold <= 0 {
		threshold = DefaultThreshold
	}
	return &Detector{
		cache:     cache,
		threshold: threshold,
	}
}

// CheckInvocation evaluates an invocation and returns an Alert if utilisation >= threshold.
func (d *Detector) CheckInvocation(ctx context.Context, contractID, txHash, network string, usage ResourceUsage, ts time.Time) (*Alert, Utilisation) {
	limits := d.cache.GetLimits(network)
	util := ComputeUtilisation(usage, limits, d.threshold)

	if !util.NearLimit {
		return nil, util
	}

	severity := "Warning"
	if util.MaxRatio >= 0.95 {
		severity = "Critical"
	}

	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	msg := fmt.Sprintf(
		"resource limit near-miss: invocation %s on contract %s reached %.1f%% of network %s limit (dominant: %s)",
		txHash, contractID, util.MaxRatio*100, network, util.DominantResource,
	)

	return &Alert{
		ContractID: contractID,
		TxHash:     txHash,
		Severity:   severity,
		Rule:       "resource_limit_near_miss",
		Message:    msg,
		Timestamp:  ts,
	}, util
}
