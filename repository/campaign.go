package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/model"
)

// CampaignRepository covers campaigns and their details.
type CampaignRepository struct {
	pool *pgxpool.Pool
}

func NewCampaignRepository(pool *pgxpool.Pool) *CampaignRepository {
	return &CampaignRepository{pool: pool}
}

const campaignColumns = `id, title, description, status, budget, platform,
	start_date, end_date, event_date, location, target_audience, objectives,
	approved_at, completed_at, user_id, region_id, supervisor_id, traffic_manager_id,
	created_at, updated_at, public_link_token, public_link_password, public_link_active,
	address, address_number, address_neighborhood, address_city, address_state, address_zipcode,
	maps_url, payment_status, payment_method, payment_gateway_id, payment_txid,
	payment_amount, payment_paid_at, payment_qr_code, payment_boleto_url, payment_boleto_barcode,
	form_id, latitude, longitude, whatsapp_confirmation_msg, whatsapp_voucher_msg,
	form_title, form_description, form_fields, form_header_text, form_cta_text,
	payment_url, display_id, rejection_reason, event_time, weekdays, event_dates,
	google_maps_link, lp_template, pending_edit, turmas, auto_relationship,
	auto_relationship_cost, accepted_at, responsible_name, responsible_whatsapp,
	template_config, products, snack_price, goal_leads`

