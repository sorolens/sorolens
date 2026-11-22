-- Rollback fee breakdown columns (issue #805).
ALTER TABLE invocations DROP COLUMN IF EXISTS inclusion_fee;
ALTER TABLE invocations DROP COLUMN IF EXISTS resource_fee;
ALTER TABLE invocations DROP COLUMN IF EXISTS resource_fee_refundable;
ALTER TABLE invocations DROP COLUMN IF EXISTS fee_refund;
