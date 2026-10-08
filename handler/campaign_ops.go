package handler

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"igb-leads-go/authctx"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// allowedUpdateFields mirrors ALLOWED_UPDATE_FIELDS.
var allowedUpdateFields = []string{
	"title", "description", "budget", "platform",
	"start_date", "end_date", "event_date", "location",
	"target_audience", "objectives", "region_id",
	"event_time", "weekdays", "event_dates", "lp_template", "turmas",
	"auto_relationship",
	"address", "address_number", "address_neighborhood",
	"address_city", "address_state", "address_zipcode",
	"maps_url", "google_maps_link", "latitude", "longitude",
	"form_title", "form_description", "form_fields",
	"whatsapp_confirmation_msg", "whatsapp_voucher_msg",
	"form_header_text", "form_cta_text",
	"responsible_name", "responsible_whatsapp",
	"template_config", "goal_leads",
}

var jsonbUpdateFields = map[string]bool{
	"weekdays": true, "event_dates": true, "turmas": true,
	"form_fields": true, "template_config": true, "products": true,
}

// Goal updates the lead goal of a campaign at any status.
// Allowed: owner, traffic manager, admin.
func (h *CampaignHandler) Goal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GoalLeads *int `json:"goal_leads"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	if body.GoalLeads != nil && *body.GoalLeads < 0 {
		WriteError(w, http.StatusBadRequest, "Meta inválida")
		return
	}
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao salvar meta")
		return
	}
	user := authctx.CurrentUser(r)
	isTraffic := c.TrafficManagerID != nil && *c.TrafficManagerID == user.ID
	if c.UserID != user.ID && !isTraffic && user.Role != model.RoleAdmin {
		WriteAppError(w, service.Forbidden(""), "Erro ao salvar meta")
		return
	}
	fields := map[string]any{"goal_leads": nil}
	if body.GoalLeads != nil {
		fields["goal_leads"] = *body.GoalLeads
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, fields); err != nil {
		WriteAppError(w, err, "Erro ao salvar meta")
		return
	}
	updated, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, err, "Erro ao salvar meta")
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

func (h *CampaignHandler) Update(w http.ResponseWriter, r *http.Request) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao atualizar campanha")
		return
	}
	user := authctx.CurrentUser(r)
	if err := checkOwnerOrAdmin(c, user); err != nil {
		WriteAppError(w, err, "Erro ao atualizar campanha")
		return
	}
	if user.Role != model.RoleAdmin && c.Status != "draft" && c.Status != "payment_pending" {
		WriteAppError(w, service.BadRequest(fmt.Sprintf("Campanha no status '%s' não pode ser editada", c.Status)), "Erro ao atualizar campanha")
		return
	}
	fields := map[string]any{}
	for _, f := range allowedUpdateFields {
		v, present := raw[f]
		if !present {
			continue
		}
		if v == nil {
			fields[f] = nil
			continue
		}
		if jsonbUpdateFields[f] {
			b, err := json.Marshal(v)
			if err != nil {
				WriteError(w, http.StatusBadRequest, "Campo inválido: "+f)
				return
			}
			fields[f] = b
			continue
		}
		if s, ok := v.(string); ok {
			if s == "" {
				fields[f] = nil
			} else {
				fields[f] = s
			}
			continue
		}
		fields[f] = v
	}
	if f, ok := fields["responsible_whatsapp"].(string); ok {
		fields["responsible_whatsapp"] = service.DigitsOnly(f)
	}
	if len(fields) > 0 {
		if err := h.campaigns.UpdateFields(r.Context(), id, fields); err != nil {
			WriteAppError(w, err, "Erro ao atualizar campanha")
			return
		}
	}
	if details, ok := raw["details"].(map[string]any); ok {
		existing, err := h.details.ByCampaign(r.Context(), id)
		if err != nil && err != pgx.ErrNoRows {
			WriteAppError(w, err, "Erro ao atualizar campanha")
			return
		}
		if existing != nil {
			df := map[string]any{}
			for k, v := range details {
				df[k] = v
			}
			if err := h.details.UpdateFields(r.Context(), existing.ID, df); err != nil {
				WriteAppError(w, err, "Erro ao atualizar campanha")
				return
			}
		} else {
			now := now()
			d := &model.CampaignDetail{ID: uuid.NewString(), CampaignID: id, CreatedAt: now, UpdatedAt: now}
			applyDetailFields(d, details)
			if err := h.details.Create(r.Context(), d); err != nil {
				WriteAppError(w, err, "Erro ao atualizar campanha")
				return
			}
		}
	}
	fresh, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, err, "Erro ao atualizar campanha")
		return
	}
	view, err := h.detailView(r, fresh, false, false)
	if err != nil {
		WriteAppError(w, err, "Erro ao atualizar campanha")
		return
	}
	WriteJSON(w, http.StatusOK, view)
}

func (h *CampaignHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao deletar campanha")
		return
	}
	user := authctx.CurrentUser(r)
	if user.Role != model.RoleAdmin {
		if c.UserID != user.ID {
			WriteAppError(w, service.Forbidden(""), "Erro ao deletar campanha")
			return
		}
		if c.Status != "completed" && c.Status != "cancelled" {
			WriteAppError(w, service.BadRequest("Campanha só pode ser deletada após concluída ou cancelada"), "Erro ao deletar campanha")
			return
		}
	}
	forms, err := h.forms.ByCampaignIDs(r.Context(), []string{id})
	if err != nil {
		WriteAppError(w, err, "Erro ao deletar campanha")
		return
	}
	if len(forms) > 0 {
		formIDs := make([]string, 0, len(forms))
		for i := range forms {
			formIDs = append(formIDs, forms[i].ID)
		}
		if err := h.leads.StampCityState(r.Context(), formIDs, c.AddressCity, c.AddressState); err != nil {
			WriteAppError(w, err, "Erro ao deletar campanha")
			return
		}
		if err := h.leads.DissociateForms(r.Context(), formIDs); err != nil {
			WriteAppError(w, err, "Erro ao deletar campanha")
			return
		}
		for i := range forms {
			if err := h.forms.Delete(r.Context(), forms[i].ID); err != nil {
				WriteAppError(w, err, "Erro ao deletar campanha")
				return
			}
		}
	}
	displayID := c.DisplayID
	title := c.Title
	if err := h.campaigns.Delete(r.Context(), id); err != nil {
		WriteAppError(w, err, "Erro ao deletar campanha")
		return
	}
	name := user.FirstName
	if name == "" {
		name = user.ID
	}
	h.log("CAMPAIGN_DELETED", r, "campaign", id,
		fmt.Sprintf("Campanha #%d \"%s\" deletada por %s (%s). Leads preservados.", displayID, title, name, user.Role),
		map[string]any{"display_id": displayID, "user_id": user.ID, "user_role": user.Role})
	WriteJSON(w, http.StatusOK, map[string]string{"message": "Campanha removida com sucesso. Leads preservados no histórico."})
}

// AdvanceToWaitingExecutive mirrors utils/campaignHelpers (also used by payments).
func (h *CampaignHandler) AdvanceToWaitingExecutive(r *http.Request, c *model.Campaign) error {
	return h.advanceToWaitingExecutive(r, c)
}

// advanceToWaitingExecutive mirrors utils/campaignHelpers.
func (h *CampaignHandler) advanceToWaitingExecutive(r *http.Request, c *model.Campaign) error {
	if err := h.forms.ActivateByCampaign(r.Context(), c.ID); err != nil {
		return err
	}
	if err := h.campaigns.UpdateFields(r.Context(), c.ID, map[string]any{"status": "waiting_executive"}); err != nil {
		return err
	}
	h.log("CAMPAIGN_READY_FOR_EXECUTIVE", r, "campaign", c.ID,
		fmt.Sprintf("Campanha #%d paga e disponível para executivos", c.DisplayID),
		map[string]any{"display_id": c.DisplayID})
	return nil
}

func (h *CampaignHandler) Submit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao submeter campanha")
		return
	}
	user := authctx.CurrentUser(r)
	if c.UserID != user.ID && user.Role != model.RoleAdmin {
		WriteAppError(w, service.Forbidden(""), "Erro ao submeter campanha")
		return
	}
	if c.Status == "draft" {
		if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{"status": "payment_pending"}); err != nil {
			WriteAppError(w, err, "Erro ao submeter campanha")
			return
		}
		c.Status = "payment_pending"
		forms, err := h.forms.ByCampaignIDs(r.Context(), []string{id})
		if err != nil {
			WriteAppError(w, err, "Erro ao submeter campanha")
			return
		}
		if forms == nil {
			forms = []model.Form{}
		}
		WriteJSON(w, http.StatusOK, map[string]any{"message": "Campanha enviada para pagamento",
			"campaign": CampaignWithForms{Campaign: *c, Forms: forms}})
		return
	}
	if c.Status == "payment_pending" {
		if c.PaymentStatus == nil || *c.PaymentStatus != "paid" {
			WriteAppError(w, service.BadRequest("Pagamento ainda não confirmado. Aguarde a confirmação ou contate o admin."), "Erro ao submeter campanha")
			return
		}
		if err := h.advanceToWaitingExecutive(r, c); err != nil {
			WriteAppError(w, err, "Erro ao submeter campanha")
			return
		}
		fresh, err := h.campaigns.ByID(r.Context(), id)
		if err != nil {
			WriteAppError(w, err, "Erro ao submeter campanha")
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"message": "Pagamento confirmado! Campanha disponível para executivos.", "campaign": fresh})
		return
	}
	WriteAppError(w, service.BadRequest("Campanha não pode ser submetida no status atual"), "Erro ao submeter campanha")
}

func (h *CampaignHandler) MarkAsPaid(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user := authctx.CurrentUser(r)
	if user.Role != model.RoleAdmin {
		WriteAppError(w, service.Forbidden("Apenas admin pode marcar como pago"), "Erro ao marcar campanha como paga")
		return
	}
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao marcar campanha como paga")
		return
	}
	now := now()
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{"payment_status": "paid", "payment_paid_at": now}); err != nil {
		WriteAppError(w, err, "Erro ao marcar campanha como paga")
		return
	}
	paid := "paid"
	c.PaymentStatus = &paid
	c.PaymentPaidAt = &now
	if c.Status == "payment_pending" {
		if err := h.advanceToWaitingExecutive(r, c); err != nil {
			WriteAppError(w, err, "Erro ao marcar campanha como paga")
			return
		}
		c.Status = "waiting_executive"
	}
	minis, _ := h.users.MinisByIDs(r.Context(), []string{c.UserID})
	h.notify.NotifyPaymentConfirmed(r.Context(), c.ID, c.Title, c.UserID)
	var short *UserMiniShort
	if m := minis[c.UserID]; m != nil {
		short = &UserMiniShort{ID: m.ID, Email: m.Email, FirstName: m.FirstName}
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Campanha marcada como paga", "campaign": CampaignPaidView{Campaign: *c, User: short}})
}

func (h *CampaignHandler) PickUp(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AssignExecutive string `json:"assign_executive"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao pegar campanha")
		return
	}
	user := authctx.CurrentUser(r)
	if c.Status != "waiting_executive" {
		WriteAppError(w, service.BadRequest("Campanha não está disponível para execução"), "Erro ao pegar campanha")
		return
	}
	if user.Role != model.RoleExecutive && user.Role != model.RoleSupervisor && user.Role != model.RoleAdmin {
		WriteAppError(w, service.Forbidden(""), "Erro ao pegar campanha")
		return
	}
	assignedID := user.ID
	if body.AssignExecutive != "" && (user.Role == model.RoleSupervisor || user.Role == model.RoleAdmin) {
		target, err := h.users.ByID(r.Context(), body.AssignExecutive)
		if err != nil || target.Role != model.RoleExecutive {
			WriteAppError(w, service.BadRequest("Executivo inválido"), "Erro ao pegar campanha")
			return
		}
		assignedID = body.AssignExecutive
	}
	now := now()
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{
		"status": "accepted", "traffic_manager_id": assignedID, "accepted_at": now,
	}); err != nil {
		WriteAppError(w, err, "Erro ao pegar campanha")
		return
	}
	h.notify.NotifyCampaignAssigned(r.Context(), c.ID, c.Title, assignedID)
	fresh, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, err, "Erro ao pegar campanha")
		return
	}
	minis, _ := h.users.MinisByIDs(r.Context(), []string{fresh.UserID, assignedID})
	msg := "Campanha aceita com sucesso"
	if body.AssignExecutive != "" {
		msg = "Campanha atribuída ao executivo"
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": msg,
		"campaign": CampaignWithTeam{Campaign: *fresh, User: minis[fresh.UserID], Executive: toNoEmail(minis[assignedID])}})
}

