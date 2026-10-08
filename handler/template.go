package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"igb-leads-go/authctx"
	"igb-leads-go/config"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// TemplateHandler mirrors MessageTemplateController.
type TemplateHandler struct {
	cfg       *config.Config
	templates *repository.TemplateRepository
	leads     *repository.LeadRepository
	forms     *repository.FormRepository
	campaigns *repository.CampaignRepository
	whatsapp  *service.WhatsAppClient
}

func NewTemplateHandler(cfg *config.Config, templates *repository.TemplateRepository, leads *repository.LeadRepository, forms *repository.FormRepository, campaigns *repository.CampaignRepository, whatsapp *service.WhatsAppClient) *TemplateHandler {
	return &TemplateHandler{cfg: cfg, templates: templates, leads: leads, forms: forms, campaigns: campaigns, whatsapp: whatsapp}
}

// scopedCampaign loads the campaign and enforces team scope: admins see
// all, supervisors their region, others their own/traffic campaigns.
func (h *TemplateHandler) scopedCampaign(r *http.Request, campaignID string) (*model.Campaign, bool) {
	user := authctx.CurrentUser(r)
	if user == nil {
		return nil, false
	}
	campaign, err := h.campaigns.ByID(r.Context(), campaignID)
	if err != nil {
		return nil, false
	}
	switch user.Role {
	case model.RoleAdmin:
		return campaign, true
	case model.RoleSupervisor:
		if user.RegionID != nil && campaign.RegionID != nil && *user.RegionID == *campaign.RegionID {
			return campaign, true
		}
	case model.RoleExecutive:
		if campaign.TrafficManagerID != nil && *campaign.TrafficManagerID == user.ID {
			return campaign, true
		}
	case model.RoleUser:
		if campaign.UserID == user.ID {
			return campaign, true
		}
	}
	return campaign, false
}

func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	campaignID := r.URL.Query().Get("campaign_id")
	if campaignID == "" {
		WriteError(w, http.StatusBadRequest, "campaign_id é obrigatório")
		return
	}
	if _, ok := h.scopedCampaign(r, campaignID); !ok {
		WriteError(w, http.StatusForbidden, "Sem permissão para esta campanha")
		return
	}
	tpls, err := h.templates.ByCampaign(r.Context(), campaignID)
	if err != nil {
		log.Printf("Erro ao listar templates: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	if tpls == nil {
		tpls = []model.MessageTemplate{}
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": tpls})
}

func (h *TemplateHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	tpl, err := h.templates.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Template não encontrado")
		return
	}
	WriteJSON(w, http.StatusOK, tpl)
}

