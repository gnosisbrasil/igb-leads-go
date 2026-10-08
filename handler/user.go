package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"igb-leads-go/authctx"
	"igb-leads-go/config"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// RegionMini is the region projection embedded in user lists.
type RegionMini struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// UserWithRegion mirrors a Sequelize user with the region included.
type UserWithRegion struct {
	model.User
	Region *model.Region `json:"region"`
}

// UserWithRegionMini mirrors a user list row (mini region).
type UserWithRegionMini struct {
	model.User
	Region *RegionMini `json:"region"`
}

func toMini(r *model.Region) *RegionMini {
	if r == nil {
		return nil
	}
	return &RegionMini{ID: r.ID, Name: r.Name, Code: r.Code}
}

// withRegion attaches the region (or null) to a user.
func withRegion(ctx context.Context, regions *repository.RegionRepository, user *model.User, mini bool) any {
	if user.RegionID == nil {
		if mini {
			return UserWithRegionMini{User: *user}
		}
		return UserWithRegion{User: *user}
	}
	region, err := regions.ByID(ctx, *user.RegionID)
	if err != nil {
		region = nil
	}
	if mini {
		return UserWithRegionMini{User: *user, Region: toMini(region)}
	}
	return UserWithRegion{User: *user, Region: region}
}

// UserHandler mirrors UserController.
type UserHandler struct {
	cfg     *config.Config
	users   *repository.UserRepository
	regions *repository.RegionRepository
	notify  *repository.NotificationRepository
}

