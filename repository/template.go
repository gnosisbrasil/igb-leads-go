package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/model"
)

// TemplateRepository covers message_templates.
type TemplateRepository struct {
	pool *pgxpool.Pool
}

func NewTemplateRepository(pool *pgxpool.Pool) *TemplateRepository {
	return &TemplateRepository{pool: pool}
}

const templateColumns = `id, campaign_id, key, label, content, sort_order,
	is_active, created_at, updated_at, phase, is_editable`

func (r *TemplateRepository) ByID(ctx context.Context, id string) (*model.MessageTemplate, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+templateColumns+` FROM message_templates WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.MessageTemplate])
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ByCampaign lists templates ordered by sort_order.
func (r *TemplateRepository) ByCampaign(ctx context.Context, campaignID string) ([]model.MessageTemplate, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+templateColumns+` FROM message_templates
		WHERE campaign_id = $1 ORDER BY sort_order ASC, key ASC`, campaignID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.MessageTemplate])
}

// ActiveByCampaign lists active templates for the public leads view.
func (r *TemplateRepository) ActiveByCampaign(ctx context.Context, campaignID string) ([]model.MessageTemplate, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+templateColumns+` FROM message_templates
		WHERE campaign_id = $1 AND is_active = true ORDER BY sort_order ASC, key ASC`, campaignID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.MessageTemplate])
}

func (r *TemplateRepository) ByCampaignAndKey(ctx context.Context, campaignID, key string) (*model.MessageTemplate, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+templateColumns+` FROM message_templates
		WHERE campaign_id = $1 AND key = $2`, campaignID, key)
	if err != nil {
		return nil, err
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.MessageTemplate])
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TemplateRepository) ByIDAndCampaign(ctx context.Context, id, campaignID string) (*model.MessageTemplate, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+templateColumns+` FROM message_templates
		WHERE id = $1 AND campaign_id = $2`, id, campaignID)
	if err != nil {
		return nil, err
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.MessageTemplate])
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TemplateRepository) ActiveByIDAndCampaign(ctx context.Context, id, campaignID string) (*model.MessageTemplate, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+templateColumns+` FROM message_templates
		WHERE id = $1 AND campaign_id = $2 AND is_active = true`, id, campaignID)
	if err != nil {
		return nil, err
	}
	t, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.MessageTemplate])
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TemplateRepository) Create(ctx context.Context, t *model.MessageTemplate) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO message_templates
		(id, campaign_id, key, label, content, sort_order, is_active,
		 created_at, updated_at, phase, is_editable)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		t.ID, t.CampaignID, t.Key, t.Label, t.Content, t.SortOrder, t.IsActive,
		t.CreatedAt, t.UpdatedAt, t.Phase, t.IsEditable)
	return err
}

var templateUpdatable = map[string]bool{
	"label": true, "content": true, "is_active": true,
	"sort_order": true, "phase": true,
}

// UpdateFields applies a whitelisted partial update and bumps updated_at.
func (r *TemplateRepository) UpdateFields(ctx context.Context, id string, fields map[string]any) error {
	set := make([]string, 0, len(fields)+1)
	args := make([]any, 0, len(fields)+2)
	i := 1
	for col, val := range fields {
		if !templateUpdatable[col] {
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
		fmt.Sprintf(`UPDATE message_templates SET %s WHERE id = $%d`, joinSet(set), i+1), args...)
	return err
}

func (r *TemplateRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM message_templates WHERE id = $1`, id)
	return err
}

// Mini is the template projection embedded in the public leads view.
type TemplateMini struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Label     string `json:"label"`
	Content   string `json:"content"`
	Phase     string `json:"phase"`
	SortOrder int    `json:"sort_order"`
}

func ToTemplateMini(t *model.MessageTemplate) TemplateMini {
	return TemplateMini{ID: t.ID, Key: t.Key, Label: t.Label, Content: t.Content, Phase: t.Phase, SortOrder: t.SortOrder}
}
