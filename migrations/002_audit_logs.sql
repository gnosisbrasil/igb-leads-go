-- 002: audit lookups reuse system_logs (see LogRepository.ByEntity).
-- Replaces the standalone audit_logs draft (dropped below; it was
-- created empty and never written).
-- Applied manually with psql (no migrate runner in this repo yet).

DROP TABLE IF EXISTS audit_logs;

CREATE INDEX IF NOT EXISTS system_logs_entity_created
    ON system_logs (entity_type, entity_id, created_at DESC);
