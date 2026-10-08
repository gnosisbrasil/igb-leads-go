package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"igb-leads-go/authctx"
	"igb-leads-go/config"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// LeadFormMini is the form projection in lead lists.
type LeadFormMini struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Slug       string `json:"slug"`
	CampaignID string `json:"campaign_id"`
}

// LeadWithFormMini mirrors the list row.
type LeadWithFormMini struct {
	model.Lead
	Form *LeadFormMini `json:"form"`
}

// LeadWithFormTree mirrors getById (full form + full campaign).
type LeadWithFormTree struct {
	model.Lead
	Form *FormWithCampaign `json:"form"`
}

// LeadHandler mirrors LeadService + LeadController.
type LeadHandler struct {
	cfg       *config.Config
	leads     *repository.LeadRepository
	forms     *repository.FormRepository
	campaigns *repository.CampaignRepository
	logs      *repository.LogRepository
	auto      *service.AutoRelationship
}

func NewLeadHandler(cfg *config.Config, leads *repository.LeadRepository, forms *repository.FormRepository, campaigns *repository.CampaignRepository, logs *repository.LogRepository, auto *service.AutoRelationship) *LeadHandler {
	return &LeadHandler{cfg: cfg, leads: leads, forms: forms, campaigns: campaigns, logs: logs, auto: auto}
}

func (h *LeadHandler) accessibleFormIDs(r *http.Request, campaignID string) ([]string, error) {
	user := authctx.CurrentUser(r)
	if user.Role == model.RoleAdmin {
		if campaignID == "" {
			return nil, nil // unscoped marker handled by caller
		}
		return h.forms.IDsByCampaignIDs(r.Context(), []string{campaignID})
	}
	campaignIDs, err := h.campaigns.IDsByFilter(r.Context(), user.ID, user.Role, strOrEmptyPtr(user.RegionID), campaignID)
	if err != nil {
		return nil, err
	}
	if len(campaignIDs) == 0 {
		return []string{}, nil
	}
	return h.forms.IDsByCampaignIDs(r.Context(), campaignIDs)
}

func strOrEmptyPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (h *LeadHandler) checkLeadAccess(r *http.Request, lead *model.Lead) error {
	user := authctx.CurrentUser(r)
	if user.Role == model.RoleAdmin {
		return nil
	}
	if lead.FormID == nil {
		return service.Forbidden("Acesso negado a este lead")
	}
	ids, err := h.accessibleFormIDs(r, "")
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == *lead.FormID {
			return nil
		}
	}
	return service.Forbidden("Acesso negado a este lead")
}

func (h *LeadHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	user := authctx.CurrentUser(r)
	campaignID := q.Get("campaign_id")
	formID := q.Get("form_id")
	status := q.Get("status")

	f := repository.LeadFilter{Status: status, Page: page, Limit: limit}
	if user.Role == model.RoleUser || user.Role == model.RoleExecutive || user.Role == model.RoleSupervisor {
		ids, err := h.accessibleFormIDs(r, campaignID)
		if err != nil {
			WriteAppError(w, err, "Erro ao listar leads")
			return
		}
		if len(ids) == 0 {
			rawLimit := 100
			if q.Get("limit") != "" {
				rawLimit = limit
			}
			WriteJSON(w, http.StatusOK, map[string]any{
				"data":       []any{},
				"pagination": map[string]any{"page": 1, "limit": rawLimit, "total": 0, "totalPages": 0},
			})
			return
		}
		f.Scoped = true
		if formID != "" {
			for _, id := range ids {
				if id == formID {
					f.FormIDs = []string{formID}
					break
				}
			}
			if f.FormIDs == nil {
				f.FormIDs = ids
			}
		} else {
			f.FormIDs = ids
		}
	} else {
		if formID != "" {
			f.Scoped = true
			f.FormIDs = []string{formID}
		}
		if campaignID != "" {
			ids, err := h.forms.IDsByCampaignIDs(r.Context(), []string{campaignID})
			if err != nil {
				WriteAppError(w, err, "Erro ao listar leads")
				return
			}
			f.Scoped = true
			f.FormIDs = ids
		}
	}
	leads, total, err := h.leads.List(r.Context(), f)
	if err != nil {
		WriteAppError(w, err, "Erro ao listar leads")
		return
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}
	rows := make([]any, 0, len(leads))
	for i := range leads {
		l := leads[i]
		var mini *LeadFormMini
		if l.FormID != nil {
			if form, err := h.forms.ByID(r.Context(), *l.FormID); err == nil {
				mini = &LeadFormMini{ID: form.ID, Title: form.Title, Slug: form.Slug, CampaignID: form.CampaignID}
			}
		}
		rows = append(rows, LeadWithFormMini{Lead: l, Form: mini})
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"data": rows,
		"pagination": map[string]any{
			"page": page, "limit": limit, "total": total,
			"totalPages": (total + limit - 1) / limit,
		},
	})
}