func (h *CampaignHandler) Reject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao rejeitar campanha")
		return
	}
	user := authctx.CurrentUser(r)
	if user.Role != model.RoleAdmin && user.Role != model.RoleSupervisor && user.Role != model.RoleExecutive {
		WriteAppError(w, service.Forbidden(""), "Erro ao rejeitar campanha")
		return
	}
	if c.Status != "waiting_executive" && c.Status != "accepted" {
		WriteAppError(w, service.BadRequest("Campanha não pode ser rejeitada no status atual"), "Erro ao rejeitar campanha")
		return
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{
		"status": "draft", "rejection_reason": body.Reason, "traffic_manager_id": nil,
	}); err != nil {
		WriteAppError(w, err, "Erro ao rejeitar campanha")
		return
	}
	c.Status = "draft"
	c.RejectionReason = &body.Reason
	c.TrafficManagerID = nil
	minis, _ := h.users.MinisByIDs(r.Context(), []string{c.UserID})
	h.notify.NotifyCampaignRejection(r.Context(), c.ID, c.Title, c.UserID, body.Reason)
	var short *UserMiniShort
	if m := minis[c.UserID]; m != nil {
		short = &UserMiniShort{ID: m.ID, Email: m.Email, FirstName: m.FirstName}
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Campanha rejeitada",
		"campaign": CampaignPaidView{Campaign: *c, User: short}})
}

