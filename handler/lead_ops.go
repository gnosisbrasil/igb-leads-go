package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"igb-leads-go/authctx"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

var validLeadStatuses = map[string]bool{
	"new": true, "confirmed": true, "contact_later": true,
	"present": true, "converted": true, "lost": true,
}

func (h *LeadHandler) History(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	user := authctx.CurrentUser(r)
	f := repository.HistoryFilter{
		City: q.Get("city"), State: q.Get("state"), Status: q.Get("status"),
		Interest: q.Get("type"), Page: page, Limit: limit,
	}
	if user.Role != model.RoleAdmin {
		var campaignIDs []string
		var err error
		switch user.Role {
		case model.RoleUser:
			campaignIDs, err = h.campaigns.IDsByFilter(r.Context(), user.ID, user.Role, "", "")
		case model.RoleExecutive:
			campaignIDs, err = h.historyExecutiveCampaigns(r)
		case model.RoleSupervisor:
			if user.RegionID == nil {
				WriteJSON(w, http.StatusOK, map[string]any{
					"data":       []any{},
					"pagination": map[string]any{"page": page, "limit": limit, "total": 0, "totalPages": 0},
				})
				return
			}
			campaignIDs, err = h.campaigns.IDsByFilter(r.Context(), user.ID, user.Role, *user.RegionID, "")
		}
		if err != nil {
			WriteAppError(w, err, "Erro ao buscar histórico de leads")
			return
		}
		if len(campaignIDs) == 0 {
			if page < 1 {
				page = 1
			}
			if limit < 1 {
				limit = 50
			}
			WriteJSON(w, http.StatusOK, map[string]any{
				"data":       []any{},
				"pagination": map[string]any{"page": page, "limit": limit, "total": 0, "totalPages": 0},
			})
			return
		}
		f.Scoped = true
		f.CampaignIDs = campaignIDs
	}
	rows, total, err := h.leads.History(r.Context(), f)
	if err != nil {
		WriteAppError(w, err, "Erro ao buscar histórico de leads")
		return
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	data := make([]any, 0, len(rows))
	for _, hrow := range rows {
		l := hrow.Lead
		city := l.City
		if city == nil {
			city = hrow.CampaignCity
		}
		state := l.State
		if state == nil {
			state = hrow.CampaignState
		}
		title := "Campanha removida"
		if hrow.CampaignTitle != nil {
			title = *hrow.CampaignTitle
		}
		var meta any
		if len(l.Metadata) > 0 {
			meta = json.RawMessage(append([]byte(nil), l.Metadata...))
		}
		data = append(data, map[string]any{
			"id": l.ID, "first_name": l.FirstName, "last_name": l.LastName,
			"whatsapp": service.MaskWhatsApp(l.Whatsapp), "email": l.Email,
			"status": l.Status, "confirmed_at": l.ConfirmedAt, "attended_at": l.AttendedAt,
			"created_at": l.CreatedAt, "campaign_id": hrow.CampaignID,
			"campaign_title": title, "campaign_display_id": hrow.CampaignDisplay,
			"city": city, "state": state, "metadata": meta,
		})
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]any{
			"page": page, "limit": limit, "total": total,
			"totalPages": (total + limit - 1) / limit,
		},
	})
}

