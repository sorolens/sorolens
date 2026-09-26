package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Verification statuses for a contract's source-verification record.
const (
	VerificationPending  = "pending"
	VerificationVerified = "verified"
	VerificationFailed   = "failed"
)

// Source kinds accepted by the verification pipeline.
const (
	VerificationSourceArchive = "archive"
	VerificationSourceGit     = "git"
)

// ContractVerificationStore is the read/write surface for source-verification
// records. Kept as its own interface so the API can be wired without pulling
// the build verifier into tests that do not need it, mirroring
// ContractUpgradeStore and HealthScoreStore.
type ContractVerificationStore interface {
	// UpsertContractVerification stores the latest verification record for a
	// contract, replacing any previous record for that contract.
	UpsertContractVerification(ctx context.Context, v ContractVerification) error

	// GetContractVerification returns the latest record for a contract, or
	// ErrNotFound when the contract has never been submitted for verification.
	GetContractVerification(ctx context.Context, contractID string) (ContractVerification, error)
}

// ---- postgres implementation ----------------------------------------------

func (s *postgresStore) UpsertContractVerification(ctx context.Context, v ContractVerification) error {
	diagnostics := v.Diagnostics
	if diagnostics == nil {
		diagnostics = []VerificationDiagnostic{}
	}
	raw, err := json.Marshal(diagnostics)
	if err != nil {
		return fmt.Errorf("marshal verification diagnostics: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO contract_verifications
			(contract_id, status, on_chain_hash, compiled_hash, matched,
			 source_kind, source_ref, source_digest, stellar_version,
			 rustc_version, cargo_version, diagnostics, build_log,
			 submitted_at, verified_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		ON CONFLICT (contract_id) DO UPDATE SET
			status          = EXCLUDED.status,
			on_chain_hash   = EXCLUDED.on_chain_hash,
			compiled_hash   = EXCLUDED.compiled_hash,
			matched         = EXCLUDED.matched,
			source_kind     = EXCLUDED.source_kind,
			source_ref      = EXCLUDED.source_ref,
			source_digest   = EXCLUDED.source_digest,
			stellar_version = EXCLUDED.stellar_version,
			rustc_version   = EXCLUDED.rustc_version,
			cargo_version   = EXCLUDED.cargo_version,
			diagnostics     = EXCLUDED.diagnostics,
			build_log       = EXCLUDED.build_log,
			verified_at     = EXCLUDED.verified_at,
			updated_at      = EXCLUDED.updated_at`,
		v.ContractID, v.Status, v.OnChainHash, v.CompiledHash, v.Matched,
		v.SourceKind, v.SourceRef, v.SourceDigest, v.StellarVersion,
		v.RustcVersion, v.CargoVersion, raw, v.BuildLog,
		v.SubmittedAt, v.VerifiedAt, v.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert contract verification: %w", err)
	}
	return nil
}

func (s *postgresStore) GetContractVerification(ctx context.Context, contractID string) (ContractVerification, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT contract_id, status, on_chain_hash, compiled_hash, matched,
		       source_kind, source_ref, source_digest, stellar_version,
		       rustc_version, cargo_version, diagnostics, build_log,
		       submitted_at, verified_at, updated_at
		FROM contract_verifications
		WHERE contract_id = $1`, contractID)

	var v ContractVerification
	var raw []byte
	err := row.Scan(
		&v.ContractID, &v.Status, &v.OnChainHash, &v.CompiledHash, &v.Matched,
		&v.SourceKind, &v.SourceRef, &v.SourceDigest, &v.StellarVersion,
		&v.RustcVersion, &v.CargoVersion, &raw, &v.BuildLog,
		&v.SubmittedAt, &v.VerifiedAt, &v.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractVerification{}, ErrNotFound
	}
	if err != nil {
		return ContractVerification{}, fmt.Errorf("get contract verification: %w", err)
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &v.Diagnostics); err != nil {
			return ContractVerification{}, fmt.Errorf("decode verification diagnostics: %w", err)
		}
	}
	return v, nil
}
