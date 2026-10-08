package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/model"
)

// ReportRepository covers the aggregates behind ReportController.
type ReportRepository struct {
	pool *pgxpool.Pool
}

func NewReportRepository(pool *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{pool: pool}
}

// CountCampaigns counts campaigns with optional status/region/traffic-manager filters.
func (r *ReportRepository) CountCampaigns(ctx context.Context, status, regionID, trafficManagerID *string) (int, error) {
	q := `SELECT COUNT(*) FROM campaigns WHERE 1=1`
	args := []any{}
	if status != nil {
		args = append(args, *status)
		q += ` AND status = $` + itoa(len(args))
	}
	if regionID != nil {
		args = append(args, *regionID)
		q += ` AND region_id = $` + itoa(len(args))
	}
	if trafficManagerID != nil {
		args = append(args, *trafficManagerID)
		q += ` AND traffic_manager_id = $` + itoa(len(args))
	}
	var n int
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CountLeads counts leads, optionally restricted to a status.
func (r *ReportRepository) CountLeads(ctx context.Context, status *string) (int, error) {
	q := `SELECT COUNT(*) FROM leads`
	var args []any
	if status != nil {
		args = append(args, *status)
		q += ` WHERE status = $1`
	}
	var n int
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CountLeadsByForms counts leads in the given forms, optionally by status.
// An empty form list counts zero, mirroring the Node length guards.
func (r *ReportRepository) CountLeadsByForms(ctx context.Context, formIDs []string, status *string) (int, error) {
	if len(formIDs) == 0 {
		return 0, nil
	}
	q := `SELECT COUNT(*) FROM leads WHERE form_id = ANY($1)`
	args := []any{formIDs}
	if status != nil {
		args = append(args, *status)
		q += ` AND status = $2`
	}
	var n int
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CampaignsByRegion lists full campaigns of a region (no ORDER BY, like Node).
func (r *ReportRepository) CampaignsByRegion(ctx context.Context, regionID string) ([]model.Campaign, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+campaignColumns+` FROM campaigns WHERE region_id = $1`, regionID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Campaign])
}

// CampaignsByTrafficManager lists full campaigns assigned to an executive.
func (r *ReportRepository) CampaignsByTrafficManager(ctx context.Context, userID string) ([]model.Campaign, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+campaignColumns+` FROM campaigns WHERE traffic_manager_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Campaign])
}

// LeadIDStatus mirrors the leads attributes projection (id, status only)
// nested inside campaignsPerformance forms.
type LeadIDStatus struct {
	ID     string `db:"id" json:"id"`
	Status string `db:"status" json:"status"`
}

// LeadsMiniByForms fetches the id/status projection for the given forms.
func (r *ReportRepository) LeadsMiniByForms(ctx context.Context, formIDs []string) (map[string][]LeadIDStatus, error) {
	out := map[string][]LeadIDStatus{}
	if len(formIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT id, status, form_id FROM leads WHERE form_id = ANY($1)`, formIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, status string
		var formID *string
		if err := rows.Scan(&id, &status, &formID); err != nil {
			return nil, err
		}
		if formID == nil {
			continue
		}
		out[*formID] = append(out[*formID], LeadIDStatus{ID: id, Status: status})
	}
	return out, rows.Err()
}
