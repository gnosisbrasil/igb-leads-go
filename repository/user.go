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

// UserRepository covers users; region joins are assembled in handlers.
type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

const userColumns = `id, email, password_hash, first_name, last_name, whatsapp,
	role, status, region_id, approved_by, approved_at, google_id, facebook_id,
	avatar_url, created_at, updated_at, reset_password_token, reset_password_expires,
	state, city`

func (r *UserRepository) ByID(ctx context.Context, id string) (*model.User, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.User])
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) ByEmail(ctx context.Context, email string) (*model.User, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email)
	if err != nil {
		return nil, err
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.User])
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) ByGoogleID(ctx context.Context, googleID string) (*model.User, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+` FROM users WHERE google_id = $1`, googleID)
	if err != nil {
		return nil, err
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.User])
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) ByResetToken(ctx context.Context, token string) (*model.User, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+` FROM users
		WHERE reset_password_token = $1 AND reset_password_expires > now()`, token)
	if err != nil {
		return nil, err
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.User])
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Create inserts a user; passwordHash may be nil (OAuth users).
func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO users
		(id, email, password_hash, first_name, last_name, whatsapp, role, status,
		 region_id, approved_by, approved_at, google_id, facebook_id, avatar_url,
		 created_at, updated_at, reset_password_token, reset_password_expires, state, city)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		u.ID, u.Email, u.PasswordHash, u.FirstName, u.LastName, u.Whatsapp,
		u.Role, u.Status, u.RegionID, u.ApprovedBy, u.ApprovedAt, u.GoogleID,
		u.FacebookID, u.AvatarURL, u.CreatedAt, u.UpdatedAt, u.ResetToken,
		u.ResetExpires, u.State, u.City)
	return err
}

// userUpdatable whitelists columns for dynamic updates.
var userUpdatable = map[string]bool{
	"email": true, "password_hash": true, "first_name": true, "last_name": true,
	"whatsapp": true, "role": true, "status": true, "region_id": true,
	"approved_by": true, "approved_at": true, "google_id": true,
	"facebook_id": true, "avatar_url": true, "reset_password_token": true,
	"reset_password_expires": true, "state": true, "city": true,
}

// UpdateFields applies a whitelisted partial update and bumps updated_at.
func (r *UserRepository) UpdateFields(ctx context.Context, id string, fields map[string]any) error {
	set := make([]string, 0, len(fields)+1)
	args := make([]any, 0, len(fields)+2)
	i := 1
	for col, val := range fields {
		if !userUpdatable[col] {
			return fmt.Errorf("coluna não atualizável: %s", col)
		}
		set = append(set, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	set = append(set, fmt.Sprintf("updated_at = $%d", i))
	args = append(args, time.Now())
	args = append(args, id)
	_, err := r.pool.Exec(ctx,
		fmt.Sprintf(`UPDATE users SET %s WHERE id = $%d`, strings.Join(set, ", "), i+1),
		args...)
	return err
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

// ListFilter mirrors the Node list query params.
type ListFilter struct {
	Status   string
	Role     string
	RegionID string
	Page     int
	Limit    int
}

// List returns the page plus the total count, newest first.
func (r *UserRepository) List(ctx context.Context, f ListFilter) ([]model.User, int, error) {
	where := []string{}
	args := []any{}
	if f.Status != "" {
		args = append(args, f.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.Role != "" {
		args = append(args, f.Role)
		where = append(where, fmt.Sprintf("role = $%d", len(args)))
	}
	if f.RegionID != "" {
		args = append(args, f.RegionID)
		where = append(where, fmt.Sprintf("region_id = $%d", len(args)))
	}
	w := ""
	if len(where) > 0 {
		w = "WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users `+w, args...).Scan(&total); err != nil {
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
	q := fmt.Sprintf(`SELECT %s FROM users %s ORDER BY created_at DESC LIMIT %d OFFSET %d`,
		userColumns, w, limit, offset)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.User])
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// ListPending returns pending users, oldest first.
func (r *UserRepository) ListPending(ctx context.Context) ([]model.User, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+` FROM users
		WHERE status = 'pending' ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[model.User])
}

// CountByRegion counts users in a region (region delete guard).
func (r *UserRepository) CountByRegion(ctx context.Context, regionID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE region_id = $1`, regionID).Scan(&n)
	return n, err
}

// RegionIDOf returns the user's region (requester scoping).
func (r *UserRepository) RegionIDOf(ctx context.Context, userID string) (*string, error) {
	var regionID *string
	err := r.pool.QueryRow(ctx, `SELECT region_id FROM users WHERE id = $1`, userID).Scan(&regionID)
	return regionID, err
}

// UserMini is the user projection embedded in campaign payloads.
type UserMini struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email,omitempty"`
}

// MinisByIDs fetches user minis for joins.
func (r *UserRepository) MinisByIDs(ctx context.Context, ids []string) (map[string]*UserMini, error) {
	out := map[string]*UserMini{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT id, first_name, last_name, email FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m UserMini
		if err := rows.Scan(&m.ID, &m.FirstName, &m.LastName, &m.Email); err != nil {
			return nil, err
		}
		out[m.ID] = &m
	}
	return out, rows.Err()
}

// SupervisorByRegion returns a supervisor id of the region, if any.
func (r *UserRepository) SupervisorByRegion(ctx context.Context, regionID *string) (*string, error) {
	if regionID == nil {
		return nil, nil
	}
	var id *string
	err := r.pool.QueryRow(ctx, `SELECT id FROM users WHERE region_id = $1 AND role = 'supervisor' LIMIT 1`, *regionID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return id, nil
}

// ByIDs fetches full users for the given ids.
func (r *UserRepository) ByIDs(ctx context.Context, ids []string) (map[string]*model.User, error) {
	out := map[string]*model.User{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+` FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.User])
	if err != nil {
		return nil, err
	}
	for i := range users {
		out[users[i].ID] = &users[i]
	}
	return out, nil
}

// AdminIDs returns every admin id.
func (r *UserRepository) AdminIDs(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM users WHERE role = 'admin'`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
