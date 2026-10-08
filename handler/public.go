package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"igb-leads-go/config"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// PublicLeadView mirrors the public lead row.
type PublicLeadView struct {
	ID                  string             `json:"id"`
	FirstName           string             `json:"first_name"`
	LastName            string             `json:"last_name"`
	Whatsapp            string             `json:"whatsapp"`
	Email               string             `json:"email"`
	Status              string             `json:"status"`
	CreatedAt           any                `json:"created_at"`
	CheckinCode         string             `json:"checkin_code"`
	ConfirmationSentAt  any                `json:"confirmation_sent_at"`
	ConfirmedAt         any                `json:"confirmed_at"`
	VoucherSentAt       any                `json:"voucher_sent_at"`
	AttendedAt          any                `json:"attended_at"`
	CancelledAt         any                `json:"cancelled_at"`
	ReminderSentAt      any                `json:"reminder_sent_at"`
	MotivationSentAt    any                `json:"motivation_sent_at"`
	ReminderEventSentAt any                `json:"reminder_event_sent_at"`
	Metadata            json.RawMessage    `json:"metadata"`
	WhatsappMasked      string             `json:"whatsapp_masked"`
	FunnelStatus        service.FunnelStep `json:"funnel_status"`
}

// PublicHandler mirrors PublicCampaignController.
type PublicHandler struct {
	cfg       *config.Config
	campaigns *repository.CampaignRepository
	forms     *repository.FormRepository
	leads     *repository.LeadRepository
	templates *repository.TemplateRepository
	whatsapp  *service.WhatsAppClient
}

func NewPublicHandler(cfg *config.Config, campaigns *repository.CampaignRepository, forms *repository.FormRepository, leads *repository.LeadRepository, templates *repository.TemplateRepository, whatsapp *service.WhatsAppClient) *PublicHandler {
	return &PublicHandler{cfg: cfg, campaigns: campaigns, forms: forms, leads: leads, templates: templates, whatsapp: whatsapp}
}

type tokenResult struct {
	campaign         *model.Campaign
	err              string
	status           int
	requiresPassword bool
}

// findCampaignByToken mirrors the helper (including the 200-status quirk).
func (h *PublicHandler) findCampaignByToken(r *http.Request, token, password string) *tokenResult {
	c, err := h.campaigns.ByPublicToken(r.Context(), token)
	if err != nil {
		return &tokenResult{err: "Link inválido ou expirado", status: http.StatusNotFound}
	}
	if c.PublicLinkPassword != nil && *c.PublicLinkPassword != "" {
		if password == "" {
			return &tokenResult{err: "Senha obrigatória", status: http.StatusOK, requiresPassword: true, campaign: c}
		}
		if !service.CheckPassword(password, *c.PublicLinkPassword) {
			return &tokenResult{err: "Senha incorreta", status: http.StatusUnauthorized}
		}
	}
	return &tokenResult{campaign: c}
}

func (h *PublicHandler) leadInCampaign(r *http.Request, leadID, campaignID string) (*model.Lead, error) {
	formIDs, err := h.forms.IDsByCampaignIDs(r.Context(), []string{campaignID})
	if err != nil {
		return nil, err
	}
	lead, err := h.leads.ByID(r.Context(), leadID)
	if err != nil {
		return nil, err
	}
	if lead.FormID == nil {
		return nil, errNotInCampaign
	}
	for _, id := range formIDs {
		if id == *lead.FormID {
			return lead, nil
		}
	}
	return nil, errNotInCampaign
}

var errNotInCampaign = service.NotFound("Lead não encontrado nesta campanha")

