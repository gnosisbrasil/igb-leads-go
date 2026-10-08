package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"igb-leads-go/model"
	"igb-leads-go/repository"
)

// RegionHandler mirrors RegionController.
type RegionHandler struct {
	regions *repository.RegionRepository
	users   *repository.UserRepository
}

func NewRegionHandler(regions *repository.RegionRepository, users *repository.UserRepository) *RegionHandler {
	return &RegionHandler{regions: regions, users: users}
}

func (h *RegionHandler) List(w http.ResponseWriter, r *http.Request) {
	var onlyActive *bool
	if v := r.URL.Query().Get("is_active"); v != "" {
		b := v == "true"
		onlyActive = &b
	}
	regions, err := h.regions.List(r.Context(), onlyActive)
	if err != nil {
		log.Printf("Erro ao listar regiões: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	if regions == nil {
		regions = []model.Region{}
	}
	WriteJSON(w, http.StatusOK, regions)
}

func (h *RegionHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	region, err := h.regions.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Região não encontrada")
		return
	}
	WriteJSON(w, http.StatusOK, region)
}

func (h *RegionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Code        string `json:"code"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Code) == "" {
		WriteError(w, http.StatusBadRequest, "Nome e código são obrigatórios")
		return
	}
	if _, err := h.regions.ByCode(r.Context(), body.Code); err == nil {
		WriteError(w, http.StatusBadRequest, "Código de região já existe")
		return
	} else if !isNotFound(err) {
		log.Printf("Erro ao criar região: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	now := time.Now().UTC()
	region := &model.Region{
		ID: uuid.NewString(), Name: body.Name, Code: strings.ToUpper(body.Code),
		Country: "Brasil", IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if body.Description != "" {
		region.Description = &body.Description
	}
	typ := "state"
	region.Type = &typ
	if err := h.regions.Create(r.Context(), region); err != nil {
		log.Printf("Erro ao criar região: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusCreated, region)
}

func (h *RegionHandler) Update(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string  `json:"name"`
		Code        string  `json:"code"`
		Description *string `json:"description"`
		IsActive    *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	region, err := h.regions.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Região não encontrada")
		return
	}
	if body.Code != "" && body.Code != region.Code {
		if _, err := h.regions.ByCode(r.Context(), body.Code); err == nil {
			WriteError(w, http.StatusBadRequest, "Código de região já existe")
			return
		} else if !isNotFound(err) {
			log.Printf("Erro ao atualizar região: %v", err)
			WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
			return
		}
	}
	if body.Name != "" {
		region.Name = body.Name
	}
	if body.Code != "" {
		region.Code = strings.ToUpper(body.Code)
	}
	if body.Description != nil {
		region.Description = body.Description
	}
	if body.IsActive != nil {
		region.IsActive = *body.IsActive
	}
	if err := h.regions.Update(r.Context(), region); err != nil {
		log.Printf("Erro ao atualizar região: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, region)
}

func (h *RegionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.regions.ByID(r.Context(), id); err != nil {
		WriteError(w, http.StatusNotFound, "Região não encontrada")
		return
	}
	userCount, err := h.users.CountByRegion(r.Context(), id)
	if err != nil {
		log.Printf("Erro ao deletar região: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	campaignCount, err := h.regions.CountCampaigns(r.Context(), id)
	if err != nil {
		log.Printf("Erro ao deletar região: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	if userCount > 0 || campaignCount > 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Não é possível excluir: " + itoa(userCount) + " usuários e " + itoa(campaignCount) + " campanhas vinculados",
		})
		return
	}
	if err := h.regions.Delete(r.Context(), id); err != nil {
		log.Printf("Erro ao deletar região: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "Região removida com sucesso"})
}

func (h *RegionHandler) AssignSupervisor(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SupervisorID string `json:"supervisor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	region, err := h.regions.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Região não encontrada")
		return
	}
	supervisor, err := h.users.ByID(r.Context(), body.SupervisorID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Supervisor não encontrado")
		return
	}
	if supervisor.Role != model.RoleSupervisor {
		WriteError(w, http.StatusBadRequest, "Usuário não é supervisor")
		return
	}
	if err := h.users.UpdateFields(r.Context(), supervisor.ID, map[string]any{"region_id": region.ID}); err != nil {
		log.Printf("Erro ao atribuir supervisor: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Supervisor atribuído",
		"region":  region,
		"supervisor": map[string]string{
			"id": supervisor.ID, "name": supervisor.FirstName + " " + supervisor.LastName,
		},
	})
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
