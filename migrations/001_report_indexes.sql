-- 001: composite indexes for report overview queries.
-- Applied manually with psql (no migrate runner in this repo yet).
-- Safe to re-run (IF NOT EXISTS).

CREATE INDEX CONCURRENTLY IF NOT EXISTS leads_form_created
    ON leads (form_id, created_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS leads_form_status
    ON leads (form_id, status);