func (r *CampaignRepository) ByID(ctx context.Context, id string) (*model.Campaign, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+campaignColumns+` FROM campaigns WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	c, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Campaign])
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CampaignRepository) ByPublicToken(ctx context.Context, token string) (*model.Campaign, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+campaignColumns+` FROM campaigns
		WHERE public_link_token = $1 AND public_link_active = true`, token)
	if err != nil {
		return nil, err
	}
	c, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Campaign])
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ByPaymentTxid finds the campaign awaiting the Pix txid.
func (r *CampaignRepository) ByPaymentTxid(ctx context.Context, txid string) (*model.Campaign, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+campaignColumns+` FROM campaigns WHERE payment_txid = $1`, txid)
	if err != nil {
		return nil, err
	}
	c, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Campaign])
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Create inserts the campaign; display_id comes back from the sequence.
func (r *CampaignRepository) Create(ctx context.Context, c *model.Campaign) error {
	pairs := []struct {
		col string
		val any
	}{
		{"id", c.ID}, {"title", c.Title}, {"description", c.Description},
		{"status", c.Status}, {"budget", c.Budget}, {"platform", c.Platform},
		{"start_date", c.StartDate}, {"end_date", c.EndDate}, {"event_date", c.EventDate},
		{"location", c.Location}, {"target_audience", c.TargetAudience}, {"objectives", c.Objectives},
		{"approved_at", c.ApprovedAt}, {"completed_at", c.CompletedAt}, {"user_id", c.UserID},
		{"region_id", c.RegionID}, {"supervisor_id", c.SupervisorID}, {"traffic_manager_id", c.TrafficManagerID},
		{"created_at", c.CreatedAt}, {"updated_at", c.UpdatedAt},
		{"public_link_token", c.PublicLinkToken}, {"public_link_password", c.PublicLinkPassword},
		{"public_link_active", c.PublicLinkActive},
		{"address", c.Address}, {"address_number", c.AddressNumber},
		{"address_neighborhood", c.AddressNeighborhood}, {"address_city", c.AddressCity},
		{"address_state", c.AddressState}, {"address_zipcode", c.AddressZipcode},
		{"maps_url", c.MapsURL}, {"payment_status", c.PaymentStatus}, {"payment_method", c.PaymentMethod},
		{"payment_gateway_id", c.PaymentGatewayID}, {"payment_txid", c.PaymentTxid},
		{"payment_amount", c.PaymentAmount}, {"payment_paid_at", c.PaymentPaidAt},
		{"payment_qr_code", c.PaymentQRCode}, {"payment_boleto_url", c.PaymentBoletoURL},
		{"payment_boleto_barcode", c.PaymentBoletoBarcode},
		{"form_id", c.FormID}, {"latitude", c.Latitude}, {"longitude", c.Longitude},
		{"whatsapp_confirmation_msg", c.WhatsappConfirmationMsg}, {"whatsapp_voucher_msg", c.WhatsappVoucherMsg},
		{"form_title", c.FormTitle}, {"form_description", c.FormDescription},
		{"form_fields", nullJSON(c.FormFields)}, {"form_header_text", c.FormHeaderText},
		{"form_cta_text", c.FormCTAText}, {"payment_url", c.PaymentURL},
		{"rejection_reason", c.RejectionReason}, {"event_time", c.EventTime},
		{"weekdays", nullJSON(c.Weekdays)}, {"event_dates", nullJSON(c.EventDates)},
		{"google_maps_link", c.GoogleMapsLink}, {"lp_template", c.LPTemplate},
		{"pending_edit", nullJSON(c.PendingEdit)}, {"turmas", nullJSON(c.Turmas)},
		{"auto_relationship", c.AutoRelationship}, {"auto_relationship_cost", c.AutoRelationshipCost},
		{"accepted_at", c.AcceptedAt}, {"responsible_name", c.ResponsibleName},
		{"responsible_whatsapp", c.ResponsibleWhatsapp}, {"template_config", nullJSON(c.TemplateConfig)},
		{"products", nullJSON(c.Products)}, {"snack_price", c.SnackPrice},
		{"goal_leads", c.GoalLeads},
	}
	cols := make([]string, 0, len(pairs))
	holders := make([]string, 0, len(pairs))
	args := make([]any, 0, len(pairs))
	for i, p := range pairs {
		cols = append(cols, p.col)
		holders = append(holders, fmt.Sprintf("$%d", i+1))
		args = append(args, p.val)
	}
	q := fmt.Sprintf(`INSERT INTO campaigns (%s) VALUES (%s) RETURNING display_id`,
		strings.Join(cols, ", "), strings.Join(holders, ", "))
	return r.pool.QueryRow(ctx, q, args...).Scan(&c.DisplayID)
}

func nullJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

// campaignUpdatable whitelists dynamic update columns.
var campaignUpdatable = map[string]bool{
	"title": true, "description": true, "status": true, "budget": true, "platform": true,
	"start_date": true, "end_date": true, "event_date": true, "location": true,
	"target_audience": true, "objectives": true, "approved_at": true, "completed_at": true,
	"region_id": true, "supervisor_id": true, "traffic_manager_id": true,
	"public_link_token": true, "public_link_password": true, "public_link_active": true,
	"address": true, "address_number": true, "address_neighborhood": true,
	"address_city": true, "address_state": true, "address_zipcode": true,
	"maps_url": true, "payment_status": true, "payment_method": true,
	"payment_gateway_id": true, "payment_txid": true, "payment_amount": true,
	"payment_paid_at": true, "payment_qr_code": true, "payment_boleto_url": true,
	"payment_boleto_barcode": true, "form_id": true, "latitude": true, "longitude": true,
	"whatsapp_confirmation_msg": true, "whatsapp_voucher_msg": true,
	"form_title": true, "form_description": true, "form_fields": true,
	"form_header_text": true, "form_cta_text": true, "payment_url": true,
	"rejection_reason": true, "event_time": true, "weekdays": true, "event_dates": true,
	"google_maps_link": true, "lp_template": true, "pending_edit": true, "turmas": true,
	"auto_relationship": true, "auto_relationship_cost": true, "accepted_at": true,
	"responsible_name": true, "responsible_whatsapp": true, "template_config": true, "goal_leads": true,
	"products": true, "snack_price": true,
}

// UpdateFields applies a whitelisted partial update and bumps updated_at.
func (r *CampaignRepository) UpdateFields(ctx context.Context, id string, fields map[string]any) error {
	set := make([]string, 0, len(fields)+1)
	args := make([]any, 0, len(fields)+2)
	i := 1
	for col, val := range fields {
		if !campaignUpdatable[col] {
			return fmt.Errorf("coluna não atualizável: %s", col)
		}
		if b, ok := val.([]byte); ok && len(b) == 0 {
			val = nil
		}
		set = append(set, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	set = append(set, fmt.Sprintf("updated_at = $%d", i))
	args = append(args, time.Now().UTC())
	args = append(args, id)
	_, err := r.pool.Exec(ctx,
		fmt.Sprintf(`UPDATE campaigns SET %s WHERE id = $%d`, strings.Join(set, ", "), i+1),
		args...)
	return err
}

func (r *CampaignRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM campaigns WHERE id = $1`, id)
	return err
}

// CampaignFilter mirrors the list scoping resolved by the handler.
type CampaignFilter struct {
	Status           string
	RegionID         string
	UserID           string
	TrafficManagerID string
	Search           string
	Page             int
	Limit            int
}

