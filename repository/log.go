package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/model"
)

// LogRepository appends system_logs rows; failures are swallowed
// by the caller like the Node LogService.
type LogRepository struct {
	pool *pgxpool.Pool
}

func NewLogRepository(pool *pgxpool.Pool) *LogRepository {
	return &LogRepository{pool: pool}
}

// Insert writes a log row; err mirrors the Node catch-and-log behavior
// (the service layer decides to ignore it).
func (r *LogRepository) Insert(ctx context.Context, userID *string, action string, entityType, entityID, description *string, metadata []byte, ip *string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO system_logs
		(id, user_id, action, entity_type, entity_id, description, metadata, ip_address, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $8)`,
		userID, action, entityType, entityID, description, nullJSON(metadata), ip, nowUTC())
	return err
}

const logColumns = `id, user_id, action, entity_type, entity_id,
	description, metadata, ip_address, created_at, updated_at`

// List returns logs newest-first, optionally filtered by a search
// substring over action/description (case-insensitive, like Op.iLike).
func (r *LogRepository) List(ctx context.Context, search string, limit, offset int) ([]model.SystemLog, error) {
	q := `SELECT ` + logColumns + ` FROM system_logs`
	var args []any
	if search != "" {
		args = append(args, "%"+search+"%")
		q += ` WHERE action ILIKE $1 OR description ILIKE $1`
	}
	args = append(args, limit, offset)
	q += ` ORDER BY created_at DESC LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.SystemLog])
}
