package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"igb-leads-go/config"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// AuthHandler mirrors AuthController: register, login, me, Google OAuth,
// account linking, password recovery and token refresh.
type AuthHandler struct {
	cfg           *config.Config
	users         *repository.UserRepository
	regions       *repository.RegionRepository
	tokens        *service.TokenService
	captcha       *service.CaptchaVerifier
	email         *service.EmailService
	googleLogin   *service.GoogleOAuth
	googleConnect *service.GoogleOAuth
}

func NewAuthHandler(cfg *config.Config, users *repository.UserRepository, regions *repository.RegionRepository, tokens *service.TokenService, captcha *service.CaptchaVerifier, email *service.EmailService) *AuthHandler {
	return &AuthHandler{
		cfg:           cfg,
		users:         users,
		regions:       regions,
		tokens:        tokens,
		captcha:       captcha,
		email:         email,
		googleLogin:   service.NewGoogleOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL()),
		googleConnect: service.NewGoogleOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleConnectRedirectURL()),
	}
}

// Captcha is the verifyCaptcha middleware: it reads cfTurnstileToken
// from the JSON body (restoring it) and validates with Cloudflare.
func (h *AuthHandler) Captcha(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "Corpo inválido")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		var body struct {
			CfTurnstileToken string `json:"cfTurnstileToken"`
		}
		_ = json.Unmarshal(raw, &body)
		if err := h.captcha.Verify(body.CfTurnstileToken); err != nil {
			if service.IsCaptchaError(err) {
				WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			log.Printf("CAPTCHA: erro na verificação: %v", err)
			WriteError(w, http.StatusInternalServerError, "Erro ao validar CAPTCHA")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Whatsapp  string `json:"whatsapp"`
		State     string `json:"state"`
		City      string `json:"city"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	for _, req := range []struct{ field, value string }{
		{"Email", body.Email}, {"Senha", body.Password},
		{"Nome", body.FirstName}, {"Sobrenome", body.LastName}, {"Whatsapp", body.Whatsapp},
	} {
		if strings.TrimSpace(req.value) == "" {
			WriteError(w, http.StatusBadRequest, req.field+" é obrigatório")
			return
		}
	}

	if _, err := h.users.ByEmail(r.Context(), body.Email); err == nil {
		WriteError(w, http.StatusBadRequest, "Email já cadastrado")
		return
	} else if !isNotFound(err) {
		log.Printf("Erro no registro: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}

	hash, err := service.HashPassword(body.Password, h.cfg.BcryptCost)
	if err != nil {
		log.Printf("Erro no registro: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	now := time.Now()
	user := &model.User{
		ID:        uuid.NewString(),
		Email:     body.Email,
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Whatsapp:  service.DigitsOnly(body.Whatsapp),
		Role:      model.RoleUser,
		Status:    model.UserPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	user.PasswordHash = &hash
	if body.State != "" {
		user.State = &body.State
	}
	if body.City != "" {
		user.City = &body.City
	}
	if err := h.users.Create(r.Context(), user); err != nil {
		log.Printf("Erro no registro: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{
		"message": "Cadastro realizado com sucesso! Aguarde a aprovação.",
		"user": map[string]any{
			"id": user.ID, "email": user.Email,
			"first_name": user.FirstName, "last_name": user.LastName,
			"status": user.Status,
		},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	user, err := h.users.ByEmail(r.Context(), body.Email)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "Credenciais inválidas")
		return
	}
	if user.Status == model.UserPending {
		WriteJSON(w, http.StatusForbidden, map[string]string{
			"error":  "Sua conta ainda não foi aprovada. Aguarde a aprovação.",
			"status": "pending",
		})
		return
	}
	if user.Status == model.UserSuspended {
		WriteJSON(w, http.StatusForbidden, map[string]string{
			"error":  "Sua conta está suspensa. Entre em contato com o suporte.",
			"status": "suspended",
		})
		return
	}
	if user.PasswordHash == nil || !service.CheckPassword(body.Password, *user.PasswordHash) {
		WriteError(w, http.StatusUnauthorized, "Credenciais inválidas")
		return
	}
	token, err := h.tokens.SignAccess(user.ID, user.Email, user.Role)
	if err != nil {
		log.Printf("Erro no login: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	refreshToken, err := h.tokens.SignRefresh(user.ID)
	if err != nil {
		log.Printf("Erro no login: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"token": token, "refreshToken": refreshToken,
		"user": map[string]any{
			"id": user.ID, "email": user.Email,
			"first_name": user.FirstName, "last_name": user.LastName,
			"role": user.Role, "status": user.Status,
		},
	})
}

// Me returns the authenticated user with the region included.
// userID is resolved by the caller (auth middleware) via context.
func (h *AuthHandler) Me(userID func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := h.users.ByID(r.Context(), userID(r))
		if err != nil {
			WriteError(w, http.StatusNotFound, "Usuário não encontrado")
			return
		}
		WriteJSON(w, http.StatusOK, h.withRegion(r, user))
	}
}

// Google starts the OAuth login flow.
func (h *AuthHandler) Google(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.GoogleConfigured() {
		http.Redirect(w, r, h.cfg.FrontendURL+"/login?error=oauth_disabled", http.StatusFound)
		return
	}
	http.Redirect(w, r, h.googleLogin.AuthURL(), http.StatusFound)
}

// GoogleCallback finishes the OAuth login flow.
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	fail := func() {
		http.Redirect(w, r, h.cfg.FrontendURL+"/login?error=oauth_error", http.StatusFound)
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		fail()
		return
	}
	profile, err := h.googleLogin.ExchangeProfile(r.Context(), code)
	if err != nil {
		log.Printf("Erro no callback Google: %v", err)
		fail()
		return
	}
	user, err := h.users.ByGoogleID(r.Context(), profile.ID)
	if err != nil {
		if !isNotFound(err) {
			log.Printf("Erro no callback Google: %v", err)
			fail()
			return
		}
		existing, err := h.users.ByEmail(r.Context(), profile.Email)
		if err != nil {
			if !isNotFound(err) {
				log.Printf("Erro no callback Google: %v", err)
				fail()
				return
			}
			now := time.Now()
			user = &model.User{
				ID: uuid.NewString(), Email: profile.Email,
				FirstName: profile.FirstName, LastName: profile.LastName,
				Whatsapp: "", Role: model.RoleUser, Status: model.UserPending,
				CreatedAt: now, UpdatedAt: now,
			}
			user.GoogleID = &profile.ID
			if profile.AvatarURL != "" {
				user.AvatarURL = &profile.AvatarURL
			}
			if err := h.users.Create(r.Context(), user); err != nil {
				log.Printf("Erro no callback Google: %v", err)
				fail()
				return
			}
		} else {
			fields := map[string]any{"google_id": profile.ID}
			if profile.AvatarURL != "" {
				fields["avatar_url"] = profile.AvatarURL
			}
			if err := h.users.UpdateFields(r.Context(), existing.ID, fields); err != nil {
				log.Printf("Erro no callback Google: %v", err)
				fail()
				return
			}
			existing.GoogleID = &profile.ID
			user = existing
		}
	}
	token, err := h.tokens.SignAccess(user.ID, user.Email, user.Role)
	if err != nil {
		log.Printf("Erro no callback Google: %v", err)
		fail()
		return
	}
	if user.Status == model.UserPending {
		http.Redirect(w, r, h.cfg.FrontendURL+"/pending-approval", http.StatusFound)
		return
	}
	http.Redirect(w, r, h.cfg.FrontendURL+"/auth/callback?token="+token, http.StatusFound)
}

// Connect starts Google account linking for the authenticated user.
func (h *AuthHandler) Connect(userID func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, h.googleConnect.ConnectURL(userID(r)), http.StatusFound)
	}
}

// ConnectCallback finishes Google account linking.
func (h *AuthHandler) ConnectCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	state := q.Get("state")
	if state == "" || !strings.HasPrefix(state, "connect:") {
		http.Redirect(w, r, h.cfg.FrontendURL+"/settings?error=invalid_state", http.StatusFound)
		return
	}
	userID := strings.TrimPrefix(state, "connect:")
	profile, err := h.googleConnect.ExchangeProfile(r.Context(), q.Get("code"))
	if err != nil {
		log.Printf("Erro ao conectar Google: %v", err)
		http.Redirect(w, r, h.cfg.FrontendURL+"/settings?error=token_exchange", http.StatusFound)
		return
	}
	existing, err := h.users.ByGoogleID(r.Context(), profile.ID)
	if err != nil && !isNotFound(err) {
		log.Printf("Erro ao conectar Google: %v", err)
		http.Redirect(w, r, h.cfg.FrontendURL+"/settings?error=connect_failed", http.StatusFound)
		return
	}
	if existing != nil && existing.ID != userID {
		http.Redirect(w, r, h.cfg.FrontendURL+"/settings?error=google_in_use", http.StatusFound)
		return
	}
	fields := map[string]any{"google_id": profile.ID}
	if profile.AvatarURL != "" {
		fields["avatar_url"] = profile.AvatarURL
	} else {
		fields["avatar_url"] = nil
	}
	if err := h.users.UpdateFields(r.Context(), userID, fields); err != nil {
		log.Printf("Erro ao conectar Google: %v", err)
		http.Redirect(w, r, h.cfg.FrontendURL+"/settings?error=connect_failed", http.StatusFound)
		return
	}
	http.Redirect(w, r, h.cfg.FrontendURL+"/settings?google=connected", http.StatusFound)
}

// Disconnect unlinks the Google account.
// NOTE: the Node route reads an inexistent :provider param and always
// answers 400; here the intended behavior is implemented.
func (h *AuthHandler) Disconnect(userID func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h.users.UpdateFields(r.Context(), userID(r), map[string]any{"google_id": nil}); err != nil {
			log.Printf("Erro ao desconectar provedor: %v", err)
			WriteError(w, http.StatusInternalServerError, "Erro ao desconectar")
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"message": "Conta Google desconectada"})
	}
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	done := map[string]string{"message": "Se o email existir, enviaremos um link de recuperação."}
	user, err := h.users.ByEmail(r.Context(), body.Email)
	if err != nil {
		WriteJSON(w, http.StatusOK, done)
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		log.Printf("Erro no forgot password: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	token := hex.EncodeToString(raw)
	expiry := time.Now().Add(time.Hour)
	if err := h.users.UpdateFields(r.Context(), user.ID, map[string]any{
		"reset_password_token": token, "reset_password_expires": expiry,
	}); err != nil {
		log.Printf("Erro no forgot password: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	resetURL := strings.TrimSuffix(h.cfg.FrontendURL, "/") + "/reset-password?token=" + token
	if err := h.email.SendPasswordResetEmail(user.Email, user.FirstName, resetURL); err != nil {
		log.Printf("Erro no forgot password: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, done)
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	if body.Token == "" || body.Password == "" {
		WriteError(w, http.StatusBadRequest, "Token e senha são obrigatórios")
		return
	}
	user, err := h.users.ByResetToken(r.Context(), body.Token)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "Token inválido ou expirado")
		return
	}
	hash, err := service.HashPassword(body.Password, h.cfg.BcryptCost)
	if err != nil {
		log.Printf("Erro no reset password: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	if err := h.users.UpdateFields(r.Context(), user.ID, map[string]any{
		"password_hash": hash, "reset_password_token": nil, "reset_password_expires": nil,
	}); err != nil {
		log.Printf("Erro no reset password: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "Senha redefinida com sucesso"})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	if body.RefreshToken == "" {
		WriteError(w, http.StatusBadRequest, "Refresh token é obrigatório")
		return
	}
	claims, err := h.tokens.VerifyRefresh(body.RefreshToken)
	if err != nil {
		if err == service.ErrTokenExpired {
			WriteError(w, http.StatusUnauthorized, "Refresh token expirado. Faça login novamente.")
			return
		}
		WriteError(w, http.StatusUnauthorized, "Refresh token inválido.")
		return
	}
	if claims.Type != "refresh" {
		WriteError(w, http.StatusUnauthorized, "Token inválido")
		return
	}
	user, err := h.users.ByID(r.Context(), claims.ID)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "Usuário não encontrado")
		return
	}
	if user.Status != model.UserActive {
		WriteError(w, http.StatusUnauthorized, "Usuário não está ativo")
		return
	}
	token, err := h.tokens.SignAccess(user.ID, user.Email, user.Role)
	if err != nil {
		log.Printf("Erro no refresh: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	refreshToken, err := h.tokens.SignRefresh(user.ID)
	if err != nil {
		log.Printf("Erro no refresh: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"token": token, "refreshToken": refreshToken})
}

// withRegion attaches the full region (or null) like Sequelize include.
func (h *AuthHandler) withRegion(r *http.Request, user *model.User) any {
	return withRegion(r.Context(), h.regions, user, false)
}

func isNotFound(err error) bool {
	return err == pgx.ErrNoRows
}
