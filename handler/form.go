package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"igb-leads-go/authctx"
	"igb-leads-go/config"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// FormCampaignMini is the campaign projection in form lists.
type FormCampaignMini struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// FormWithCampaignMini mirrors the list row.
type FormWithCampaignMini struct {
	model.Form
	Campaign *FormCampaignMini `json:"campaign"`
}

// FormWithCampaign mirrors getById (full campaign).
type FormWithCampaign struct {
	model.Form
	Campaign *model.Campaign `json:"campaign"`
}

// PublicFormCampaign is the 29-attribute projection for public forms.
type PublicFormCampaign struct {
	ID                  string          `json:"id"`
	Title               string          `json:"title"`
	Description         *string         `json:"description"`
	EventDate           *time.Time      `json:"event_date"`
	EventTime           *string         `json:"event_time"`
	Weekdays            json.RawMessage `json:"weekdays"`
	EventDates          json.RawMessage `json:"event_dates"`
	Location            *string         `json:"location"`
	Objectives          *string         `json:"objectives"`
	AddressCity         *string         `json:"address_city"`
	AddressState        *string         `json:"address_state"`
	Address             *string         `json:"address"`
	AddressNumber       *string         `json:"address_number"`
	AddressNeighborhood *string         `json:"address_neighborhood"`
	LPTemplate          *string         `json:"lp_template"`
	Turmas              json.RawMessage `json:"turmas"`
	SnackPrice          *string         `json:"snack_price"`
	Products            json.RawMessage `json:"products"`
	MapsURL             *string         `json:"maps_url"`
	GoogleMapsLink      *string         `json:"google_maps_link"`
	Latitude            *string         `json:"latitude"`
	Longitude           *string         `json:"longitude"`
	AddressZipcode      *string         `json:"address_zipcode"`
	Platform            string          `json:"platform"`
	AutoRelationship    bool            `json:"auto_relationship"`
	ResponsibleName     *string         `json:"responsible_name"`
	ResponsibleWhatsapp *string         `json:"responsible_whatsapp"`
	TemplateConfig      json.RawMessage `json:"template_config"`
	FormCTAText         *string         `json:"form_cta_text"`
}

// FormWithPublicCampaign mirrors getByToken/getBySlug.
type FormWithPublicCampaign struct {
	model.Form
	Campaign *PublicFormCampaign `json:"campaign"`
}

func toPublicCampaign(c *model.Campaign) *PublicFormCampaign {
	if c == nil {
		return nil
	}
	p := &PublicFormCampaign{
		ID: c.ID, Title: c.Title, Description: c.Description,
		EventTime: c.EventTime, Weekdays: c.Weekdays, EventDates: c.EventDates,
		Location: c.Location, Objectives: c.Objectives, AddressCity: c.AddressCity,
		AddressState: c.AddressState, Address: c.Address, AddressNumber: c.AddressNumber,
		AddressNeighborhood: c.AddressNeighborhood, LPTemplate: c.LPTemplate, Turmas: c.Turmas,
		SnackPrice: c.SnackPrice, Products: c.Products, MapsURL: c.MapsURL,
		GoogleMapsLink: c.GoogleMapsLink, Latitude: c.Latitude, Longitude: c.Longitude,
		AddressZipcode: c.AddressZipcode, Platform: c.Platform,
		AutoRelationship: c.AutoRelationship, ResponsibleName: c.ResponsibleName,
		ResponsibleWhatsapp: c.ResponsibleWhatsapp, TemplateConfig: c.TemplateConfig,
		FormCTAText: c.FormCTAText,
	}
	p.EventDate = c.EventDate
	return p
}

// FormHandler mirrors FormController.
type FormHandler struct {
	cfg       *config.Config
	forms     *repository.FormRepository
	campaigns *repository.CampaignRepository
}

func NewFormHandler(cfg *config.Config, forms *repository.FormRepository, campaigns *repository.CampaignRepository) *FormHandler {
	return &FormHandler{cfg: cfg, forms: forms, campaigns: campaigns}
}

