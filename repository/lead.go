package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/model"
)

// LeadRepository covers leads.
type LeadRepository struct {
	pool *pgxpool.Pool
}

func NewLeadRepository(pool *pgxpool.Pool) *LeadRepository {
	return &LeadRepository{pool: pool}
}

const leadColumns = `id, form_id, first_name, last_name, whatsapp, email,
	checkin_code, status, confirmation_sent_at, qr_sent_at, checkin_at, notes,
	metadata, created_at, updated_at, reminder_sent_at, confirmed_at, voucher_sent_at,
	motivation_sent_at, reminder_event_sent_at, attended_at, cancelled_at, city, state`

func (r *LeadRepository) ByID(ctx context.Context, id string) (*model.Lead, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+leadColumns+` FROM leads WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	l, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Lead])
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *LeadRepository) ByCheckinCode(ctx context.Context, code string) (*model.Lead, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+leadColumns+` FROM leads WHERE checkin_code = $1`, code)
	if err != nil {
		return nil, err
	}
	l, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Lead])
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *LeadRepository) Create(ctx context.Context, l *model.Lead) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO leads
		(id, form_id, first_name, last_name, whatsapp, email, checkin_code, status,
		 confirmation_sent_at, qr_sent_at, checkin_at, notes, metadata, created_at, updated_at,
		 reminder_sent_at, confirmed_at, voucher_sent_at, motivation_sent_at,
		 reminder_event_sent_at, attended_at, cancelled_at, city, state)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`,
		l.ID, l.FormID, l.FirstName, l.LastName, l.Whatsapp, l.Email, l.CheckinCode, l.Status,
		l.ConfirmationSentAt, l.QRSentAt, l.CheckinAt, l.Notes, nullJSON(l.Metadata),
		l.CreatedAt, l.UpdatedAt, l.ReminderSentAt, l.ConfirmedAt, l.VoucherSentAt,
		l.MotivationSentAt, l.ReminderEventSentAt, l.AttendedAt, l.CancelledAt, l.City, l.State)
	return err
}

var leadUpdatable = map[string]bool{
	"first_name": true, "last_name": true, "whatsapp": true, "email": true,
	"status": true, "confirmation_sent_at": true, "qr_sent_at": true, "checkin_at": true,
	"notes": true, "metadata": true, "reminder_sent_at": true, "confirmed_at": true,
	"voucher_sent_at": true, "motivation_sent_at": true, "reminder_event_sent_at": true,
	"attended_at": true, "cancelled_at": true, "city": true, "state": true, "form_id": true,
}

// UpdateFields applies a whitelisted partial update and bumps updated_at.
func (r *LeadRepository) UpdateFields(ctx context.Context, id string, fields map[string]any) error {
	set := make([]string, 0, len(fields)+1)
	args := make([]any, 0, len(fields)+2)
	i := 1
	for col, val := range fields {
		if !leadUpdatable[col] {
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
		fmt.Sprintf(`UPDATE leads SET %s WHERE id = $%d`, joinSet(set), i+1), args...)
	return err
}

func (r *LeadRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM leads WHERE id = $1`, id)
	return err
}

// ListFilter mirrors the lead list scoping resolved by the handler.
type LeadFilter struct {
	FormIDs []string // nil = unscoped; non-nil (maybe empty handled by caller)
	Scoped  bool
	Status  string
	Page    int
	Limit   int
}

// List returns the page plus the total, newest first.
func (r *LeadRepository) List(ctx context.Context, f LeadFilter) ([]model.Lead, int, error) {
	where := []string{}
	args := []any{}
	if f.Status != "" {
		args = append(args, f.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.Scoped {
		args = append(args, f.FormIDs)
		where = append(where, fmt.Sprintf("form_id = ANY($%d)", len(args)))
	}
	w := ""
	if len(where) > 0 {
		w = "WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM leads `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}
	offset := (page - 1) * limit
	q := fmt.Sprintf(`SELECT %s FROM leads %s ORDER BY created_at DESC LIMIT %d OFFSET %d`,
		leadColumns, w, limit, offset)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	leads, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Lead])
	if err != nil {
		return nil, 0, err
	}
	return leads, total, nil
}