func (h *PublicHandler) GenerateLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := r.PathValue("id")
	if _, err := h.campaigns.ByID(r.Context(), id); err != nil {
		WriteError(w, http.StatusNotFound, "Campanha não encontrada")
		return
	}
	token := strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	var hashPtr *string
	if body.Password != "" {
		hash, err := service.HashPassword(body.Password, 10)
		if err != nil {
			WriteAppError(w, err, "Erro ao gerar link público")
			return
		}
		hashPtr = &hash
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{
		"public_link_token": token, "public_link_password": hashPtr, "public_link_active": true,
	}); err != nil {
		WriteAppError(w, err, "Erro ao gerar link público")
		return
	}
	base := h.cfg.FrontendURL
	if base == "" {
		base = "http://localhost:5173"
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Link público gerado com sucesso", "token": token,
		"url":          base + "/leads-view/" + token,
		"has_password": body.Password != "", "has_public_link": true,
	})
}

func (h *PublicHandler) RevokeLink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.campaigns.ByID(r.Context(), id); err != nil {
		WriteError(w, http.StatusNotFound, "Campanha não encontrada")
		return
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{
		"public_link_token": nil, "public_link_password": nil, "public_link_active": false,
	}); err != nil {
		WriteAppError(w, err, "Erro ao revogar link público")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "Link público revogado com sucesso"})
}

// GetStatus mirrors getStatus, including reading the password flag
// (which the Node attribute projection omits, always yielding false).
func (h *PublicHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	c, err := h.campaigns.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Campanha não encontrada")
		return
	}
	var publicURL *string
	if c.PublicLinkToken != nil {
		base := h.cfg.FrontendURL
		if base == "" {
			base = "http://localhost:5173"
		}
		u := base + "/leads-view/" + *c.PublicLinkToken
		publicURL = &u
	}
	hasLink := c.PublicLinkToken != nil && c.PublicLinkActive
	WriteJSON(w, http.StatusOK, map[string]any{
		"has_public_link": hasLink, "url": publicURL,
		"has_password": c.PublicLinkPassword != nil && *c.PublicLinkPassword != "",
	})
}

func toPublicLead(l *model.Lead) PublicLeadView {
	return PublicLeadView{
		ID: l.ID, FirstName: l.FirstName, LastName: l.LastName, Whatsapp: l.Whatsapp,
		Email: l.Email, Status: l.Status, CreatedAt: l.CreatedAt, CheckinCode: l.CheckinCode,
		ConfirmationSentAt: l.ConfirmationSentAt, ConfirmedAt: l.ConfirmedAt,
		VoucherSentAt: l.VoucherSentAt, AttendedAt: l.AttendedAt, CancelledAt: l.CancelledAt,
		ReminderSentAt: l.ReminderSentAt, MotivationSentAt: l.MotivationSentAt,
		ReminderEventSentAt: l.ReminderEventSentAt, Metadata: l.Metadata,
		WhatsappMasked: service.MaskWhatsApp(l.Whatsapp),
		FunnelStatus:   service.FunnelStatusOf(l),
	}
}