func (h *FormHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	var campaignIDs []string
	scoped := false
	if cid := q.Get("campaign_id"); cid != "" {
		campaignIDs = []string{cid}
		scoped = true
	}
	if authctx.UserRole(r) == model.RoleUser {
		ids, err := h.campaigns.IDsByUser(r.Context(), authctx.UserID(r))
		if err != nil {
			WriteAppError(w, err, "Erro ao listar formulários")
			return
		}
		campaignIDs = ids
		scoped = true
	}
	forms, total, err := h.forms.List(r.Context(), campaignIDs, scoped, page, limit)
	if err != nil {
		WriteAppError(w, err, "Erro ao listar formulários")
		return
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	rows := make([]any, 0, len(forms))
	for i := range forms {
		f := forms[i]
		var mini *FormCampaignMini
		if c, err := h.campaigns.ByID(r.Context(), f.CampaignID); err == nil {
			mini = &FormCampaignMini{ID: c.ID, Title: c.Title, Status: c.Status}
		}
		rows = append(rows, FormWithCampaignMini{Form: f, Campaign: mini})
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"data": rows,
		"pagination": map[string]any{
			"page": page, "limit": limit, "total": total,
			"totalPages": (total + limit - 1) / limit,
		},
	})
}

func (h *FormHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	f, err := h.forms.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Formulário não encontrado")
		return
	}
	if authctx.UserRole(r) == model.RoleUser {
		c, err := h.campaigns.ByID(r.Context(), f.CampaignID)
		if err != nil || c.UserID != authctx.UserID(r) {
			WriteError(w, http.StatusForbidden, "Acesso negado")
			return
		}
	}
	c, err := h.campaigns.ByID(r.Context(), f.CampaignID)
	if err != nil {
		c = nil
	}
	WriteJSON(w, http.StatusOK, FormWithCampaign{Form: *f, Campaign: c})
}

func (h *FormHandler) publicForm(r *http.Request, f *model.Form) (*FormWithPublicCampaign, *service.AppError) {
	c, err := h.campaigns.ByID(r.Context(), f.CampaignID)
	if err != nil {
		c = nil
	}
	if !f.IsActive && (c == nil || c.Status != "completed") {
		return nil, service.Forbidden("Formulário não está ativo")
	}
	return &FormWithPublicCampaign{Form: *f, Campaign: toPublicCampaign(c)}, nil
}

func (h *FormHandler) GetByToken(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	f, err := h.forms.ByPublicToken(r.Context(), token)
	if err != nil {
		if err != pgx.ErrNoRows {
			WriteAppError(w, err, "Erro ao buscar formulário público")
			return
		}
		f, err = h.forms.ByShortCode(r.Context(), token)
		if err != nil {
			WriteError(w, http.StatusNotFound, "Formulário não encontrado")
			return
		}
	}
	view, appErr := h.publicForm(r, f)
	if appErr != nil {
		WriteAppError(w, appErr, "Erro ao buscar formulário público")
		return
	}
	WriteJSON(w, http.StatusOK, view)
}

func (h *FormHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	f, err := h.forms.BySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Formulário não encontrado")
		return
	}
	view, appErr := h.publicForm(r, f)
	if appErr != nil {
		WriteAppError(w, appErr, "Erro ao buscar formulário público")
		return
	}
	WriteJSON(w, http.StatusOK, view)
}

var shortCodeRe = regexp.MustCompile(`^[a-z0-9_-]+$`)

