-- Claim fencing is introduced after the original stale-row migration so
-- already-deployed databases can upgrade without editing migration 67.
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS judging_claim_token UUID;