func (h *PublicHandler) GetLeads(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res := h.findCampaignByToken(r, r.PathValue("token"), body.Password)
	if res.err != "" {
		if res.requiresPassword {
			WriteJSON(w, res.status, map[string]any{
				"error": res.err, "status": res.status,
				"requires_password": true, "campaign": map[string]string{"title": res.campaign.Title},
			})
			return
		}
		WriteError(w, res.status, res.err)
		return
	}
	c := res.campaign
	forms, err := h.forms.ByCampaignIDs(r.Context(), []string{c.ID})
	if err != nil {
		WriteAppError(w, err, "Erro ao buscar leads públicos")
		return
	}
	formIDs := make([]string, 0, len(forms))
	for i := range forms {
		formIDs = append(formIDs, forms[i].ID)
	}
	leads, err := h.leads.ByFormIDs(r.Context(), formIDs)
	if err != nil {
		WriteAppError(w, err, "Erro ao buscar leads públicos")
		return
	}
	views := make([]any, 0, len(leads))
	for i := range leads {
		views = append(views, toPublicLead(&leads[i]))
	}
	tpls, err := h.templates.ActiveByCampaign(r.Context(), c.ID)
	if err != nil {
		WriteAppError(w, err, "Erro ao buscar leads públicos")
		return
	}
	minis := make([]any, 0, len(tpls))
	for i := range tpls {
		minis = append(minis, repository.ToTemplateMini(&tpls[i]))
	}
	count := func(f func(*model.Lead) bool) int {
		n := 0
		for i := range leads {
			if f(&leads[i]) {
				n++
			}
		}
		return n
	}
	stats := map[string]int{
		"total": len(leads),
		"new":   count(func(l *model.Lead) bool { return l.ConfirmationSentAt == nil && l.CancelledAt == nil }),
		"contacted": count(func(l *model.Lead) bool {
			return l.ConfirmationSentAt != nil && l.ConfirmedAt == nil && l.CancelledAt == nil
		}),
		"confirmed": count(func(l *model.Lead) bool {
			return l.ConfirmedAt != nil && l.VoucherSentAt == nil && l.CancelledAt == nil
		}),
		"voucher_sent": count(func(l *model.Lead) bool { return l.VoucherSentAt != nil && l.AttendedAt == nil && l.CancelledAt == nil }),
		"attended":     count(func(l *model.Lead) bool { return l.AttendedAt != nil }),
		"cancelled":    count(func(l *model.Lead) bool { return l.CancelledAt != nil }),
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"requires_password": false,
		"campaign": map[string]any{
			"id": c.ID, "title": c.Title, "description": c.Description, "status": c.Status,
			"event_date": c.EventDate, "event_time": c.EventTime, "location": c.Location,
			"objectives": c.Objectives, "start_date": c.StartDate, "end_date": c.EndDate,
			"weekdays": c.Weekdays, "event_dates": c.EventDates,
			"address_city": c.AddressCity, "address": c.Address,
			"maps_url": c.MapsURL, "google_maps_link": c.GoogleMapsLink,
		},
		"stats": stats, "leads": views, "templates": minis,
	})
}

var publicActions = map[string]struct {
	Column string
	Status string
	Revert bool
}{
	"mark_confirmation_sent":   {Column: "confirmation_sent_at"},
	"mark_confirmed":           {Column: "confirmed_at", Status: "confirmed"},
	"mark_voucher_sent":        {Column: "voucher_sent_at"},
	"mark_attended":            {Column: "attended_at", Status: "present"},
	"mark_cancelled":           {Column: "cancelled_at", Status: "lost"},
	"mark_reminder_event_sent": {Column: "reminder_event_sent_at"},
	"revert":                   {Revert: true},
}

func (h *PublicHandler) LeadAction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res := h.findCampaignByToken(r, r.PathValue("token"), body.Password)
	if res.err != "" {
		WriteError(w, res.status, res.err)
		return
	}
	lead, err := h.leadInCampaign(r, r.PathValue("leadId"), res.campaign.ID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Lead não encontrado nesta campanha")
		return
	}
	action, ok := publicActions[r.PathValue("action")]
	if !ok {
		WriteError(w, http.StatusBadRequest, "Ação inválida")
		return
	}
	if action.Revert {
		rule, ok := service.RevertMap[service.FunnelStatusOf(lead).Step]
		if !ok {
			WriteError(w, http.StatusBadRequest, "Não é possível reverter deste estágio")
			return
		}
		update := map[string]any{"status": rule.Status}
		for _, col := range rule.Clear {
			update[col] = nil
		}
		if err := h.leads.UpdateFields(r.Context(), lead.ID, update); err != nil {
			WriteAppError(w, err, "Erro na ação pública do lead")
			return
		}
	} else {
		update := map[string]any{action.Column: now()}
		if action.Status != "" {
			update["status"] = action.Status
		}
		if err := h.leads.UpdateFields(r.Context(), lead.ID, update); err != nil {
			WriteAppError(w, err, "Erro na ação pública do lead")
			return
		}
	}
	fresh, err := h.leads.ByID(r.Context(), lead.ID)
	if err != nil {
		WriteAppError(w, err, "Erro na ação pública do lead")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Ação realizada com sucesso",
		"lead":    toPublicLeadFull(fresh),
	})
}

