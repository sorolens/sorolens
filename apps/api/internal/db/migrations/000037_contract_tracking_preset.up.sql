-- ============================================================
-- 000037_contract_tracking_preset
--
-- Records the contract-type preset ("dex", "lending", "stablecoin", ...)
-- that was applied when a contract was tracked (issue #564). The API
-- writes it through UpsertContract and reads it back in GetContract and
-- ListContracts, and the CLI prints it after `sorolens track --preset`.
--
-- Presets are open-ended: contributors register new types in
-- cli/internal/presets without touching the schema, so this is a plain
-- TEXT column rather than an enum or a CHECK constraint.
--
-- NOT NULL DEFAULT '' rather than nullable: the Go struct field is a
-- plain string, so a NULL would fail to scan. '' means "no preset",
-- which is exactly what the zero value means everywhere else.
-- ============================================================

ALTER TABLE contracts
    ADD COLUMN tracking_preset TEXT NOT NULL DEFAULT '';