// ByFormIDs returns leads of the forms, newest first (public view, export).
func (r *LeadRepository) ByFormIDs(ctx context.Context, formIDs []string) ([]model.Lead, error) {
	if len(formIDs) == 0 {
		return []model.Lead{}, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+leadColumns+` FROM leads
		WHERE form_id = ANY($1) ORDER BY created_at DESC`, formIDs)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Lead])
}

// Search finds leads by name/email/whatsapp, newest capped at 20.
func (r *LeadRepository) Search(ctx context.Context, q string, formIDs []string, scoped bool) ([]model.Lead, error) {
	like := "%" + q + "%"
	where := `(first_name ILIKE $1 OR last_name ILIKE $1 OR email ILIKE $1 OR whatsapp ILIKE $1)`
	args := []any{like}
	if scoped {
		args = append(args, formIDs)
		where += ` AND form_id = ANY($2)`
	}
	rows, err := r.pool.Query(ctx, `SELECT `+leadColumns+` FROM leads WHERE `+where+` LIMIT 20`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Lead])
}

// HistoryFilter mirrors the history scoping resolved by the handler.
type HistoryFilter struct {
	CampaignIDs []string // nil = unscoped
	Scoped      bool
	City        string
	State       string
	Status      string
	Interest    string // "", "interested", "registered"
	Page        int
	Limit       int
}

// HistoryRow joins lead + form + campaign mini for the history projection.
type HistoryRow struct {
	Lead            model.Lead
	FormID          *string
	CampaignID      *string
	CampaignTitle   *string
	CampaignDisplay *int
	CampaignCity    *string
	CampaignState   *string
}

// History returns joined rows plus the total, newest first.
func (r *LeadRepository) History(ctx context.Context, f HistoryFilter) ([]HistoryRow, int, error) {
	join := `LEFT JOIN forms f ON f.id = l.form_id LEFT JOIN campaigns c ON c.id = f.campaign_id`
	where := []string{}
	args := []any{}
	if f.Scoped {
		args = append(args, f.CampaignIDs)
		join = `JOIN forms f ON f.id = l.form_id AND f.campaign_id = ANY($1) JOIN campaigns c ON c.id = f.campaign_id`
	}
	if f.City != "" {
		args = append(args, "%"+f.City+"%")
		where = append(where, fmt.Sprintf("c.address_city ILIKE $%d", len(args)))
	}
	if f.State != "" {
		args = append(args, strings.ToUpper(f.State))
		where = append(where, fmt.Sprintf("c.address_state = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where = append(where, fmt.Sprintf("l.status = $%d", len(args)))
	}
	// NOTE: Node's `{[Op.ne]: true}` on the JSONB path excludes NULLs
	// (every plain lead); the intent is "not interested", implemented here.
	switch f.Interest {
	case "interested":
		where = append(where, `(l.metadata->>'interest_only')::boolean IS TRUE`)
	case "registered":
		where = append(where, `(l.metadata->>'interest_only')::boolean IS DISTINCT FROM TRUE`)
	}
	w := ""
	if len(where) > 0 {
		w = "WHERE " + strings.Join(where, " AND ")
	}
	cols := `l.id, l.form_id, l.first_name, l.last_name, l.whatsapp, l.email,
		l.checkin_code, l.status, l.confirmation_sent_at, l.qr_sent_at, l.checkin_at, l.notes,
		l.metadata, l.created_at, l.updated_at, l.reminder_sent_at, l.confirmed_at, l.voucher_sent_at,
		l.motivation_sent_at, l.reminder_event_sent_at, l.attended_at, l.cancelled_at, l.city, l.state,
		f.id AS form_id2, c.id AS campaign_id, c.title AS campaign_title,
		c.display_id AS campaign_display_id, c.address_city AS campaign_city, c.address_state AS campaign_state`
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM leads l `+join+` `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	offset := (page - 1) * limit
	q := fmt.Sprintf(`SELECT %s FROM leads l %s %s ORDER BY l.created_at DESC LIMIT %d OFFSET %d`,
		cols, join, w, limit, offset)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []HistoryRow{}
	for rows.Next() {
		var h HistoryRow
		var l = &h.Lead
		var meta []byte
		err := rows.Scan(&l.ID, &l.FormID, &l.FirstName, &l.LastName, &l.Whatsapp, &l.Email,
			&l.CheckinCode, &l.Status, &l.ConfirmationSentAt, &l.QRSentAt, &l.CheckinAt, &l.Notes,
			&meta, &l.CreatedAt, &l.UpdatedAt, &l.ReminderSentAt, &l.ConfirmedAt, &l.VoucherSentAt,
			&l.MotivationSentAt, &l.ReminderEventSentAt, &l.AttendedAt, &l.CancelledAt, &l.City, &l.State,
			&h.FormID, &h.CampaignID, &h.CampaignTitle, &h.CampaignDisplay, &h.CampaignCity, &h.CampaignState)
		if err != nil {
			return nil, 0, err
		}
		l.Metadata = meta
		out = append(out, h)
	}
	return out, total, rows.Err()
}

// DistinctStates returns lead states for getStates.
func (r *LeadRepository) DistinctStates(ctx context.Context, campaignIDs []string, scoped bool) ([]string, error) {
	q := `SELECT DISTINCT state FROM leads WHERE state IS NOT NULL`
	var args []any
	if scoped {
		q += ` AND form_id IN (SELECT id FROM forms WHERE campaign_id = ANY($1))`
		args = append(args, campaignIDs)
	}
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// CountNotNull counts leads of the forms with the column set (health).
func (r *LeadRepository) CountNotNull(ctx context.Context, formIDs []string, column string) (int, error) {
	allowed := map[string]bool{
		"confirmation_sent_at": true, "confirmed_at": true, "attended_at": true,
		"cancelled_at": true, "voucher_sent_at": true,
	}
	if !allowed[column] {
		return 0, fmt.Errorf("coluna inválida: %s", column)
	}
	var n int
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM leads
		WHERE form_id = ANY($1) AND %s IS NOT NULL`, column), formIDs).Scan(&n)
	return n, err
}