// toPublicLeadFull mirrors the action response (full lead + extras).
func toPublicLeadFull(l *model.Lead) map[string]any {
	m := map[string]any{}
	b, _ := json.Marshal(l)
	_ = json.Unmarshal(b, &m)
	m["whatsapp_masked"] = service.MaskWhatsApp(l.Whatsapp)
	m["funnel_status"] = service.FunnelStatusOf(l)
	return m
}

func (h *PublicHandler) Checkin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res := h.findCampaignByToken(r, r.PathValue("token"), body.Password)
	if res.err != "" {
		WriteError(w, res.status, res.err)
		return
	}
	lead, err := h.leads.ByCheckinCode(r.Context(), body.Code)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Código de check-in não encontrado nesta campanha")
		return
	}
	formIDs, err := h.forms.IDsByCampaignIDs(r.Context(), []string{res.campaign.ID})
	if err != nil {
		WriteAppError(w, err, "Erro no check-in público")
		return
	}
	belongs := false
	if lead.FormID != nil {
		for _, id := range formIDs {
			if id == *lead.FormID {
				belongs = true
				break
			}
		}
	}
	if !belongs {
		WriteError(w, http.StatusNotFound, "Código de check-in não encontrado nesta campanha")
		return
	}
	if lead.CheckinAt != nil {
		WriteJSON(w, http.StatusOK, map[string]any{
			"success": true, "alreadyCheckedIn": true,
			"lead": map[string]any{
				"id": lead.ID, "first_name": lead.FirstName,
				"last_name": lead.LastName, "checkin_at": lead.CheckinAt,
			},
		})
		return
	}
	now := now()
	if err := h.leads.UpdateFields(r.Context(), lead.ID, map[string]any{
		"status": "present", "checkin_at": now, "attended_at": now,
	}); err != nil {
		WriteAppError(w, err, "Erro no check-in público")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"success": true, "alreadyCheckedIn": false,
		"lead": map[string]any{
			"id": lead.ID, "first_name": lead.FirstName,
			"last_name": lead.LastName, "checkin_at": now,
		},
	})
}

func (h *PublicHandler) WhatsAppLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res := h.findCampaignByToken(r, r.PathValue("token"), body.Password)
	if res.err != "" {
		WriteError(w, res.status, res.err)
		return
	}
	lead, err := h.leadInCampaign(r, r.PathValue("leadId"), res.campaign.ID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Lead não encontrado")
		return
	}
	tpl, err := h.templates.ActiveByIDAndCampaign(r.Context(), r.PathValue("templateId"), res.campaign.ID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Template não encontrado")
		return
	}
	phone := "55" + service.DigitsOnly(lead.Whatsapp)
	message := service.SubstitutePatterns(tpl.Content, lead, res.campaign, tpl.Key, h.cfg.FrontendURL, h.cfg.APIURL)
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
		"phone": phone, "message": message, "whatsapp_url": waURL, "sent_via_api": sent,
	})
}

