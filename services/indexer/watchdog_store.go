package main

import (
	"context"
	"time"
)

// watchdogStore is the write surface the interceptor needs, mirroring
// store.WatchdogStore from apps/api/internal/store.
type watchdogStore interface {
	UpsertMonitoredContract(ctx context.Context, m wdMonitoredContract) error
	DeleteMonitoredContract(ctx context.Context, contractID string) error
	InsertHealthCheck(ctx context.Context, h wdHealthCheck) error
	InsertContractAlert(ctx context.Context, a wdContractAlert) error
}

type wdMonitoredContract struct {
	ContractID    string
	Network       string
	Name          string
	Owner         string
	Status        string
	LastCheck     *time.Time
	CheckInterval int64
	RegisteredAt  time.Time
	UpdatedAt     time.Time
}

type wdHealthCheck struct {
	ContractID string
	Status     string
	Metadata   string
	Ledger     int64
	TxHash     string
	Timestamp  time.Time
}

type wdContractAlert struct {
	ContractID string
	Severity   string
	Message    string
	Ledger     int64
	TxHash     string
	Timestamp  time.Time
}
