ALTER TABLE match_participants ADD COLUMN IF NOT EXISTS rating_mu_after DOUBLE PRECISION;
ALTER TABLE match_participants ADD COLUMN IF NOT EXISTS rating_phi_after DOUBLE PRECISION;
ALTER TABLE match_participants ADD COLUMN IF NOT EXISTS rating_sigma_after DOUBLE PRECISION;