func NewUserHandler(cfg *config.Config, users *repository.UserRepository, regions *repository.RegionRepository, notify *repository.NotificationRepository) *UserHandler {
	return &UserHandler{cfg: cfg, users: users, regions: regions, notify: notify}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	f := repository.ListFilter{
		Status: q.Get("status"), Role: q.Get("role"), RegionID: q.Get("region_id"),
		Page: page, Limit: limit,
	}
	role := authctx.UserRole(r)
	if role == model.RoleExecutive {
		f.Role = model.RoleUser
		regionID, err := h.users.RegionIDOf(r.Context(), authctx.UserID(r))
		if err != nil {
			log.Printf("Erro ao listar usuários: %v", err)
			WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
			return
		}
		if regionID != nil {
			f.RegionID = *regionID
		}
	} else if role == model.RoleSupervisor {
		regionID, err := h.users.RegionIDOf(r.Context(), authctx.UserID(r))
		if err != nil {
			log.Printf("Erro ao listar usuários: %v", err)
			WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
			return
		}
		if regionID != nil {
			f.RegionID = *regionID
		}
	}
	users, total, err := h.users.List(r.Context(), f)
	if err != nil {
		log.Printf("Erro ao listar usuários: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	ids := []string{}
	for i := range users {
		if users[i].RegionID != nil {
			ids = append(ids, *users[i].RegionID)
		}
	}
	regions, err := h.regions.ByIDs(r.Context(), ids)
	if err != nil {
		log.Printf("Erro ao listar usuários: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	rows := make([]any, 0, len(users))
	for i := range users {
		var mini *RegionMini
		if users[i].RegionID != nil {
			mini = toMini(regions[*users[i].RegionID])
		}
		rows = append(rows, UserWithRegionMini{User: users[i], Region: mini})
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"data": rows,
		"pagination": map[string]any{
			"page": f.Page, "limit": f.Limit, "total": total,
			"totalPages": (total + f.Limit - 1) / f.Limit,
		},
	})
}

func (h *UserHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.ListPending(r.Context())
	if err != nil {
		log.Printf("Erro ao listar usuários pendentes: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	ids := []string{}
	for i := range users {
		if users[i].RegionID != nil {
			ids = append(ids, *users[i].RegionID)
		}
	}
	regions, err := h.regions.ByIDs(r.Context(), ids)
	if err != nil {
		log.Printf("Erro ao listar usuários pendentes: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	rows := make([]any, 0, len(users))
	for i := range users {
		var mini *RegionMini
		if users[i].RegionID != nil {
			mini = toMini(regions[*users[i].RegionID])
		}
		rows = append(rows, UserWithRegionMini{User: users[i], Region: mini})
	}
	WriteJSON(w, http.StatusOK, rows)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FirstName string  `json:"first_name"`
		LastName  string  `json:"last_name"`
		Email     string  `json:"email"`
		Password  string  `json:"password"`
		Whatsapp  string  `json:"whatsapp"`
		Role      string  `json:"role"`
		RegionID  *string `json:"region_id"`
		City      string  `json:"city"`
		State     string  `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	requesterRole := authctx.UserRole(r)
	requesterID := authctx.UserID(r)
	var requesterRegionID *string
	if requesterRole != model.RoleAdmin {
		regionID, err := h.users.RegionIDOf(r.Context(), requesterID)
		if err != nil {
			log.Printf("Erro ao criar usuário: %v", err)
			WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
			return
		}
		requesterRegionID = regionID
	}
	allowed := map[string][]string{
		model.RoleAdmin:      {model.RoleAdmin, model.RoleSupervisor, model.RoleExecutive, model.RoleUser},
		model.RoleSupervisor: {model.RoleExecutive, model.RoleUser},
		model.RoleExecutive:  {model.RoleUser},
	}
	ok := false
	for _, role := range allowed[requesterRole] {
		if role == body.Role {
			ok = true
			break
		}
	}
	if !ok {
		WriteError(w, http.StatusForbidden, "Você não tem permissão para criar este tipo de usuário")
		return
	}
	if requesterRole != model.RoleAdmin {
		if requesterRegionID == nil {
			WriteError(w, http.StatusBadRequest, "Você precisa ter uma região definida para criar usuários")
			return
		}
		if body.RegionID != nil && *body.RegionID != *requesterRegionID {
			WriteError(w, http.StatusForbidden, "Você só pode criar usuários na sua própria região")
			return
		}
	}
	if _, err := h.users.ByEmail(r.Context(), body.Email); err == nil {
		WriteError(w, http.StatusBadRequest, "Email já cadastrado")
		return
	} else if !isNotFound(err) {
		log.Printf("Erro ao criar usuário: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	hash, err := service.HashPassword(body.Password, h.cfg.BcryptCost)
	if err != nil {
		log.Printf("Erro ao criar usuário: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	var effectiveRegionID *string
	if requesterRole == model.RoleAdmin {
		effectiveRegionID = body.RegionID
	} else {
		effectiveRegionID = requesterRegionID
	}
	now := now()
	user := &model.User{
		ID: uuid.NewString(), FirstName: body.FirstName, LastName: body.LastName,
		Email: body.Email, Role: body.Role, RegionID: effectiveRegionID,
		Status: model.UserActive, CreatedAt: now, UpdatedAt: now,
	}
	if user.Role == "" {
		user.Role = model.RoleUser
	}
	user.PasswordHash = &hash
	if body.Whatsapp != "" {
		user.Whatsapp = service.DigitsOnly(body.Whatsapp)
	}
	if body.City != "" {
		user.City = &body.City
	}
	if body.State != "" {
		user.State = &body.State
	}
	if err := h.users.Create(r.Context(), user); err != nil {
		log.Printf("Erro ao criar usuário: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{
		"id": user.ID, "first_name": user.FirstName, "last_name": user.LastName,
		"email": user.Email, "role": user.Role, "status": user.Status,
	})
}

func (h *UserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	user, err := h.users.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Usuário não encontrado")
		return
	}
	WriteJSON(w, http.StatusOK, withRegion(r.Context(), h.regions, user, false))
}

func (h *UserHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, err := h.users.ByID(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Usuário não encontrado")
		return
	}
	if user.Status != model.UserPending {
		WriteError(w, http.StatusBadRequest, "Usuário não está pendente")
		return
	}
	now := now()
	if err := h.users.UpdateFields(r.Context(), id, map[string]any{
		"status": model.UserActive, "approved_by": authctx.UserID(r), "approved_at": now,
	}); err != nil {
		log.Printf("Erro ao aprovar usuário: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	user.Status = model.UserActive
	h.notify.NotifyUserApproval(r.Context(), user.ID)
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Usuário aprovado com sucesso",
		"user": map[string]any{
			"id": user.ID, "email": user.Email,
			"first_name": user.FirstName, "status": user.Status,
		},
	})
}

func (h *UserHandler) Suspend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.users.ByID(r.Context(), id); err != nil {
		WriteError(w, http.StatusNotFound, "Usuário não encontrado")
		return
	}
	if err := h.users.UpdateFields(r.Context(), id, map[string]any{"status": model.UserSuspended}); err != nil {
		log.Printf("Erro ao suspender usuário: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Usuário suspenso com sucesso",
		"user":    map[string]any{"id": id, "status": model.UserSuspended},
	})
}

func (h *UserHandler) Reactivate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.users.ByID(r.Context(), id); err != nil {
		WriteError(w, http.StatusNotFound, "Usuário não encontrado")
		return
	}
	if err := h.users.UpdateFields(r.Context(), id, map[string]any{"status": model.UserActive}); err != nil {
		log.Printf("Erro ao reativar usuário: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Usuário reativado com sucesso",
		"user":    map[string]any{"id": id, "status": model.UserActive},
	})
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	user, err := h.users.ByID(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Usuário não encontrado")
		return
	}
	role, requesterID := authctx.UserRole(r), authctx.UserID(r)
	if role != model.RoleAdmin && requesterID != id {
		requesterRegionID, err := h.users.RegionIDOf(r.Context(), requesterID)
		if err != nil {
			log.Printf("Erro ao atualizar usuário: %v", err)
			WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
			return
		}
		if user.Role != model.RoleUser {
			WriteError(w, http.StatusForbidden, "Você só pode editar usuários comuns")
			return
		}
		if requesterRegionID != nil && (user.RegionID == nil || *user.RegionID != *requesterRegionID) {
			WriteError(w, http.StatusForbidden, "Você só pode editar usuários da sua região")
			return
		}
	}
	fields := map[string]any{}
	str := func(key string) (string, bool) {
		v, present := raw[key]
		if !present {
			return "", false
		}
		s, _ := v.(string)
		return s, true
	}
	if s, present := str("first_name"); present && s != "" {
		fields["first_name"] = s
	}
	if s, present := str("last_name"); present && s != "" {
		fields["last_name"] = s
	}
	if s, present := str("whatsapp"); present && s != "" {
		fields["whatsapp"] = service.DigitsOnly(s)
	}
	if s, present := str("state"); present && s != "" {
		fields["state"] = s
	}
	if s, present := str("city"); present && s != "" {
		fields["city"] = s
	}
	if v, present := raw["avatar_url"]; present {
		fields["avatar_url"] = v
	}
	if role == model.RoleAdmin {
		if s, present := str("role"); present && s != "" {
			fields["role"] = s
		}
		if v, present := raw["region_id"]; present {
			if s, _ := v.(string); s != "" {
				fields["region_id"] = s
			} else {
				fields["region_id"] = nil
			}
		}
	}
	if len(fields) > 0 {
		if err := h.users.UpdateFields(r.Context(), id, fields); err != nil {
			log.Printf("Erro ao atualizar usuário: %v", err)
			WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
			return
		}
	}
	updated, err := h.users.ByID(r.Context(), id)
	if err != nil {
		log.Printf("Erro ao atualizar usuário: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, err := h.users.ByID(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Usuário não encontrado")
		return
	}
	if authctx.UserRole(r) != model.RoleAdmin {
		if user.Role != model.RoleUser {
			WriteError(w, http.StatusForbidden, "Você só pode remover usuários comuns")
			return
		}
		requesterRegionID, err := h.users.RegionIDOf(r.Context(), authctx.UserID(r))
		if err != nil {
			log.Printf("Erro ao deletar usuário: %v", err)
			WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
			return
		}
		if requesterRegionID != nil && (user.RegionID == nil || *user.RegionID != *requesterRegionID) {
			WriteError(w, http.StatusForbidden, "Você só pode remover usuários da sua região")
			return
		}
	}
	if err := h.users.Delete(r.Context(), id); err != nil {
		log.Printf("Erro ao deletar usuário: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "Usuário removido com sucesso"})
}
