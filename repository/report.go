package repository

import (
	"context"
	"sort"
	"time"

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

// CampaignIDsFiltered returns campaign IDs matching the resolved scope.
// All filters are optional; nil means unfiltered.
func (r *ReportRepository) CampaignIDsFiltered(ctx context.Context, campaignID, regionID, trafficManagerID, ownerID *string) ([]string, error) {
	q := `SELECT id FROM campaigns WHERE 1=1`
	args := []any{}
	add := func(cond string, v string) {
		args = append(args, v)
		q += ` AND ` + cond + ` $` + itoa(len(args))
	}
	if campaignID != nil {
		add(`id =`, *campaignID)
	}
	if regionID != nil {
		add(`region_id =`, *regionID)
	}
	if trafficManagerID != nil {
		add(`traffic_manager_id =`, *trafficManagerID)
	}
	if ownerID != nil {
		add(`user_id =`, *ownerID)
	}
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// dateBounds appends created_at bounds to a leads query. From is inclusive,
// To is exclusive. Table alias for leads must be "l".
func dateBounds(q *string, args *[]any, from, to *time.Time) {
	if from != nil {
		*args = append(*args, *from)
		*q += ` AND l.created_at >= $` + itoa(len(*args))
	}
	if to != nil {
		*args = append(*args, *to)
		*q += ` AND l.created_at < $` + itoa(len(*args))
	}
}

// LeadStatusCounts groups lead counts by status for the given campaigns.
func (r *ReportRepository) LeadStatusCounts(ctx context.Context, campaignIDs []string, from, to *time.Time) (map[string]int, error) {
	out := map[string]int{}
	if len(campaignIDs) == 0 {
		return out, nil
	}
	q := `
		SELECT l.status, COUNT(*)
		FROM leads l
		JOIN forms f ON f.id = l.form_id
		WHERE f.campaign_id = ANY($1)`
	args := []any{campaignIDs}
	dateBounds(&q, &args, from, to)
	q += ` GROUP BY l.status`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

// CampaignLeadStat is one row of the per-campaign breakdown.
type CampaignLeadStat struct {
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	Status     string         `json:"status"`
	Objectives *string        `json:"objectives"`
	RegionID   *string        `json:"region_id"`
	State      *string        `json:"state"`
	Leads      int            `json:"leads"`
	ByStatus   map[string]int `json:"by_status"`
}

// CampaignLeadStats lists per-campaign lead totals with status breakdown.
func (r *ReportRepository) CampaignLeadStats(ctx context.Context, campaignIDs []string, from, to *time.Time) ([]CampaignLeadStat, error) {
	out := []CampaignLeadStat{}
	if len(campaignIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT id, title, status, objectives, region_id, NULLIF(UPPER(TRIM(address_state)), '') FROM campaigns WHERE id = ANY($1) ORDER BY created_at DESC`, campaignIDs)
	if err != nil {
		return nil, err
	}
	stats := map[string]*CampaignLeadStat{}
	order := []string{}
	for rows.Next() {
		var s CampaignLeadStat
		if err := rows.Scan(&s.ID, &s.Title, &s.Status, &s.Objectives, &s.RegionID, &s.State); err != nil {
			rows.Close()
			return nil, err
		}
		s.ByStatus = map[string]int{}
		stats[s.ID] = &s
		order = append(order, s.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	dq := `
		SELECT f.campaign_id, l.status, COUNT(*)
		FROM leads l
		JOIN forms f ON f.id = l.form_id
		WHERE f.campaign_id = ANY($1)`
	dargs := []any{campaignIDs}
	dateBounds(&dq, &dargs, from, to)
	dq += ` GROUP BY f.campaign_id, l.status`
	detail, err := r.pool.Query(ctx, dq, dargs...)
	if err != nil {
		return nil, err
	}
	defer detail.Close()
	for detail.Next() {
		var cid, status string
		var n int
		if err := detail.Scan(&cid, &status, &n); err != nil {
			return nil, err
		}
		if s, ok := stats[cid]; ok {
			s.ByStatus[status] = n
			s.Leads += n
		}
	}
	if err := detail.Err(); err != nil {
		return nil, err
	}
	for _, id := range order {
		out = append(out, *stats[id])
	}
	return out, nil
}

// RegionLeadStat is one row of the per-region breakdown.
type RegionLeadStat struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Campaigns int    `json:"campaigns"`
	Leads     int    `json:"leads"`
}

// RegionLeadStats groups campaign and lead counts by region.
func (r *ReportRepository) RegionLeadStats(ctx context.Context, campaignIDs []string, from, to *time.Time) ([]RegionLeadStat, error) {
	out := []RegionLeadStat{}
	if len(campaignIDs) == 0 {
		return out, nil
	}
	q := `
		SELECT g.id, g.name, COUNT(DISTINCT c.id), COUNT(l.id)
		FROM regions g
		JOIN campaigns c ON c.region_id = g.id AND c.id = ANY($1)
		LEFT JOIN forms f ON f.campaign_id = c.id
		LEFT JOIN leads l ON l.form_id = f.id`
	args := []any{campaignIDs}
	// Bounds inside ON (not WHERE) so regions with zero leads stay listed.
	on := ""
	dateBounds(&on, &args, from, to)
	q += on + ` GROUP BY g.id, g.name ORDER BY COUNT(l.id) DESC`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s RegionLeadStat
		if err := rows.Scan(&s.ID, &s.Name, &s.Campaigns, &s.Leads); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// StateLeadStat is one row of the per-state (UF) breakdown.
type StateLeadStat struct {
	State     string         `json:"state"`
	Campaigns int            `json:"campaigns"`
	Leads     int            `json:"leads"`
	ByStatus  map[string]int `json:"by_status"`
}

// StateLeadStats groups campaign and lead counts by campaign state (UF),
// ignoring campaigns without a state.
func (r *ReportRepository) StateLeadStats(ctx context.Context, campaignIDs []string, from, to *time.Time) ([]StateLeadStat, error) {
	out := []StateLeadStat{}
	if len(campaignIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT NULLIF(UPPER(TRIM(c.address_state)), ''), COUNT(DISTINCT c.id)
		FROM campaigns c
		WHERE c.id = ANY($1)
		GROUP BY 1`, campaignIDs)
	if err != nil {
		return nil, err
	}
	stats := map[string]*StateLeadStat{}
	for rows.Next() {
		var uf *string
		var n int
		if err := rows.Scan(&uf, &n); err != nil {
			rows.Close()
			return nil, err
		}
		if uf == nil || *uf == "" {
			continue
		}
		stats[*uf] = &StateLeadStat{State: *uf, Campaigns: n, ByStatus: map[string]int{}}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	dq := `
		SELECT NULLIF(UPPER(TRIM(c.address_state)), ''), l.status, COUNT(*)
		FROM leads l
		JOIN forms f ON f.id = l.form_id
		JOIN campaigns c ON c.id = f.campaign_id
		WHERE f.campaign_id = ANY($1)`
	dargs := []any{campaignIDs}
	dateBounds(&dq, &dargs, from, to)
	dq += ` GROUP BY 1, 2`
	detail, err := r.pool.Query(ctx, dq, dargs...)
	if err != nil {
		return nil, err
	}
	defer detail.Close()
	for detail.Next() {
		var uf *string
		var status string
		var n int
		if err := detail.Scan(&uf, &status, &n); err != nil {
			return nil, err
		}
		if uf == nil || *uf == "" {
			continue
		}
		if s, ok := stats[*uf]; ok {
			s.ByStatus[status] = n
			s.Leads += n
		}
	}
	if err := detail.Err(); err != nil {
		return nil, err
	}
	for _, s := range stats {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Leads > out[j].Leads })
	return out, nil
}

// DailyLeadCount is one row of the signups-per-day series.
type DailyLeadCount struct {
	Date  string `json:"date"`
	Leads int    `json:"leads"`
}

// LeadDailyCounts returns signups per day (oldest first, capped).
func (r *ReportRepository) LeadDailyCounts(ctx context.Context, campaignIDs []string, days int, from, to *time.Time) ([]DailyLeadCount, error) {
	out := []DailyLeadCount{}
	if len(campaignIDs) == 0 {
		return out, nil
	}
	if days <= 0 || days > 365 {
		days = 30
	}
	q := `
		SELECT to_char(l.created_at, 'YYYY-MM-DD'), COUNT(*)
		FROM leads l
		JOIN forms f ON f.id = l.form_id
		WHERE f.campaign_id = ANY($1)`
	args := []any{campaignIDs}
	dateBounds(&q, &args, from, to)
	if from == nil && to == nil {
		args = append(args, days)
		q += ` AND l.created_at >= now() - make_interval(days => $` + itoa(len(args)) + `)`
	}
	q += ` GROUP BY 1 ORDER BY 1`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d DailyLeadCount
		if err := rows.Scan(&d.Date, &d.Leads); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
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
