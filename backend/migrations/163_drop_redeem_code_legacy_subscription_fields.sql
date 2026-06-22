DROP INDEX IF EXISTS idx_redeem_codes_group_id;

ALTER TABLE redeem_codes
    DROP COLUMN IF EXISTS group_id,
    DROP COLUMN IF EXISTS validity_days;