// CountByForms counts leads of the forms (health total).
func (r *LeadRepository) CountByForms(ctx context.Context, formIDs []string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM leads WHERE form_id = ANY($1)`, formIDs).Scan(&n)
	return n, err
}

// StampCityState fills city/state from the campaign before dissociation.
func (r *LeadRepository) StampCityState(ctx context.Context, formIDs []string, city, state *string) error {
	_, err := r.pool.Exec(ctx, `UPDATE leads SET city = $1, state = $2, updated_at = $3
		WHERE form_id = ANY($4) AND city IS NULL`, city, state, nowUTC(), formIDs)
	return err
}

// DissociateForms nulls form_id for the campaign delete.
func (r *LeadRepository) DissociateForms(ctx context.Context, formIDs []string) error {
	_, err := r.pool.Exec(ctx, `UPDATE leads SET form_id = NULL, updated_at = $1
		WHERE form_id = ANY($2)`, nowUTC(), formIDs)
	return err
}

// FollowUpCandidates mirrors the follow-up cron filter: asked, never
// confirmed, not cancelled, created over 24h ago.
func (r *LeadRepository) FollowUpCandidates(ctx context.Context, since time.Time) ([]model.Lead, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+leadColumns+` FROM leads
		WHERE confirmation_sent_at IS NOT NULL AND confirmed_at IS NULL
		  AND cancelled_at IS NULL AND created_at <= $1`, since)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Lead])
}

// StampedLeads returns confirmed, uncancelled leads of the campaign
// missing the stamp column (reminder/voucher crons).
func (r *LeadRepository) StampedLeads(ctx context.Context, campaignID, missingColumn string) ([]model.Lead, error) {
	allowed := map[string]bool{"reminder_event_sent_at": true, "voucher_sent_at": true}
	if !allowed[missingColumn] {
		return nil, fmt.Errorf("coluna inválida: %s", missingColumn)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+leadColumns+` FROM leads l
		WHERE l.confirmed_at IS NOT NULL AND l.cancelled_at IS NULL
		  AND l.`+missingColumn+` IS NULL
		  AND l.form_id IN (SELECT id FROM forms WHERE campaign_id = $1)`, campaignID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Lead])
}

// CountByCampaign counts leads attached to the campaign forms.
func (r *LeadRepository) CountByCampaign(ctx context.Context, campaignID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM leads l
		JOIN forms f ON f.id = l.form_id WHERE f.campaign_id = $1`, campaignID).Scan(&n)
	return n, err
}
