package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/model"
)

// ScheduleRepository covers email_schedules.
type ScheduleRepository struct {
	pool *pgxpool.Pool
}

func NewScheduleRepository(pool *pgxpool.Pool) *ScheduleRepository {
	return &ScheduleRepository{pool: pool}
}

const scheduleColumns = `id, campaign_id, event_id, email_type, scheduled_for,
	sent_at, recipient_type, status, error_message, created_at, updated_at`

// DuePending returns pending schedules whose time has come.
func (r *ScheduleRepository) DuePending(ctx context.Context) ([]model.EmailSchedule, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+scheduleColumns+` FROM email_schedules
		WHERE scheduled_for <= now() AND status = 'pending'`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.EmailSchedule])
}

// ExistsScheduled reports whether the type was already scheduled for the
// campaign (pending or sent; failed rows may be retried).
func (r *ScheduleRepository) ExistsScheduled(ctx context.Context, campaignID, emailType string) (bool, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM email_schedules
		WHERE campaign_id = $1 AND email_type = $2 AND status IN ('pending','sent')`,
		campaignID, emailType).Scan(&n)
	return n > 0, err
}

// ExistsPending reports whether a pending schedule of the type exists.
func (r *ScheduleRepository) ExistsPending(ctx context.Context, campaignID, emailType string) (bool, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM email_schedules
		WHERE campaign_id = $1 AND email_type = $2 AND status = 'pending'`,
		campaignID, emailType).Scan(&n)
	return n > 0, err
}

func (r *ScheduleRepository) Create(ctx context.Context, campaignID, emailType string, scheduledFor time.Time) error {
	now := nowUTC()
	_, err := r.pool.Exec(ctx, `INSERT INTO email_schedules
		(id, campaign_id, event_id, email_type, scheduled_for, sent_at,
		 recipient_type, status, error_message, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		uuid.NewString(), campaignID, nil, emailType, scheduledFor, nil,
		"user", "pending", nil, now, now)
	return err
}

// MarkSent stamps the schedule delivered.
func (r *ScheduleRepository) MarkSent(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE email_schedules
		SET status = 'sent', sent_at = $1, updated_at = $1 WHERE id = $2`, nowUTC(), id)
	return err
}

// MarkFailed stamps the schedule failed with the reason.
func (r *ScheduleRepository) MarkFailed(ctx context.Context, id, reason string) error {
	_, err := r.pool.Exec(ctx, `UPDATE email_schedules
		SET status = 'failed', error_message = $1, updated_at = $2 WHERE id = $3`,
		reason, nowUTC(), id)
	return err
}