func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CampaignID string `json:"campaign_id"`
		Key        string `json:"key"`
		Label      string `json:"label"`
		Content    string `json:"content"`
		SortOrder  int    `json:"sort_order"`
		Phase      string `json:"phase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	if _, ok := h.scopedCampaign(r, body.CampaignID); !ok {
		WriteError(w, http.StatusForbidden, "Sem permissão para esta campanha")
		return
	}
	if body.Phase == "" {
		body.Phase = "general"
	}
	now := time.Now().UTC()
	tpl := &model.MessageTemplate{
		ID: uuid.NewString(), CampaignID: body.CampaignID, Key: body.Key,
		Label: body.Label, Content: body.Content, SortOrder: body.SortOrder,
		IsActive: true, CreatedAt: now, UpdatedAt: now,
		Phase: body.Phase, IsEditable: true,
	}
	if err := h.templates.Create(r.Context(), tpl); err != nil {
		log.Printf("Erro ao criar template: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusCreated, tpl)
}

func (h *TemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	tpl, err := h.templates.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Template não encontrado")
		return
	}
	if _, ok := h.scopedCampaign(r, tpl.CampaignID); !ok {
		WriteError(w, http.StatusForbidden, "Sem permissão para esta campanha")
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	fields := map[string]any{}
	for _, col := range []string{"label", "content", "is_active", "sort_order", "phase"} {
		if v, ok := body[col]; ok {
			fields[col] = v
		}
	}
	if err := h.templates.UpdateFields(r.Context(), tpl.ID, fields); err != nil {
		log.Printf("Erro ao atualizar template: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	updated, err := h.templates.ByID(r.Context(), tpl.ID)
	if err != nil {
		log.Printf("Erro ao atualizar template: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

func (h *TemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	tpl, err := h.templates.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Template não encontrado")
		return
	}
	if !tpl.IsEditable {
		WriteError(w, http.StatusForbidden, "Templates padrão não podem ser removidos")
		return
	}
	if _, ok := h.scopedCampaign(r, tpl.CampaignID); !ok {
		WriteError(w, http.StatusForbidden, "Sem permissão para esta campanha")
		return
	}
	if err := h.templates.Delete(r.Context(), tpl.ID); err != nil {
		log.Printf("Erro ao remover template: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Template removido"})
}

// leadCampaign resolves the lead's form and campaign, mirroring the
// nested include (either may be absent for orphaned leads).
func (h *TemplateHandler) leadCampaign(r *http.Request, leadID string) (*model.Lead, *model.Campaign, error) {
	lead, err := h.leads.ByID(r.Context(), leadID)
	if err != nil {
		return nil, nil, err
	}
	if lead.FormID == nil {
		return lead, nil, nil
	}
	form, err := h.forms.ByID(r.Context(), *lead.FormID)
	if err != nil {
		return lead, nil, nil
	}
	campaign, err := h.campaigns.ByID(r.Context(), form.CampaignID)
	if err != nil {
		return lead, nil, nil
	}
	return lead, campaign, nil
}

func (h *TemplateHandler) GenerateLink(w http.ResponseWriter, r *http.Request) {
	lead, campaign, err := h.leadCampaign(r, r.PathValue("lead_id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Lead não encontrado")
		return
	}
	tpl, err := h.templates.ByID(r.Context(), r.PathValue("template_id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Template não encontrado")
		return
	}
	if campaign == nil {
		WriteError(w, http.StatusBadRequest, "Lead não está vinculado a uma campanha")
		return
	}
	message := service.SubstitutePatterns(tpl.Content, lead, campaign, tpl.Key, h.cfg.FrontendURL, h.cfg.APIURL)
	phone := "55" + service.DigitsOnly(lead.Whatsapp)
	sent := false
	if h.whatsapp.IsConfigured() {
		// Meow delivers text only; the QR/checkin URLs already sit
		// inside the substituted message for voucher templates.
		if err := h.whatsapp.SendText(phone, message); err != nil {
			log.Printf("Erro ao enviar via WhatsApp API: %v", err)
		} else {
			sent = true
		}
	}
	waURL := "https://api.whatsapp.com/send?phone=" + phone + "&text=" + service.EncodeURIComponent(message)
	WriteJSON(w, http.StatusOK, map[string]any{
		"template_id": tpl.ID, "template_key": tpl.Key, "template_label": tpl.Label,
		"message": message, "phone": phone, "whatsapp_url": waURL, "sent_via_api": sent,
	})
}

func (h *TemplateHandler) GenerateAllLinks(w http.ResponseWriter, r *http.Request) {
	lead, campaign, err := h.leadCampaign(r, r.PathValue("lead_id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Lead não encontrado")
		return
	}
	campaignID := r.URL.Query().Get("campaign_id")
	if campaignID == "" && campaign != nil {
		campaignID = campaign.ID
	}
	if campaignID == "" {
		WriteError(w, http.StatusBadRequest, "campaign_id é obrigatório")
		return
	}
	if campaign == nil {
		WriteError(w, http.StatusBadRequest, "Lead não está vinculado a uma campanha")
		return
	}
	tpls, err := h.templates.ActiveByCampaign(r.Context(), campaignID)
	if err != nil {
		log.Printf("Erro ao gerar links WhatsApp: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	phone := "55" + service.DigitsOnly(lead.Whatsapp)
	links := []map[string]any{}
	for _, tpl := range tpls {
		message := service.SubstitutePatterns(tpl.Content, lead, campaign, tpl.Key, h.cfg.FrontendURL, h.cfg.APIURL)
		links = append(links, map[string]any{
			"template_id": tpl.ID, "template_key": tpl.Key, "template_label": tpl.Label,
			"message": message, "phone": phone,
			"whatsapp_url": "https://api.whatsapp.com/send?phone=" + phone + "&text=" + service.EncodeURIComponent(message),
		})
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": links})
}

func (h *TemplateHandler) SeedDefaults(w http.ResponseWriter, r *http.Request) {
	campaign, err := h.campaigns.ByID(r.Context(), r.PathValue("campaign_id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Campanha não encontrada")
		return
	}
	if _, ok := h.scopedCampaign(r, campaign.ID); !ok {
		WriteError(w, http.StatusForbidden, "Sem permissão para esta campanha")
		return
	}
	objectives := ""
	if campaign.Objectives != nil {
		objectives = *campaign.Objectives
	}
	if objectives == "" {
		objectives = "camara_publica"
	}
	created := []model.MessageTemplate{}
	for _, t := range service.DefaultsFor(objectives) {
		if _, err := h.templates.ByCampaignAndKey(r.Context(), campaign.ID, t.Key); err == nil {
			continue
		}
		now := time.Now().UTC()
		tpl := &model.MessageTemplate{
			ID: uuid.NewString(), CampaignID: campaign.ID, Key: t.Key,
			Label: t.Label, Content: t.Content, SortOrder: t.SortOrder,
			IsActive: true, CreatedAt: now, UpdatedAt: now,
			Phase: t.Phase, IsEditable: false,
		}
		if tpl.Phase == "" {
			tpl.Phase = "general"
		}
		if err := h.templates.Create(r.Context(), tpl); err != nil {
			log.Printf("Erro ao criar templates padrão: %v", err)
			WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
			return
		}
		created = append(created, *tpl)
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": strconv.Itoa(len(created)) + " templates criados",
		"data":    created,
	})
}
