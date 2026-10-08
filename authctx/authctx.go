// Package authctx carries the authenticated identity through the
// request context. It sits below handler and middleware so neither
// direction creates an import cycle.
package authctx

import (
	"context"
	"net/http"

	"igb-leads-go/model"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxUserRole
	ctxUser
)

// WithUser stores the authenticated user in the context.
func WithUser(ctx context.Context, user *model.User) context.Context {
	ctx = context.WithValue(ctx, ctxUserID, user.ID)
	ctx = context.WithValue(ctx, ctxUserRole, user.Role)
	return context.WithValue(ctx, ctxUser, user)
}

// UserID returns the authenticated user id, or "" when absent.
func UserID(r *http.Request) string {
	if id, ok := r.Context().Value(ctxUserID).(string); ok {
		return id
	}
	return ""
}

// UserRole returns the authenticated user role, or "" when absent.
func UserRole(r *http.Request) string {
	if role, ok := r.Context().Value(ctxUserRole).(string); ok {
		return role
	}
	return ""
}

// CurrentUser returns the authenticated user, or nil when absent.
func CurrentUser(r *http.Request) *model.User {
	if u, ok := r.Context().Value(ctxUser).(*model.User); ok {
		return u
	}
	return nil
}
