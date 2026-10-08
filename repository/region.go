package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/model"
)

// RegionRepository covers regions.
type RegionRepository struct {
	pool *pgxpool.Pool
}

func NewRegionRepository(pool *pgxpool.Pool) *RegionRepository {
	return &RegionRepository{pool: pool}
}

const regionColumns = `id, name, code, country, description, is_active,
	created_at, updated_at, type, states, ibge_state_code, ibge_city_codes,
	cities, geo_bounds, center_lat, center_lng, parent_region_id,
	whatsapp_session, whatsapp_phone, whatsapp_connected_at`

func (r *RegionRepository) List(ctx context.Context, onlyActive *bool) ([]model.Region, error) {
	q := `SELECT ` + regionColumns + ` FROM regions`
	if onlyActive != nil {
		if *onlyActive {
			q += ` WHERE is_active = true`
		} else {
			q += ` WHERE is_active = false`
		}
	}
	q += ` ORDER BY name ASC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Region])
}

func (r *RegionRepository) ByID(ctx context.Context, id string) (*model.Region, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+regionColumns+` FROM regions WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	g, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Region])
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *RegionRepository) ByCode(ctx context.Context, code string) (*model.Region, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+regionColumns+` FROM regions WHERE code = $1`, code)
	if err != nil {
		return nil, err
	}
	g, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Region])
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// ByIDs fetches regions for user list joins.
func (r *RegionRepository) ByIDs(ctx context.Context, ids []string) (map[string]*model.Region, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+regionColumns+` FROM regions WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	regions, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Region])
	if err != nil {
		return nil, err
	}
	out := make(map[string]*model.Region, len(regions))
	for i := range regions {
		out[regions[i].ID] = &regions[i]
	}
	return out, nil
}

func (r *RegionRepository) Create(ctx context.Context, g *model.Region) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO regions
		(id, name, code, country, description, is_active, created_at, updated_at,
		 type, states, ibge_state_code, ibge_city_codes, cities, geo_bounds,
		 center_lat, center_lng, parent_region_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		g.ID, g.Name, g.Code, g.Country, g.Description, g.IsActive,
		g.CreatedAt, g.UpdatedAt, g.Type, g.States, g.IBGEStateCode,
		g.IBGECityCodes, g.Cities, g.GeoBounds, g.CenterLat, g.CenterLng,
		g.ParentRegionID)
	return err
}

// Update applies the editable fields and bumps updated_at.
func (r *RegionRepository) Update(ctx context.Context, g *model.Region) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `UPDATE regions SET name = $1, code = $2,
		description = $3, is_active = $4, updated_at = $5 WHERE id = $6`,
		g.Name, g.Code, g.Description, g.IsActive, now, g.ID)
	if err != nil {
		return err
	}
	g.UpdatedAt = now
	return nil
}

func (r *RegionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM regions WHERE id = $1`, id)
	return err
}

// UpdateWhatsApp persists the team's meow session binding.
func (r *RegionRepository) UpdateWhatsApp(ctx context.Context, id string, session, phone *string, connectedAt *time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE regions SET whatsapp_session = $1,
		whatsapp_phone = $2, whatsapp_connected_at = $3, updated_at = $4 WHERE id = $5`,
		session, phone, connectedAt, time.Now(), id)
	return err
}

// CountCampaigns counts campaigns in a region (region delete guard).
func (r *RegionRepository) CountCampaigns(ctx context.Context, regionID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM campaigns WHERE region_id = $1`, regionID).Scan(&n)
	return n, err
}