func (h *LeadHandler) historyExecutiveCampaigns(r *http.Request) ([]string, error) {
	user := authctx.CurrentUser(r)
	own, err := h.campaigns.IDsByFilter(r.Context(), user.ID, model.RoleExecutive, "", "")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, id := range own {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if user.RegionID != nil {
		regional, err := h.campaigns.IDsByFilter(r.Context(), "", model.RoleSupervisor, *user.RegionID, "")
		if err != nil {
			return nil, err
		}
		for _, id := range regional {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out, nil
}

func (h *LeadHandler) GetStates(w http.ResponseWriter, r *http.Request) {
	user := authctx.CurrentUser(r)
	set := map[string]bool{}
	campaignStates, err := h.campaigns.DistinctStates(r.Context(), user.ID, user.Role, strOrEmptyPtr(user.RegionID))
	if err != nil {
		WriteAppError(w, err, "Erro ao buscar estados")
		return
	}
	for _, s := range campaignStates {
		set[strings.ToUpper(s)] = true
	}
	var campaignIDs []string
	scoped := false
	if user.Role != model.RoleAdmin {
		switch user.Role {
		case model.RoleUser:
			campaignIDs, err = h.campaigns.IDsByFilter(r.Context(), user.ID, user.Role, "", "")
		case model.RoleExecutive:
			campaignIDs, err = h.historyExecutiveCampaigns(r)
		case model.RoleSupervisor:
			if user.RegionID == nil {
				WriteJSON(w, http.StatusOK, []string{})
				return
			}
			campaignIDs, err = h.campaigns.IDsByFilter(r.Context(), user.ID, user.Role, *user.RegionID, "")
		}
		if err != nil {
			WriteAppError(w, err, "Erro ao buscar estados")
			return
		}
		if len(campaignIDs) > 0 {
			scoped = true
		}
	}
	leadStates, err := h.leads.DistinctStates(r.Context(), campaignIDs, scoped)
	if err != nil {
		WriteAppError(w, err, "Erro ao buscar estados")
		return
	}
	for _, s := range leadStates {
		set[strings.ToUpper(s)] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	WriteJSON(w, http.StatusOK, out)
}

func (h *LeadHandler) ValidateCheckin(w http.ResponseWriter, r *http.Request) {
	lead, err := h.leads.ByCheckinCode(r.Context(), r.PathValue("code"))
	if err != nil {
		WriteAppError(w, service.NotFound("Código de check-in inválido"), "Erro ao validar check-in")
		return
	}
	WriteJSON(w, http.StatusOK, lead)
}

func (h *LeadHandler) DoCheckin(w http.ResponseWriter, r *http.Request) {
	lead, err := h.leads.ByCheckinCode(r.Context(), r.PathValue("code"))
	if err != nil {
		WriteAppError(w, service.NotFound("Código de check-in inválido"), "Erro ao fazer check-in")
		return
	}
	if lead.CheckinAt != nil {
		WriteAppError(w, service.BadRequest("Check-in já foi realizado"), "Erro ao fazer check-in")
		return
	}
	now := now()
	if err := h.leads.UpdateFields(r.Context(), lead.ID, map[string]any{
		"status": "present", "checkin_at": now, "attended_at": now,
	}); err != nil {
		WriteAppError(w, err, "Erro ao fazer check-in")
		return
	}
	fresh, err := h.leads.ByID(r.Context(), lead.ID)
	if err != nil {
		WriteAppError(w, err, "Erro ao fazer check-in")
		return
	}
	WriteJSON(w, http.StatusOK, fresh)
}

func (h *LeadHandler) ValidateConfirm(w http.ResponseWriter, r *http.Request) {
	lead, err := h.leads.ByCheckinCode(r.Context(), r.PathValue("code"))
	if err != nil {
		WriteAppError(w, service.NotFound("Código de check-in inválido"), "Erro ao validar confirmação")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"valid": true,
		"lead": map[string]any{
			"first_name": lead.FirstName, "last_name": lead.LastName,
			"email": lead.Email, "status": lead.Status,
		},
	})
}

func (h *LeadHandler) DoConfirm(w http.ResponseWriter, r *http.Request) {
	lead, err := h.leads.ByCheckinCode(r.Context(), r.PathValue("code"))
	if err != nil {
		WriteAppError(w, service.NotFound("Código de check-in inválido"), "Erro ao confirmar")
		return
	}
	if lead.ConfirmedAt != nil {
		WriteError(w, http.StatusBadRequest, "Confirmação já foi realizada")
		return
	}
	if err := h.leads.UpdateFields(r.Context(), lead.ID, map[string]any{
		"status": "confirmed", "confirmed_at": now(),
	}); err != nil {
		WriteAppError(w, err, "Erro ao confirmar")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Presença confirmada!",
		"lead":    map[string]string{"first_name": lead.FirstName, "last_name": lead.LastName},
	})
}

// ServeQRImage serves the check-in PNG (both /api/qr/:code and /api/leads/qr/:code).
func (h *LeadHandler) ServeQRImage(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("checkin_code")
	if code == "" {
		code = r.PathValue("code")
	}
	lead, err := h.leads.ByCheckinCode(r.Context(), code)
	if err != nil {
		WriteError(w, http.StatusNotFound, "QR Code não encontrado")
		return
	}
	size := 300
	if n, err := strconv.Atoi(r.URL.Query().Get("size")); err == nil && n >= 100 && n <= 1200 {
		size = n
	}
	png, err := service.GenerateQRPNGSize(lead.CheckinCode, size)
	if err != nil {
		WriteError(w, http.StatusNotFound, "QR Code não encontrado")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(png)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (h *LeadHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string          `json:"status"`
		Notes  json.RawMessage `json:"notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := r.PathValue("id")
	lead, err := h.leads.ByID(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Lead não encontrado")
		return
	}
	if err := h.checkLeadAccess(r, lead); err != nil {
		WriteAppError(w, err, "Erro ao atualizar status")
		return
	}
	if body.Status != "" && !validLeadStatuses[body.Status] {
		WriteError(w, http.StatusBadRequest, "Status inválido")
		return
	}
	update := map[string]any{}
	if body.Status != "" {
		update["status"] = body.Status
	} else {
		update["status"] = lead.Status
	}
	if len(body.Notes) > 0 {
		var notes *string
		if string(body.Notes) != "null" {
			var s string
			if err := json.Unmarshal(body.Notes, &s); err == nil {
				notes = &s
			}
		}
		update["notes"] = notes
	}
	if err := h.leads.UpdateFields(r.Context(), id, update); err != nil {
		WriteAppError(w, err, "Erro ao atualizar status")
		return
	}
	fresh, err := h.leads.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, err, "Erro ao atualizar status")
		return
	}
	WriteJSON(w, http.StatusOK, fresh)
}

func (h *LeadHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	lead, err := h.leads.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Lead não encontrado"), "Erro ao deletar lead")
		return
	}
	if err := h.checkLeadAccess(r, lead); err != nil {
		WriteAppError(w, err, "Erro ao deletar lead")
		return
	}
	name := lead.FirstName + " " + lead.LastName
	user := authctx.CurrentUser(r)
	if err := h.leads.Delete(r.Context(), id); err != nil {
		WriteAppError(w, err, "Erro ao deletar lead")
		return
	}
	masked := "-"
	if d := service.DigitsOnly(lead.Whatsapp); len(d) >= 4 {
		masked = "***" + d[len(d)-4:]
	}
	meta, _ := json.Marshal(map[string]any{"user_id": user.ID, "user_role": user.Role, "campaign_id": nil})
	ip := requestIP(r)
	_ = h.logs.Insert(r.Context(), &user.ID, "LEAD_DELETED",
		strPtr("lead"), &id,
		strPtr(fmt.Sprintf("Lead \"%s\" deletado por %s (%s). WhatsApp: %s", name, firstOrID(user), user.Role, masked)),
		meta, &ip)
	WriteJSON(w, http.StatusOK, map[string]string{"message": "Lead removido com sucesso"})
}

func firstOrID(user *model.User) string {
	if user.FirstName != "" {
		return user.FirstName
	}
	return user.ID
}

func (h *LeadHandler) CreateManual(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FormID    string          `json:"form_id"`
		FirstName string          `json:"first_name"`
		LastName  string          `json:"last_name"`
		Whatsapp  string          `json:"whatsapp"`
		Email     string          `json:"email"`
		Status    string          `json:"status"`
		Notes     *string         `json:"notes"`
		City      *string         `json:"city"`
		State     *string         `json:"state"`
		Metadata  json.RawMessage `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	if body.FirstName == "" || body.LastName == "" || body.Whatsapp == "" || body.Email == "" || body.FormID == "" {
		WriteError(w, http.StatusBadRequest, "Nome, sobrenome, whatsapp, email e formulário são obrigatórios")
		return
	}
	if body.Status != "" && !validLeadStatuses[body.Status] {
		WriteError(w, http.StatusBadRequest, "Status inválido")
		return
	}
	form, err := h.forms.ByID(r.Context(), body.FormID)
	if err != nil {
		WriteAppError(w, service.NotFound("Formulário não encontrado"), "Erro ao criar lead")
		return
	}
	user := authctx.CurrentUser(r)
	if user.Role != model.RoleAdmin {
		ids, err := h.accessibleFormIDs(r, "")
		if err != nil {
			WriteAppError(w, err, "Erro ao criar lead")
			return
		}
		allowed := false
		for _, id := range ids {
			if id == body.FormID {
				allowed = true
				break
			}
		}
		if !allowed {
			WriteAppError(w, service.Forbidden("Acesso negado a este formulário"), "Erro ao criar lead")
			return
		}
	}
	city, state := body.City, body.State
	if c, err := h.campaigns.ByID(r.Context(), form.CampaignID); err == nil {
		if c.AddressCity != nil {
			city = c.AddressCity
		}
		if c.AddressState != nil {
			state = c.AddressState
		}
	}
	now := now()
	lead := &model.Lead{
		ID: uuid.NewString(), FormID: &form.ID,
		FirstName: body.FirstName, LastName: body.LastName,
		Whatsapp: service.DigitsOnly(body.Whatsapp), Email: body.Email,
		Status: "new", CheckinCode: newCheckinCode(), Notes: body.Notes,
		City: city, State: state, CreatedAt: now, UpdatedAt: now,
	}
	if body.Status != "" {
		lead.Status = body.Status
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
	fresh, err := h.leads.ByID(r.Context(), lead.ID)
	if err != nil {
		WriteAppError(w, err, "Erro ao criar lead")
		return
	}
	WriteJSON(w, http.StatusCreated, fresh)
}

func (h *LeadHandler) Update(w http.ResponseWriter, r *http.Request) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	id := r.PathValue("id")
	lead, err := h.leads.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Lead não encontrado"), "Erro ao atualizar lead")
		return
	}
	if err := h.checkLeadAccess(r, lead); err != nil {
		WriteAppError(w, err, "Erro ao atualizar lead")
		return
	}
	update := map[string]any{}
	for _, k := range []string{"first_name", "last_name", "whatsapp", "email", "city", "state", "notes"} {
		if v, present := raw[k]; present {
			update[k] = v
		}
	}
	if wapp, ok := update["whatsapp"].(string); ok {
		update["whatsapp"] = service.DigitsOnly(wapp)
	}
	if len(update) > 0 {
		if err := h.leads.UpdateFields(r.Context(), id, update); err != nil {
			WriteAppError(w, err, "Erro ao atualizar lead")
			return
		}
	}
	fresh, err := h.leads.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, err, "Erro ao atualizar lead")
		return
	}
	WriteJSON(w, http.StatusOK, fresh)
}
