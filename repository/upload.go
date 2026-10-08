package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/model"
)

// UploadRepository covers the uploads table.
type UploadRepository struct {
	pool *pgxpool.Pool
}

func NewUploadRepository(pool *pgxpool.Pool) *UploadRepository {
	return &UploadRepository{pool: pool}
}

const uploadColumns = `id, file_name, original_name, file_path, file_size,
	mime_type, uploaded_by, entity_type, entity_id, created_at, updated_at`

func (r *UploadRepository) Create(ctx context.Context, u *model.Upload) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO uploads
		(id, file_name, original_name, file_path, file_size, mime_type,
		 uploaded_by, entity_type, entity_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		u.ID, u.FileName, u.OriginalName, u.FilePath, u.FileSize, u.MimeType,
		u.UploadedBy, u.EntityType, u.EntityID, time.Now().UTC(), time.Now().UTC())
	return err
}

func (r *UploadRepository) ByID(ctx context.Context, id string) (*model.Upload, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+uploadColumns+` FROM uploads WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Upload])
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UploadRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM uploads WHERE id = $1`, id)
	return err
}