func (h *LeadHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" || len([]rune(query)) < 2 {
		WriteJSON(w, http.StatusOK, []any{})
		return
	}
	user := authctx.CurrentUser(r)
	var formIDs []string
	scoped := false
	if user.Role != model.RoleAdmin {
		ids, err := h.accessibleFormIDs(r, "")
		if err != nil {
			WriteAppError(w, err, "Erro ao buscar leads")
			return
		}
		if len(ids) == 0 {
			WriteJSON(w, http.StatusOK, []any{})
			return
		}
		formIDs, scoped = ids, true
	}
	leads, err := h.leads.Search(r.Context(), query, formIDs, scoped)
	if err != nil {
		WriteAppError(w, err, "Erro ao buscar leads")
		return
	}
	if leads == nil {
		leads = []model.Lead{}
	}
	WriteJSON(w, http.StatusOK, leads)
}

func (h *LeadHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	lead, err := h.leads.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteAppError(w, service.NotFound("Lead não encontrado"), "Erro ao buscar lead")
		return
	}
	user := authctx.CurrentUser(r)
	if user.Role == model.RoleUser || user.Role == model.RoleExecutive || user.Role == model.RoleSupervisor {
		ids, err := h.accessibleFormIDs(r, "")
		if err != nil {
			WriteAppError(w, err, "Erro ao buscar lead")
			return
		}
		allowed := false
		if lead.FormID != nil {
			for _, id := range ids {
				if id == *lead.FormID {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			WriteAppError(w, service.Forbidden(""), "Erro ao buscar lead")
			return
		}
	}
	var tree *FormWithCampaign
	if lead.FormID != nil {
		if form, err := h.forms.ByID(r.Context(), *lead.FormID); err == nil {
			c, err := h.campaigns.ByID(r.Context(), form.CampaignID)
			if err != nil {
				c = nil
			}
			tree = &FormWithCampaign{Form: *form, Campaign: c}
		}
	}
	WriteJSON(w, http.StatusOK, LeadWithFormTree{Lead: *lead, Form: tree})
}

func newCheckinCode() string {
	return strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:12])
}

