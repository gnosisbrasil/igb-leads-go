package middleware

import (
	"net/http"
	"strings"

	"igb-leads-go/authctx"
	"igb-leads-go/handler"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// Auth verifies the Bearer token, loads the user and requires active status.
func Auth(users *repository.UserRepository, tokens *service.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				handler.WriteError(w, http.StatusUnauthorized, "Token não fornecido")
				return
			}
			parts := strings.Split(header, " ")
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				handler.WriteError(w, http.StatusUnauthorized, "Token mal formatado")
				return
			}
			claims, err := tokens.VerifyAccess(parts[1])
			if err != nil {
				handler.WriteError(w, http.StatusUnauthorized, "Token inválido")
				return
			}
			user, err := users.ByID(r.Context(), claims.ID)
			if err != nil {
				handler.WriteError(w, http.StatusUnauthorized, "Usuário não encontrado")
				return
			}
			if user.Status != model.UserActive {
				handler.WriteError(w, http.StatusUnauthorized, "Usuário não está ativo")
				return
			}
			next.ServeHTTP(w, r.WithContext(authctx.WithUser(r.Context(), user)))
		})
	}
}

// RoleCheck allows only the listed roles.
func RoleCheck(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authctx.UserID(r) == "" {
				handler.WriteError(w, http.StatusUnauthorized, "Usuário não autenticado")
				return
			}
			if !allowed[authctx.UserRole(r)] {
				handler.WriteError(w, http.StatusForbidden, "Acesso negado. Permissão insuficiente.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