func (h *PublicHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res := h.findCampaignByToken(r, r.PathValue("token"), body.Password)
	if res.err != "" {
		WriteError(w, res.status, res.err)
		return
	}
	tpls, err := h.templates.ByCampaign(r.Context(), res.campaign.ID)
	if err != nil {
		WriteAppError(w, err, "Erro ao listar templates públicos")
		return
	}
	if tpls == nil {
		tpls = []model.MessageTemplate{}
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": tpls})
}

func (h *PublicHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password  string `json:"password"`
		Key       string `json:"key"`
		Label     string `json:"label"`
		Content   string `json:"content"`
		SortOrder int    `json:"sort_order"`
		Phase     string `json:"phase"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res := h.findCampaignByToken(r, r.PathValue("token"), body.Password)
	if res.err != "" {
		WriteError(w, res.status, res.err)
		return
	}
	if body.Key == "" || body.Label == "" || body.Content == "" {
		WriteError(w, http.StatusBadRequest, "Chave, rótulo e conteúdo são obrigatórios")
		return
	}
	now := now()
	phase := body.Phase
	if phase == "" {
		phase = "general"
	}
	tpl := &model.MessageTemplate{
		ID: uuid.NewString(), CampaignID: res.campaign.ID, Key: body.Key,
		Label: body.Label, Content: body.Content, SortOrder: body.SortOrder,
		IsActive: true, CreatedAt: now, UpdatedAt: now, Phase: phase, IsEditable: true,
	}
	if err := h.templates.Create(r.Context(), tpl); err != nil {
		WriteAppError(w, err, "Erro ao criar template público")
		return
	}
	WriteJSON(w, http.StatusCreated, tpl)
}

func (h *PublicHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	var raw map[string]any
	_ = json.NewDecoder(r.Body).Decode(&raw)
	password, _ := raw["password"].(string)
	res := h.findCampaignByToken(r, r.PathValue("token"), password)
	if res.err != "" {
		WriteError(w, res.status, res.err)
		return
	}
	tpl, err := h.templates.ByIDAndCampaign(r.Context(), r.PathValue("templateId"), res.campaign.ID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Template não encontrado")
		return
	}
	if !tpl.IsEditable {
		WriteError(w, http.StatusForbidden, "Templates padrão não podem ser editados")
		return
	}
	fields := map[string]any{}
	for _, k := range []string{"label", "content", "is_active", "sort_order", "phase"} {
		if v, present := raw[k]; present {
			fields[k] = v
		}
	}
	if len(fields) > 0 {
		if err := h.templates.UpdateFields(r.Context(), tpl.ID, fields); err != nil {
			WriteAppError(w, err, "Erro ao atualizar template público")
			return
		}
	}
	fresh, err := h.templates.ByID(r.Context(), tpl.ID)
	if err != nil {
		WriteAppError(w, err, "Erro ao atualizar template público")
		return
	}
	WriteJSON(w, http.StatusOK, fresh)
}

func (h *PublicHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res := h.findCampaignByToken(r, r.PathValue("token"), body.Password)
	if res.err != "" {
		WriteError(w, res.status, res.err)
		return
	}
	tpl, err := h.templates.ByIDAndCampaign(r.Context(), r.PathValue("templateId"), res.campaign.ID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Template não encontrado")
		return
	}
	if !tpl.IsEditable {
		WriteError(w, http.StatusForbidden, "Templates padrão não podem ser removidos")
		return
	}
	if err := h.templates.Delete(r.Context(), tpl.ID); err != nil {
		WriteAppError(w, err, "Erro ao remover template público")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "Template removido"})
}

func (h *PublicHandler) SeedTemplates(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res := h.findCampaignByToken(r, r.PathValue("token"), body.Password)
	if res.err != "" {
		WriteError(w, res.status, res.err)
		return
	}
	objectives := ""
	if res.campaign.Objectives != nil {
		objectives = *res.campaign.Objectives
	}
	if objectives == "" {
		objectives = "camara_publica"
	}
	created := []model.MessageTemplate{}
	for _, t := range service.DefaultsFor(objectives) {
		if _, err := h.templates.ByCampaignAndKey(r.Context(), res.campaign.ID, t.Key); err == nil {
			continue
		}
		now := now()
		tpl := &model.MessageTemplate{
			ID: uuid.NewString(), CampaignID: res.campaign.ID, Key: t.Key,
			Label: t.Label, Content: t.Content, SortOrder: t.SortOrder,
			IsActive: true, CreatedAt: now, UpdatedAt: now,
			Phase: t.Phase, IsEditable: false,
		}
		if tpl.Phase == "" {
			tpl.Phase = "general"
		}
		if err := h.templates.Create(r.Context(), tpl); err != nil {
			WriteAppError(w, err, "Erro ao criar templates padrão públicos")
			return
		}
		created = append(created, *tpl)
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": strconv.Itoa(len(created)) + " templates criados",
		"data":    created,
	})
}