func (h *CampaignHandler) AssignExecutive(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TrafficManagerID string `json:"traffic_manager_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao atribuir executivo")
		return
	}
	exec, err := h.users.ByID(r.Context(), body.TrafficManagerID)
	if err != nil || exec.Role != model.RoleExecutive {
		WriteAppError(w, service.BadRequest("Executivo inválido"), "Erro ao atribuir executivo")
		return
	}
	status := c.Status
	if status == "waiting_executive" {
		status = "accepted"
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{
		"traffic_manager_id": body.TrafficManagerID, "status": status,
	}); err != nil {
		WriteAppError(w, err, "Erro ao atribuir executivo")
		return
	}
	c.TrafficManagerID = &body.TrafficManagerID
	c.Status = status
	h.notify.NotifyCampaignAssigned(r.Context(), c.ID, c.Title, body.TrafficManagerID)
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Executivo atribuído com sucesso", "campaign": c})
}

func (h *CampaignHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao iniciar campanha")
		return
	}
	user := authctx.CurrentUser(r)
	// NOTE: the Node guard is a tautology (always passes); the intended
	// owner/staff/assignee check is implemented here.
	assigned := c.TrafficManagerID != nil && *c.TrafficManagerID == user.ID
	if c.UserID != user.ID && user.Role != model.RoleAdmin && user.Role != model.RoleSupervisor && !assigned {
		WriteAppError(w, service.Forbidden(""), "Erro ao iniciar campanha")
		return
	}
	if c.Status != "accepted" {
		WriteAppError(w, service.BadRequest("Campanha precisa estar aceita por um executivo para ser iniciada"), "Erro ao iniciar campanha")
		return
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{"status": "in_progress"}); err != nil {
		WriteAppError(w, err, "Erro ao iniciar campanha")
		return
	}
	c.Status = "in_progress"
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Campanha em andamento", "campaign": c})
}

func (h *CampaignHandler) Complete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao finalizar campanha")
		return
	}
	if c.Status != "in_progress" {
		WriteAppError(w, service.BadRequest("Campanha precisa estar em andamento para ser finalizada"), "Erro ao finalizar campanha")
		return
	}
	update := map[string]any{"status": "completed", "completed_at": now()}
	if c.AutoRelationship {
		if cost, err := h.auto.CalculateCampaignCost(r.Context(), id); err == nil && cost != nil {
			update["auto_relationship_cost"] = *cost
		}
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, update); err != nil {
		WriteAppError(w, err, "Erro ao finalizar campanha")
		return
	}
	c.Status = "completed"
	now := now()
	c.CompletedAt = &now
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Campanha finalizada", "campaign": c})
}

func (h *CampaignHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao cancelar campanha")
		return
	}
	if c.Status != "draft" && c.Status != "payment_pending" && c.Status != "waiting_executive" && c.Status != "accepted" {
		WriteAppError(w, service.BadRequest("Campanha não pode ser cancelada no status atual"), "Erro ao cancelar campanha")
		return
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{"status": "cancelled"}); err != nil {
		WriteAppError(w, err, "Erro ao cancelar campanha")
		return
	}
	c.Status = "cancelled"
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Campanha cancelada", "campaign": c})
}

func (h *CampaignHandler) ToDraft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao voltar campanha para rascunho")
		return
	}
	if c.Status != "payment_pending" {
		WriteAppError(w, service.BadRequest("Campanha não está com pagamento pendente"), "Erro ao voltar campanha para rascunho")
		return
	}
	user := authctx.CurrentUser(r)
	if c.UserID != user.ID && user.Role != model.RoleAdmin {
		WriteAppError(w, service.Forbidden(""), "Erro ao voltar campanha para rascunho")
		return
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{"status": "draft"}); err != nil {
		WriteAppError(w, err, "Erro ao voltar campanha para rascunho")
		return
	}
	c.Status = "draft"
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Campanha voltou para rascunho", "campaign": c})
}

var proposeEditFields = []string{
	"title", "description", "address", "address_number", "address_neighborhood",
	"address_city", "address_state", "address_zipcode", "maps_url", "google_maps_link",
	"start_date", "end_date", "event_date", "event_time", "weekdays", "event_dates",
	"turmas", "products", "snack_price",
}

func (h *CampaignHandler) ProposeEdit(w http.ResponseWriter, r *http.Request) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao propor edição")
		return
	}
	user := authctx.CurrentUser(r)
	if c.UserID != user.ID && user.Role != model.RoleAdmin {
		WriteAppError(w, service.Forbidden(""), "Erro ao propor edição")
		return
	}
	switch c.Status {
	case "in_progress", "waiting_executive", "accepted":
	default:
		WriteAppError(w, service.BadRequest("Campanha precisa estar ativa para propor alterações"), "Erro ao propor edição")
		return
	}
	proposed := map[string]any{}
	for _, f := range proposeEditFields {
		if v, present := raw[f]; present {
			proposed[f] = v
		}
	}
	b, _ := json.Marshal(proposed)
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{"pending_edit": b}); err != nil {
		WriteAppError(w, err, "Erro ao propor edição")
		return
	}
	h.notify.NotifyEditProposed(r.Context(), h.users, c.ID, c.Title, c.TrafficManagerID, c.RegionID)
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Alterações propostas e aguardando aprovação", "pending_edit": proposed})
}

func (h *CampaignHandler) ApproveEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao aprovar edição")
		return
	}
	if len(c.PendingEdit) == 0 {
		WriteAppError(w, service.BadRequest("Nenhuma alteração pendente"), "Erro ao aprovar edição")
		return
	}
	if authctx.CurrentUser(r).Role == model.RoleUser {
		WriteAppError(w, service.Forbidden(""), "Erro ao aprovar edição")
		return
	}
	var proposed map[string]any
	if err := json.Unmarshal(c.PendingEdit, &proposed); err != nil {
		WriteAppError(w, err, "Erro ao aprovar edição")
		return
	}
	fields := map[string]any{"pending_edit": nil}
	for k, v := range proposed {
		if jsonbUpdateFields[k] {
			b, err := json.Marshal(v)
			if err != nil {
				WriteError(w, http.StatusBadRequest, "Campo inválido: "+k)
				return
			}
			fields[k] = b
		} else {
			fields[k] = v
		}
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, fields); err != nil {
		WriteAppError(w, err, "Erro ao aprovar edição")
		return
	}
	fresh, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, err, "Erro ao aprovar edição")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Alterações aprovadas e aplicadas", "campaign": fresh})
}

func (h *CampaignHandler) RejectEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao rejeitar edição")
		return
	}
	if len(c.PendingEdit) == 0 {
		WriteAppError(w, service.BadRequest("Nenhuma alteração pendente"), "Erro ao rejeitar edição")
		return
	}
	if authctx.CurrentUser(r).Role == model.RoleUser {
		WriteAppError(w, service.Forbidden(""), "Erro ao rejeitar edição")
		return
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{"pending_edit": nil}); err != nil {
		WriteAppError(w, err, "Erro ao rejeitar edição")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "Alterações rejeitadas"})
}

func (h *CampaignHandler) Health(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao obter saúde da campanha")
		return
	}
	formIDs, err := h.forms.IDsByCampaignIDs(r.Context(), []string{id})
	if err != nil {
		WriteAppError(w, err, "Erro ao obter saúde da campanha")
		return
	}
	if len(formIDs) == 0 {
		WriteJSON(w, http.StatusOK, map[string]any{
			"total": 0, "confirmation_sent": 0, "reminder_sent": 0, "confirmed": 0,
			"voucher_sent": 0, "motivation_sent": 0, "reminder_event_sent": 0,
			"attended": 0, "cancelled": 0, "confirmation_rate": 0, "attendance_rate": 0,
			"alerts": []any{},
		})
		return
	}
	total, _ := h.leads.CountByForms(r.Context(), formIDs)
	sent, _ := h.leads.CountNotNull(r.Context(), formIDs, "confirmation_sent_at")
	confirmed, _ := h.leads.CountNotNull(r.Context(), formIDs, "confirmed_at")
	attended, _ := h.leads.CountNotNull(r.Context(), formIDs, "attended_at")
	cancelled, _ := h.leads.CountNotNull(r.Context(), formIDs, "cancelled_at")
	alerts := []any{}
	if unconfirmed := total - sent; unconfirmed > 0 {
		alerts = append(alerts, map[string]string{"type": "warning",
			"message": fmt.Sprintf("%d lead(s) sem pedido de confirmação enviado", unconfirmed)})
	}
	if gap := sent - confirmed; gap > 5 {
		alerts = append(alerts, map[string]string{"type": "warning",
			"message": fmt.Sprintf("%d lead(s) receberam pedido mas não confirmaram", gap)})
	}
	if c.EventDate != nil {
		diffDays := int(math.Ceil(c.EventDate.Sub(now()).Hours() / 24))
		switch {
		case diffDays == 1:
			alerts = append(alerts, map[string]string{"type": "urgent", "message": "Evento é amanhã! Verifique lembretes."})
		case diffDays == 0:
			alerts = append(alerts, map[string]string{"type": "urgent", "message": "Evento é hoje!"})
		case diffDays <= 3 && diffDays > 0:
			alerts = append(alerts, map[string]string{"type": "info", "message": fmt.Sprintf("Evento em %d dias", diffDays)})
		}
	}
	if total > 0 && confirmed > 0 {
		rate := float64(attended) / float64(confirmed) * 100
		if rate < 50 && attended > 0 {
			alerts = append(alerts, map[string]string{"type": "warning",
				"message": fmt.Sprintf("Taxa de presença baixa: %.0f%%", rate)})
		}
	}
	var confRate, attRate any = 0, 0
	if total > 0 {
		confRate = strconv.FormatFloat(float64(confirmed)/float64(total)*100, 'f', 1, 64)
	}
	if confirmed > 0 {
		attRate = strconv.FormatFloat(float64(attended)/float64(confirmed)*100, 'f', 1, 64)
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"total": total, "confirmation_sent": sent, "confirmed": confirmed,
		"voucher_sent": 0, "attended": attended, "cancelled": cancelled,
		"confirmation_rate": confRate, "attendance_rate": attRate, "alerts": alerts,
	})
}

func (h *CampaignHandler) ListAvailable(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	campaigns, total, err := h.campaigns.ListAvailable(r.Context(), q.Get("region_id"), page, limit)
	if err != nil {
		WriteAppError(w, err, "Erro ao listar campanhas disponíveis")
		return
	}
	userIDs := []string{}
	regionIDs := []string{}
	for i := range campaigns {
		userIDs = append(userIDs, campaigns[i].UserID)
		if campaigns[i].RegionID != nil {
			regionIDs = append(regionIDs, *campaigns[i].RegionID)
		}
	}
	minis, _ := h.users.MinisByIDs(r.Context(), userIDs)
	regions, _ := h.regions.ByIDs(r.Context(), regionIDs)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	rows := make([]any, 0, len(campaigns))
	for i := range campaigns {
		c := campaigns[i]
		var region *RegionMini
		if c.RegionID != nil {
			region = toMini(regions[*c.RegionID])
		}
		rows = append(rows, struct {
			model.Campaign
			User   *repository.UserMini `json:"user"`
			Region *RegionMini          `json:"region"`
		}{Campaign: c, User: minis[c.UserID], Region: region})
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"data": rows,
		"pagination": map[string]any{
			"page": page, "limit": limit, "total": total,
			"totalPages": (total + limit - 1) / limit,
		},
	})
}

// UserMiniShort is the markAsPaid user projection (no last_name).
type UserMiniShort struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
}

// CampaignPaidView mirrors the markAsPaid response.
type CampaignPaidView struct {
	model.Campaign
	User *UserMiniShort `json:"user"`
}

// CampaignWithForms mirrors the submit draft response.
type CampaignWithForms struct {
	model.Campaign
	Forms []model.Form `json:"forms"`
}
