package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/model"
)

// NotificationRepository covers notification writes used across phases;
// the read controller arrives in phase 4.
type NotificationRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

// Create inserts a notification; failures are logged like the Node service.
func (r *NotificationRepository) Create(ctx context.Context, userID, title, message, typ string, entityType, entityID *string) {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `INSERT INTO notifications
		(id, user_id, title, message, type, entity_type, entity_id, is_read, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,false,$8,$8)`,
		uuid.NewString(), userID, title, message, typ, entityType, entityID, now)
	if err != nil {
		log.Printf("Erro ao criar notificação: %v", err)
	}
}

// NotifyUserApproval mirrors NotificationService.notifyUserApproval.
func (r *NotificationRepository) NotifyUserApproval(ctx context.Context, userID string) {
	entity := "user"
	r.Create(ctx, userID,
		"Conta Aprovada!",
		"Sua conta foi aprovada. Você já pode acessar o sistema e criar campanhas.",
		"success", &entity, &userID)
}

// NotifyPaymentConfirmed mirrors notifyPaymentConfirmed.
func (r *NotificationRepository) NotifyPaymentConfirmed(ctx context.Context, campaignID, title, userID string) {
	entity := "campaign"
	r.Create(ctx, userID,
		"Pagamento Confirmado",
		`O pagamento da campanha "`+title+`" foi confirmado. Envie para aprovação.`,
		"success", &entity, &campaignID)
}

// NotifyCampaignAssigned mirrors notifyCampaignAssigned.
func (r *NotificationRepository) NotifyCampaignAssigned(ctx context.Context, campaignID, title, executiveID string) {
	entity := "campaign"
	r.Create(ctx, executiveID,
		"Nova Campanha Atribuída",
		`A campanha "`+title+`" foi atribuída a você.`,
		"info", &entity, &campaignID)
}

// NotifyCampaignRejection mirrors notifyCampaignRejection.
func (r *NotificationRepository) NotifyCampaignRejection(ctx context.Context, campaignID, title, userID, reason string) {
	if reason == "" {
		reason = "Não informado"
	}
	entity := "campaign"
	r.Create(ctx, userID,
		"Campanha Rejeitada",
		`Sua campanha "`+title+`" foi rejeitada. Motivo: `+reason+`. Você pode editar e reenviar.`,
		"warning", &entity, &campaignID)
}

// NotifyEditProposed mirrors notifyEditProposed.
func (r *NotificationRepository) NotifyEditProposed(ctx context.Context, users *UserRepository, campaignID, title string, trafficManagerID, regionID *string) {
	targets := []string{}
	seen := map[string]bool{}
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			targets = append(targets, id)
		}
	}
	if trafficManagerID != nil {
		add(*trafficManagerID)
	}
	if sup, err := users.SupervisorByRegion(ctx, regionID); err == nil && sup != nil {
		add(*sup)
	}
	if admins, err := users.AdminIDs(ctx); err == nil {
		for _, id := range admins {
			add(id)
		}
	}
	entity := "campaign"
	for _, id := range targets {
		r.Create(ctx, id,
			"Alterações propostas: "+title,
			`O usuário propôs alterações na campanha "`+title+`".`,
			"warning", &entity, &campaignID)
	}
}

// NotifyTopUp mirrors NotificationService.notifyTopUp.
func (r *NotificationRepository) NotifyTopUp(ctx context.Context, users *UserRepository, campaignID, title string, trafficManagerID, regionID *string, ownerID string, amount float64) {
	targets := []string{}
	seen := map[string]bool{}
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			targets = append(targets, id)
		}
	}
	if trafficManagerID != nil {
		add(*trafficManagerID)
	}
	if sup, err := users.SupervisorByRegion(ctx, regionID); err == nil && sup != nil {
		add(*sup)
	}
	add(ownerID)
	if admins, err := users.AdminIDs(ctx); err == nil {
		for _, id := range admins {
			add(id)
		}
	}
	entity := "campaign"
	msg := "Crédito adicional de R$ " + formatBRL(amount) + " adicionado à campanha \"" + title + "\"."
	for _, id := range targets {
		r.Create(ctx, id, "Crédito adicional: "+title, msg, "info", &entity, &campaignID)
	}
}

func formatBRL(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

const notificationColumns = `id, user_id, title, message, type, entity_type,
	entity_id, is_read, created_at, updated_at`

// List returns a user's notifications newest-first with a total count.
func (r *NotificationRepository) List(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]model.Notification, int, error) {
	where := `user_id = $1`
	args := []any{userID}
	if unreadOnly {
		where += ` AND is_read = false`
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, `SELECT `+notificationColumns+` FROM notifications WHERE `+where+
		` ORDER BY created_at DESC LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Notification])
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UnreadCount counts a user's unread notifications.
func (r *NotificationRepository) UnreadCount(ctx context.Context, userID string) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`, userID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ByID fetches one notification by id.
func (r *NotificationRepository) ByID(ctx context.Context, id string) (*model.Notification, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+notificationColumns+` FROM notifications WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	n, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Notification])
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// MarkRead flags one notification as read and returns it.
func (r *NotificationRepository) MarkRead(ctx context.Context, id string) (*model.Notification, error) {
	rows, err := r.pool.Query(ctx, `UPDATE notifications SET is_read = true, updated_at = $1
		WHERE id = $2 RETURNING `+notificationColumns, time.Now().UTC(), id)
	if err != nil {
		return nil, err
	}
	n, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Notification])
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// MarkAllRead flags every unread notification of a user as read.
func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE notifications SET is_read = true, updated_at = $1
		WHERE user_id = $2 AND is_read = false`, time.Now().UTC(), userID)
	return err
}