// Create handles POST /api/leads/public/:token (public inscription).
func (h *LeadHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FirstName string          `json:"first_name"`
		LastName  string          `json:"last_name"`
		Whatsapp  string          `json:"whatsapp"`
		Email     string          `json:"email"`
		Metadata  json.RawMessage `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	if body.FirstName == "" || body.LastName == "" || body.Whatsapp == "" || body.Email == "" {
		WriteError(w, http.StatusBadRequest, "Nome, sobrenome, whatsapp e email são obrigatórios")
		return
	}
	form, err := h.forms.ByPublicToken(r.Context(), r.PathValue("token"))
	if err != nil {
		WriteAppError(w, service.NotFound("Formulário não encontrado"), "Erro ao criar lead")
		return
	}
	if !form.IsActive {
		WriteAppError(w, service.BadRequest("Formulário não está ativo"), "Erro ao criar lead")
		return
	}
	now := now()
	lead := &model.Lead{
		ID: uuid.NewString(), FormID: &form.ID,
		FirstName: body.FirstName, LastName: body.LastName,
		Whatsapp: service.DigitsOnly(body.Whatsapp), Email: body.Email,
		Status: "new", CheckinCode: newCheckinCode(),
		CreatedAt: now, UpdatedAt: now,
	}
	if len(body.Metadata) > 0 {
		lead.Metadata = body.Metadata
	} else {
		lead.Metadata = []byte("{}")
	}
	if err := h.leads.Create(r.Context(), lead); err != nil {
		WriteAppError(w, err, "Erro ao criar lead")
		return
	}
	var campaignView any
	if c, err := h.campaigns.ByID(r.Context(), form.CampaignID); err == nil {
		if c.AddressCity != nil || c.AddressState != nil {
			_ = h.leads.UpdateFields(r.Context(), lead.ID, map[string]any{"city": c.AddressCity, "state": c.AddressState})
			lead.City, lead.State = c.AddressCity, c.AddressState
		}
		if c.AutoRelationship {
			if err := h.auto.HandleNewLead(r.Context(), lead); err != nil {
				log.Printf("AutoRelationship error: %v", err)
			}
		}
		campaignView = map[string]any{
			"title": c.Title, "event_date": c.EventDate, "event_time": c.EventTime,
			"address": c.Address, "address_city": c.AddressCity, "address_state": c.AddressState,
			"address_number": c.AddressNumber, "address_neighborhood": c.AddressNeighborhood,
		}
	}
	fresh, err := h.leads.ByID(r.Context(), lead.ID)
	if err != nil {
		WriteAppError(w, err, "Erro ao criar lead")
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{"lead": fresh, "campaign": campaignView})
}

func (h *LeadHandler) mark(w http.ResponseWriter, r *http.Request, id, field string) {
	mf, ok := service.MarkFieldMap[field]
	if !ok {
		WriteAppError(w, service.BadRequest("Campo inválido"), "Erro ao marcar "+field)
		return
	}
	lead, err := h.leads.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Lead não encontrado"), "Erro ao marcar "+field)
		return
	}
	if err := h.checkLeadAccess(r, lead); err != nil {
		WriteAppError(w, err, "Erro ao marcar "+field)
		return
	}
	update := map[string]any{mf.Column: now()}
	if mf.Status != "" {
		update["status"] = mf.Status
	}
	if err := h.leads.UpdateFields(r.Context(), id, update); err != nil {
		WriteAppError(w, err, "Erro ao marcar "+field)
		return
	}
	if field == "confirmed" && lead.FormID != nil {
		if form, err := h.forms.ByID(r.Context(), *lead.FormID); err == nil {
			if c, err := h.campaigns.ByID(r.Context(), form.CampaignID); err == nil && c.AutoRelationship {
				if err := h.auto.HandleConfirmed(r.Context(), lead); err != nil {
					log.Printf("AutoRelationship error: %v", err)
				}
			}
		}
	}
	fresh, err := h.leads.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, err, "Erro ao marcar "+field)
		return
	}
	WriteJSON(w, http.StatusOK, fresh)
}

// Mark handles POST /api/leads/:id/mark/:field.
func (h *LeadHandler) Mark(w http.ResponseWriter, r *http.Request) {
	h.mark(w, r, r.PathValue("id"), r.PathValue("field"))
}

// PostDispatch routes POST /api/leads/{id}/{rest...} to the mark, revert
// and QR actions. A single multi-segment pattern avoids the mux conflict
// between /{id}/mark-* and the public /public/{token} route.
func (h *LeadHandler) PostDispatch(w http.ResponseWriter, r *http.Request) {
	id, rest := r.PathValue("id"), r.PathValue("rest")
	switch {
	case rest == "revert":
		h.Revert(w, r)
	case rest == "generate-qr":
		h.GenerateQRCode(w, r)
	case strings.HasPrefix(rest, "mark/"):
		h.mark(w, r, id, strings.TrimPrefix(rest, "mark/"))
	case strings.HasPrefix(rest, "mark-"):
		h.mark(w, r, id, strings.TrimPrefix(rest, "mark-"))
	default:
		WriteError(w, http.StatusNotFound, "Rota não encontrada")
	}
}

func (h *LeadHandler) markNamed(field string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.mark(w, r, r.PathValue("id"), field)
	}
}

func (h *LeadHandler) MarkConfirmationSent(w http.ResponseWriter, r *http.Request) {
	h.markNamed("confirmation-sent")(w, r)
}
func (h *LeadHandler) MarkReminderSent(w http.ResponseWriter, r *http.Request) {
	h.markNamed("reminder-sent")(w, r)
}
func (h *LeadHandler) MarkConfirmed(w http.ResponseWriter, r *http.Request) {
	h.markNamed("confirmed")(w, r)
}
func (h *LeadHandler) MarkVoucherSent(w http.ResponseWriter, r *http.Request) {
	h.markNamed("voucher-sent")(w, r)
}
func (h *LeadHandler) MarkMotivationSent(w http.ResponseWriter, r *http.Request) {
	h.markNamed("motivation-sent")(w, r)
}
func (h *LeadHandler) MarkReminderEventSent(w http.ResponseWriter, r *http.Request) {
	h.markNamed("reminder-event-sent")(w, r)
}
func (h *LeadHandler) MarkAttended(w http.ResponseWriter, r *http.Request) {
	h.markNamed("attended")(w, r)
}
func (h *LeadHandler) MarkCancelled(w http.ResponseWriter, r *http.Request) {
	h.markNamed("cancelled")(w, r)
}
func (h *LeadHandler) MarkQRSent(w http.ResponseWriter, r *http.Request) {
	h.markNamed("qr-sent")(w, r)
}

func (h *LeadHandler) Revert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lead, err := h.leads.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Lead não encontrado"), "Erro ao reverter lead")
		return
	}
	if err := h.checkLeadAccess(r, lead); err != nil {
		WriteAppError(w, err, "Erro ao reverter lead")
		return
	}
	rule, ok := service.RevertMap[service.FunnelStatusOf(lead).Step]
	if !ok {
		WriteAppError(w, service.BadRequest("Não é possível reverter deste estágio"), "Erro ao reverter lead")
		return
	}
	update := map[string]any{"status": rule.Status}
	for _, col := range rule.Clear {
		update[col] = nil
	}
	if err := h.leads.UpdateFields(r.Context(), id, update); err != nil {
		WriteAppError(w, err, "Erro ao reverter lead")
		return
	}
	fresh, err := h.leads.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, err, "Erro ao reverter lead")
		return
	}
	WriteJSON(w, http.StatusOK, fresh)
}

func (h *LeadHandler) GenerateQRCode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lead, err := h.leads.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Lead não encontrado"), "Erro ao gerar QR Code")
		return
	}
	if err := h.checkLeadAccess(r, lead); err != nil {
		WriteAppError(w, err, "Erro ao gerar QR Code")
		return
	}
	if lead.CheckinCode == "" {
		WriteAppError(w, service.BadRequest("Lead sem código de check-in"), "Erro ao gerar QR Code")
		return
	}
	dataURL, err := service.GenerateQRDataURL(lead.CheckinCode)
	if err != nil {
		WriteAppError(w, err, "Erro ao gerar QR Code")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"qrcode": dataURL, "checkin_code": lead.CheckinCode})
}

func (h *LeadHandler) Export(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("campaign_id")
	user := authctx.CurrentUser(r)
	if user.Role != model.RoleAdmin {
		ids, err := h.accessibleFormIDs(r, campaignID)
		if err != nil {
			WriteAppError(w, err, "Erro ao exportar leads")
			return
		}
		if len(ids) == 0 {
			WriteAppError(w, service.Forbidden("Acesso negado a esta campanha"), "Erro ao exportar leads")
			return
		}
	}
	formIDs, err := h.forms.IDsByCampaignIDs(r.Context(), []string{campaignID})
	if err != nil {
		WriteAppError(w, err, "Erro ao exportar leads")
		return
	}
	if len(formIDs) == 0 {
		WriteJSON(w, http.StatusOK, []any{})
		return
	}
	leads, err := h.leads.ByFormIDs(r.Context(), formIDs)
	if err != nil {
		WriteAppError(w, err, "Erro ao exportar leads")
		return
	}
	rows := make([]any, 0, len(leads))
	for i := range leads {
		l := leads[i]
		rows = append(rows, map[string]string{
			"Nome":              l.FirstName + " " + l.LastName,
			"Email":             l.Email,
			"WhatsApp":          service.MaskWhatsApp(l.Whatsapp),
			"Status":            l.Status,
			"Data de Inscrição": service.FormatDateTimeBR(l.CreatedAt),
			"Confirmado":        fmtTimePtr(l.ConfirmedAt),
			"Compareceu":        fmtTimePtr(l.AttendedAt),
			"Código Check-in":   l.CheckinCode,
		})
	}
	WriteJSON(w, http.StatusOK, rows)
}

func fmtTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return service.FormatDateTimeBR(*t)
}
