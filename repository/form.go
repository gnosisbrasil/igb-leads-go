package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/model"
)

// FormRepository covers forms.
type FormRepository struct {
	pool *pgxpool.Pool
}

func NewFormRepository(pool *pgxpool.Pool) *FormRepository {
	return &FormRepository{pool: pool}
}

const formColumns = `id, campaign_id, slug, title, description, is_active,
	custom_fields, created_at, updated_at, public_token, short_code`

func (r *FormRepository) ByID(ctx context.Context, id string) (*model.Form, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+formColumns+` FROM forms WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	f, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Form])
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FormRepository) ByPublicToken(ctx context.Context, token string) (*model.Form, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+formColumns+` FROM forms WHERE public_token = $1`, token)
	if err != nil {
		return nil, err
	}
	f, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Form])
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FormRepository) BySlug(ctx context.Context, slug string) (*model.Form, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+formColumns+` FROM forms WHERE slug = $1`, slug)
	if err != nil {
		return nil, err
	}
	f, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Form])
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FormRepository) ByShortCode(ctx context.Context, code string) (*model.Form, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+formColumns+` FROM forms WHERE short_code = $1`, code)
	if err != nil {
		return nil, err
	}
	f, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Form])
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// ShortCodeTaken reports whether the code is used by another form.
func (r *FormRepository) ShortCodeTaken(ctx context.Context, code, exceptID string) (bool, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM forms WHERE short_code = $1 AND id != $2`, code, exceptID).Scan(&n)
	return n > 0, err
}

// SlugTaken reports whether the slug exists.
func (r *FormRepository) SlugTaken(ctx context.Context, slug string) (bool, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM forms WHERE slug = $1`, slug).Scan(&n)
	return n > 0, err
}

// ByCampaignIDs returns forms of the campaigns, newest first.
func (r *FormRepository) ByCampaignIDs(ctx context.Context, campaignIDs []string) ([]model.Form, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+formColumns+` FROM forms
		WHERE campaign_id = ANY($1) ORDER BY created_at DESC`, campaignIDs)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Form])
}

// IDsByCampaignIDs returns form ids for lead scoping.
func (r *FormRepository) IDsByCampaignIDs(ctx context.Context, campaignIDs []string) ([]string, error) {
	if len(campaignIDs) == 0 {
		return []string{}, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT id FROM forms WHERE campaign_id = ANY($1)`, campaignIDs)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (r *FormRepository) Create(ctx context.Context, f *model.Form) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO forms
		(id, campaign_id, slug, title, description, is_active, custom_fields,
		 created_at, updated_at, public_token, short_code)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		f.ID, f.CampaignID, f.Slug, f.Title, f.Description, f.IsActive,
		nullJSON(f.CustomFields), f.CreatedAt, f.UpdatedAt, f.PublicToken, f.ShortCode)
	return err
}

var formUpdatable = map[string]bool{
	"title": true, "description": true, "custom_fields": true,
	"is_active": true, "short_code": true,
}

// UpdateFields applies a whitelisted partial update and bumps updated_at.
func (r *FormRepository) UpdateFields(ctx context.Context, id string, fields map[string]any) error {
	set := make([]string, 0, len(fields)+1)
	args := make([]any, 0, len(fields)+2)
	i := 1
	for col, val := range fields {
		if !formUpdatable[col] {
			return fmt.Errorf("coluna não atualizável: %s", col)
		}
		set = append(set, quoteCol(col, i))
		args = append(args, val)
		i++
	}
	set = append(set, quoteCol("updated_at", i))
	args = append(args, nowUTC())
	args = append(args, id)
	_, err := r.pool.Exec(ctx,
		fmt.Sprintf(`UPDATE forms SET %s WHERE id = $%d`, joinSet(set), i+1), args...)
	return err
}

// ActivateByCampaign enables every form of the campaign.
func (r *FormRepository) ActivateByCampaign(ctx context.Context, campaignID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE forms SET is_active = true, updated_at = $1
		WHERE campaign_id = $2 AND is_active = false`, nowUTC(), campaignID)
	return err
}

func (r *FormRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM forms WHERE id = $1`, id)
	return err
}

// List returns forms (optionally of the campaigns) plus the total.
func (r *FormRepository) List(ctx context.Context, campaignIDs []string, scoped bool, page, limit int) ([]model.Form, int, error) {
	w := ""
	var args []any
	if scoped {
		w = `WHERE campaign_id = ANY($1)`
		args = append(args, campaignIDs)
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM forms `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit
	q := fmt.Sprintf(`SELECT %s FROM forms %s ORDER BY created_at DESC LIMIT %d OFFSET %d`,
		formColumns, w, limit, offset)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	forms, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Form])
	if err != nil {
		return nil, 0, err
	}
	return forms, total, nil
}