func (r *CampaignRepository) List(ctx context.Context, f CampaignFilter) ([]model.Campaign, int, error) {
	where := []string{}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.RegionID != "" {
		add("region_id = $%d", f.RegionID)
	}
	if f.UserID != "" {
		add("user_id = $%d", f.UserID)
	}
	if f.TrafficManagerID != "" {
		add("traffic_manager_id = $%d", f.TrafficManagerID)
	}
	if f.Search != "" {
		ors := []string{}
		args = append(args, "%"+f.Search+"%")
		ors = append(ors, fmt.Sprintf("title ILIKE $%d", len(args)))
		args = append(args, "%"+f.Search+"%")
		ors = append(ors, fmt.Sprintf("address_city ILIKE $%d", len(args)))
		if n, err := strconv.Atoi(f.Search); err == nil {
			args = append(args, n)
			ors = append(ors, fmt.Sprintf("display_id = $%d", len(args)))
		}
		where = append(where, "("+strings.Join(ors, " OR ")+")")
	}
	w := ""
	if len(where) > 0 {
		w = "WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM campaigns `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit
	q := fmt.Sprintf(`SELECT %s FROM campaigns %s ORDER BY created_at DESC LIMIT %d OFFSET %d`,
		campaignColumns, w, limit, offset)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	campaigns, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Campaign])
	if err != nil {
		return nil, 0, err
	}
	return campaigns, total, nil
}

// ListAvailable lists waiting_executive campaigns, newest first.
func (r *CampaignRepository) ListAvailable(ctx context.Context, regionID string, page, limit int) ([]model.Campaign, int, error) {
	return r.List(ctx, CampaignFilter{Status: "waiting_executive", RegionID: regionID, Page: page, Limit: limit})
}

// AutoBetween returns auto_relationship campaigns with event_date in range.
func (r *CampaignRepository) AutoBetween(ctx context.Context, from, to time.Time) ([]model.Campaign, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+campaignColumns+` FROM campaigns
		WHERE auto_relationship = true AND event_date BETWEEN $1 AND $2`, from, to)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Campaign])
}

// InProgressWithEventBetween returns in_progress campaigns with event_date in range.
func (r *CampaignRepository) InProgressWithEventBetween(ctx context.Context, from, to time.Time) ([]model.Campaign, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+campaignColumns+` FROM campaigns
		WHERE status = 'in_progress' AND event_date BETWEEN $1 AND $2`, from, to)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Campaign])
}

// CampaignObjectives carries id + objectives for the boot template sync.
type CampaignObjectives struct {
	ID         string
	Objectives string
}

// IDsAndObjectives lists every campaign id with its objectives.
func (r *CampaignRepository) IDsAndObjectives(ctx context.Context) ([]CampaignObjectives, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, COALESCE(objectives, '') FROM campaigns`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CampaignObjectives{}
	for rows.Next() {
		var c CampaignObjectives
		if err := rows.Scan(&c.ID, &c.Objectives); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// IDsByUser returns campaign ids owned by the user.
func (r *CampaignRepository) IDsByUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM campaigns WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// IDsByFilter returns campaign ids for lead scoping.
func (r *CampaignRepository) IDsByFilter(ctx context.Context, userID, role, regionID, campaignID string) ([]string, error) {
	where := []string{}
	args := []any{}
	switch role {
	case model.RoleUser:
		args = append(args, userID)
		where = append(where, fmt.Sprintf("user_id = $%d", len(args)))
	case model.RoleExecutive:
		args = append(args, userID)
		where = append(where, fmt.Sprintf("traffic_manager_id = $%d", len(args)))
	case model.RoleSupervisor:
		if regionID == "" {
			return []string{}, nil
		}
		args = append(args, regionID)
		where = append(where, fmt.Sprintf("region_id = $%d", len(args)))
	}
	if campaignID != "" {
		args = append(args, campaignID)
		where = append(where, fmt.Sprintf("id = $%d", len(args)))
	}
	w := ""
	if len(where) > 0 {
		w = "WHERE " + strings.Join(where, " AND ")
	}
	rows, err := r.pool.Query(ctx, `SELECT id FROM campaigns `+w, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// DistinctStates returns campaign address_state values for getStates.
func (r *CampaignRepository) DistinctStates(ctx context.Context, userID, role, regionID string) ([]string, error) {
	where := []string{}
	args := []any{}
	switch role {
	case model.RoleUser:
		args = append(args, userID)
		where = append(where, fmt.Sprintf("user_id = $%d", len(args)))
	case model.RoleExecutive:
		args = append(args, userID)
		cond := fmt.Sprintf("traffic_manager_id = $%d", len(args))
		if regionID != "" {
			args = append(args, regionID)
			cond = fmt.Sprintf("(traffic_manager_id = $%d OR region_id = $%d)", len(args)-1, len(args))
		}
		where = append(where, cond)
	case model.RoleSupervisor:
		if regionID == "" {
			return []string{}, nil
		}
		args = append(args, regionID)
		where = append(where, fmt.Sprintf("region_id = $%d", len(args)))
	}
	w := ""
	if len(where) > 0 {
		w = "WHERE " + strings.Join(where, " AND ")
	}
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT address_state FROM campaigns `+w+` AND address_state IS NOT NULL`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
