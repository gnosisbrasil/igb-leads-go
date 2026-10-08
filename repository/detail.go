package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/model"
)

// DetailRepository covers campaign_details.
type DetailRepository struct {
	pool *pgxpool.Pool
}

func NewDetailRepository(pool *pgxpool.Pool) *DetailRepository {
	return &DetailRepository{pool: pool}
}

const detailColumns = `id, campaign_id, ad_creative_text, ad_image_url,
	landing_page_url, call_to_action, target_demographics, custom_fields,
	created_at, updated_at`

func (r *DetailRepository) ByCampaign(ctx context.Context, campaignID string) (*model.CampaignDetail, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+detailColumns+` FROM campaign_details WHERE campaign_id = $1`, campaignID)
	if err != nil {
		return nil, err
	}
	d, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.CampaignDetail])
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DetailRepository) Create(ctx context.Context, d *model.CampaignDetail) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO campaign_details
		(id, campaign_id, ad_creative_text, ad_image_url, landing_page_url,
		 call_to_action, target_demographics, custom_fields, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		d.ID, d.CampaignID, d.AdCreativeText, d.AdImageURL, d.LandingPageURL,
		d.CallToAction, nullJSON(d.TargetDemographics), nullJSON(d.CustomFields),
		d.CreatedAt, d.UpdatedAt)
	return err
}

// UpdateFields applies detail fields from a decoded JSON body.
func (r *DetailRepository) UpdateFields(ctx context.Context, id string, fields map[string]any) error {
	allowed := map[string]bool{
		"ad_creative_text": true, "ad_image_url": true, "landing_page_url": true,
		"call_to_action": true, "target_demographics": true, "custom_fields": true,
	}
	set := []string{}
	args := []any{}
	i := 1
	for col, val := range fields {
		if !allowed[col] {
			continue
		}
		set = append(set, quoteCol(col, i))
		args = append(args, val)
		i++
	}
	set = append(set, quoteCol("updated_at", i))
	args = append(args, time.Now().UTC())
	args = append(args, id)
	_, err := r.pool.Exec(ctx, `UPDATE campaign_details SET `+joinSet(set)+` WHERE id = $`+itoa(i+1), args...)
	return err
}