func (h *FormHandler) SetShortCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ShortCode string `json:"short_code"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.ShortCode == "" || !shortCodeRe.MatchString(lowerASCII(body.ShortCode)) {
		WriteError(w, http.StatusBadRequest, "Código inválido. Use apenas letras, números, hífen e underline.")
		return
	}
	id := r.PathValue("id")
	taken, err := h.forms.ShortCodeTaken(r.Context(), body.ShortCode, id)
	if err != nil {
		WriteAppError(w, err, "Erro ao definir short code")
		return
	}
	if taken {
		WriteError(w, http.StatusConflict, "Este código curto já está em uso. Escolha outro.")
		return
	}
	f, err := h.forms.ByID(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Formulário não encontrado")
		return
	}
	if err := h.forms.UpdateFields(r.Context(), id, map[string]any{"short_code": body.ShortCode}); err != nil {
		WriteAppError(w, err, "Erro ao definir short code")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{
		"short_code": body.ShortCode, "public_token": f.PublicToken, "message": "Link curto salvo!",
	})
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func (h *FormHandler) RemoveShortCode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.forms.ByID(r.Context(), id); err != nil {
		WriteError(w, http.StatusNotFound, "Formulário não encontrado")
		return
	}
	if err := h.forms.UpdateFields(r.Context(), id, map[string]any{"short_code": nil}); err != nil {
		WriteAppError(w, err, "Erro ao remover short code")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "Link curto removido"})
}

func (h *FormHandler) RedirectShortCode(w http.ResponseWriter, r *http.Request) {
	f, err := h.forms.ByShortCode(r.Context(), r.PathValue("shortCode"))
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Link não encontrado"))
		return
	}
	base := h.cfg.FrontendURL
	if base == "" {
		base = "https://leads.gnosisbrasil.com"
	}
	http.Redirect(w, r, base+"/form/"+f.PublicToken, http.StatusFound)
}

func (h *FormHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CampaignID   string          `json:"campaign_id"`
		Title        string          `json:"title"`
		Description  *string         `json:"description"`
		CustomFields json.RawMessage `json:"custom_fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	if body.Title == "" || body.CampaignID == "" {
		WriteError(w, http.StatusBadRequest, "Título e campanha são obrigatórios")
		return
	}
	if authctx.UserRole(r) == model.RoleUser {
		c, err := h.campaigns.ByID(r.Context(), body.CampaignID)
		if err != nil || c.UserID != authctx.UserID(r) {
			WriteError(w, http.StatusForbidden, "Acesso negado")
			return
		}
	}
	slug := service.SlugifyForm(body.Title)
	if taken, err := h.forms.SlugTaken(r.Context(), slug); err != nil {
		WriteAppError(w, err, "Erro ao criar formulário")
		return
	} else if taken {
		slug = slug + "-" + strconv.FormatInt(now().UnixMilli(), 10)
	}
	now := now()
	f := &model.Form{
		ID: uuid.NewString(), CampaignID: body.CampaignID, Slug: slug, Title: body.Title,
		Description: body.Description, IsActive: true, CreatedAt: now, UpdatedAt: now,
		PublicToken: uuid.NewString(),
	}
	if len(body.CustomFields) > 0 {
		f.CustomFields = body.CustomFields
	} else {
		f.CustomFields = []byte("[]")
	}
	if err := h.forms.Create(r.Context(), f); err != nil {
		WriteAppError(w, err, "Erro ao criar formulário")
		return
	}
	WriteJSON(w, http.StatusCreated, f)
}

func (h *FormHandler) Update(w http.ResponseWriter, r *http.Request) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	id := r.PathValue("id")
	f, err := h.forms.ByID(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Formulário não encontrado")
		return
	}
	if authctx.UserRole(r) == model.RoleUser {
		c, err := h.campaigns.ByID(r.Context(), f.CampaignID)
		if err != nil || c.UserID != authctx.UserID(r) {
			WriteError(w, http.StatusForbidden, "Acesso negado")
			return
		}
	}
	fields := map[string]any{}
	if s, ok := raw["title"].(string); ok && s != "" {
		fields["title"] = s
	}
	if v, present := raw["description"]; present {
		fields["description"] = v
	}
	if v, present := raw["custom_fields"]; present && v != nil {
		if b, err := json.Marshal(v); err == nil {
			fields["custom_fields"] = b
		}
	}
	if v, present := raw["is_active"]; present {
		fields["is_active"] = v
	}
	if len(fields) > 0 {
		if err := h.forms.UpdateFields(r.Context(), id, fields); err != nil {
			WriteAppError(w, err, "Erro ao atualizar formulário")
			return
		}
	}
	fresh, err := h.forms.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, err, "Erro ao atualizar formulário")
		return
	}
	WriteJSON(w, http.StatusOK, fresh)
}

func (h *FormHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, err := h.forms.ByID(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Formulário não encontrado")
		return
	}
	if authctx.UserRole(r) == model.RoleUser {
		c, err := h.campaigns.ByID(r.Context(), f.CampaignID)
		if err != nil || c.UserID != authctx.UserID(r) {
			WriteError(w, http.StatusForbidden, "Acesso negado")
			return
		}
	}
	if err := h.forms.Delete(r.Context(), id); err != nil {
		WriteAppError(w, err, "Erro ao deletar formulário")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "Formulário removido com sucesso"})
}
