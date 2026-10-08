package handler

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"igb-leads-go/authctx"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// ReportHandler mirrors ReportController.
type ReportHandler struct {
	reports   *repository.ReportRepository
	forms     *repository.FormRepository
	users     *repository.UserRepository
	regions   *repository.RegionRepository
	logs      *repository.LogRepository
	campaigns *repository.CampaignRepository
}

func NewReportHandler(reports *repository.ReportRepository, forms *repository.FormRepository, users *repository.UserRepository, regions *repository.RegionRepository, logs *repository.LogRepository, campaigns *repository.CampaignRepository) *ReportHandler {
	return &ReportHandler{reports: reports, forms: forms, users: users, regions: regions, logs: logs, campaigns: campaigns}
}

func (h *ReportHandler) AdminReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	str := func(s string) *string { return &s }
	totalCampaigns, err1 := h.reports.CountCampaigns(ctx, nil, nil, nil)
	activeCampaigns, err2 := h.reports.CountCampaigns(ctx, str("in_progress"), nil, nil)
	completedCampaigns, err3 := h.reports.CountCampaigns(ctx, str("completed"), nil, nil)
	totalLeads, err4 := h.reports.CountLeads(ctx, nil)
	convertedLeads, err5 := h.reports.CountLeads(ctx, str("converted"))
	pendingCampaigns, err6 := h.reports.CountCampaigns(ctx, str("pending_approval"), nil, nil)
	if err := firstErr(err1, err2, err3, err4, err5, err6); err != nil {
		log.Printf("Erro ao gerar relatório admin: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"totalCampaigns":     totalCampaigns,
		"activeCampaigns":    activeCampaigns,
		"completedCampaigns": completedCampaigns,
		"totalLeads":         totalLeads,
		"conversionRate":     service.ConversionRate(convertedLeads, totalLeads),
		"pendingApprovals":   pendingCampaigns,
	})
}

// LogUser mirrors the user attributes included in system logs.
type LogUser struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

// LogItem is a system log with its nested user (null when orphaned).
type LogItem struct {
	model.SystemLog
	User *LogUser `json:"user"`
}

func (h *ReportHandler) SystemLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, err1 := strconv.Atoi(firstOr(q.Get("page"), "1"))
	limit, err2 := strconv.Atoi(firstOr(q.Get("limit"), "20"))
	if err1 != nil || err2 != nil {
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	rows, err := h.logs.List(r.Context(), q.Get("search"), limit, (page-1)*limit)
	if err != nil {
		log.Printf("Erro ao buscar logs: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	ids := []string{}
	for _, l := range rows {
		if l.UserID != nil && *l.UserID != "" {
			ids = append(ids, *l.UserID)
		}
	}
	users, err := h.users.ByIDs(r.Context(), ids)
	if err != nil {
		log.Printf("Erro ao buscar logs: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	out := []LogItem{}
	for _, l := range rows {
		item := LogItem{SystemLog: l}
		if l.UserID != nil {
			if u, ok := users[*l.UserID]; ok {
				item.User = &LogUser{FirstName: u.FirstName, LastName: u.LastName, Email: u.Email}
			}
		}
		out = append(out, item)
	}
	WriteJSON(w, http.StatusOK, out)
}

// CampaignWithUser mirrors the supervisor report campaign items.
type CampaignWithUser struct {
	model.Campaign
	User *model.User `json:"user"`
}

func (h *ReportHandler) SupervisorReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, err := h.users.ByID(ctx, authctx.UserID(r))
	if err != nil || user.RegionID == nil || *user.RegionID == "" {
		WriteError(w, http.StatusBadRequest, "Usuário não possui região atribuída")
		return
	}
	regionID := *user.RegionID
	region, err := h.regions.ByID(ctx, regionID)
	if err != nil {
		log.Printf("Erro ao gerar relatório supervisor: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	str := func(s string) *string { return &s }
	totalCampaigns, err1 := h.reports.CountCampaigns(ctx, nil, &regionID, nil)
	campaignsToApprove, err2 := h.reports.CountCampaigns(ctx, str("pending_approval"), &regionID, nil)
	campaigns, err3 := h.reports.CampaignsByRegion(ctx, regionID)
	if err := firstErr(err1, err2, err3); err != nil {
		log.Printf("Erro ao gerar relatório supervisor: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	campaignIDs := make([]string, 0, len(campaigns))
	ownerIDs := make([]string, 0, len(campaigns))
	for _, c := range campaigns {
		campaignIDs = append(campaignIDs, c.ID)
		ownerIDs = append(ownerIDs, c.UserID)
	}
	formIDs, err := h.forms.IDsByCampaignIDs(ctx, campaignIDs)
	if err != nil {
		log.Printf("Erro ao gerar relatório supervisor: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	totalLeads, err1 := h.reports.CountLeadsByForms(ctx, formIDs, nil)
	convertedLeads, err2 := h.reports.CountLeadsByForms(ctx, formIDs, str("converted"))
	owners, err3 := h.users.ByIDs(ctx, ownerIDs)
	if err := firstErr(err1, err2, err3); err != nil {
		log.Printf("Erro ao gerar relatório supervisor: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	items := []CampaignWithUser{}
	for _, c := range campaigns {
		items = append(items, CampaignWithUser{Campaign: c, User: owners[c.UserID]})
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"region":             region,
		"totalCampaigns":     totalCampaigns,
		"campaignsToApprove": campaignsToApprove,
		"totalLeads":         totalLeads,
		"conversionRate":     service.ConversionRate(convertedLeads, totalLeads),
		"campaigns":          items,
	})
}

// FormWithLeads mirrors the executive report form items.
type FormWithLeads struct {
	model.Form
	Leads []repository.LeadIDStatus `json:"leads"`
}

// CampaignPerformance mirrors the executive report campaign items.
type CampaignPerformance struct {
	model.Campaign
	Forms []FormWithLeads `json:"forms"`
}

func (h *ReportHandler) ExecutiveReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := authctx.UserID(r)
	str := func(s string) *string { return &s }
	assigned, err1 := h.reports.CountCampaigns(ctx, nil, nil, &userID)
	active, err2 := h.reports.CountCampaigns(ctx, str("in_progress"), nil, &userID)
	completed, err3 := h.reports.CountCampaigns(ctx, str("completed"), nil, &userID)
	campaigns, err4 := h.reports.CampaignsByTrafficManager(ctx, userID)
	if err := firstErr(err1, err2, err3, err4); err != nil {
		log.Printf("Erro ao gerar relatório executivo: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	campaignIDs := make([]string, 0, len(campaigns))
	for _, c := range campaigns {
		campaignIDs = append(campaignIDs, c.ID)
	}
	forms, err := h.forms.ByCampaignIDs(ctx, campaignIDs)
	if err != nil {
		log.Printf("Erro ao gerar relatório executivo: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	formIDs := make([]string, 0, len(forms))
	for _, f := range forms {
		formIDs = append(formIDs, f.ID)
	}
	minis, err := h.reports.LeadsMiniByForms(ctx, formIDs)
	if err != nil {
		log.Printf("Erro ao gerar relatório executivo: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	totalLeads := 0
	convertedLeads := 0
	perf := []CampaignPerformance{}
	for _, c := range campaigns {
		items := []FormWithLeads{}
		for _, f := range forms {
			if f.CampaignID != c.ID {
				continue
			}
			leads := minis[f.ID]
			if leads == nil {
				leads = []repository.LeadIDStatus{}
			}
			totalLeads += len(leads)
			for _, l := range leads {
				if l.Status == "converted" {
					convertedLeads++
				}
			}
			items = append(items, FormWithLeads{Form: f, Leads: leads})
		}
		perf = append(perf, CampaignPerformance{Campaign: c, Forms: items})
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"assignedCampaigns":    assigned,
		"activeCampaigns":      active,
		"completedCampaigns":   completed,
		"totalLeadsGathered":   totalLeads,
		"conversionRate":       service.ConversionRateString(convertedLeads, totalLeads),
		"campaignsPerformance": perf,
	})
}

// Overview returns scoped report aggregates: totals, per-campaign and
// per-region breakdowns, and signups per day. Filters: campaign_id,
// region_id (admin only), days (default 30).
func (h *ReportHandler) Overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	user, err := h.users.ByID(ctx, authctx.UserID(r))
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "Usuário não encontrado")
		return
	}
	scope, err := service.ResolveReportScope(user.Role, user.ID, user.RegionID, q.Get("campaign_id"), q.Get("region_id"))
	if err != nil {
		if err == service.ErrRegionForbidden {
			WriteError(w, http.StatusForbidden, err.Error())
			return
		}
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if scope.CampaignID != nil && user.Role != model.RoleAdmin {
		c, err := h.campaigns.ByID(ctx, *scope.CampaignID)
		if err != nil {
			WriteError(w, http.StatusNotFound, "Campanha não encontrada")
			return
		}
		if !service.CampaignInScope(scope, c.RegionID, c.UserID, c.TrafficManagerID) {
			WriteError(w, http.StatusForbidden, "Campanha fora do seu escopo")
			return
		}
	}
	ids, err := h.reports.CampaignIDsFiltered(ctx, scope.CampaignID, scope.RegionID, scope.TrafficManagerID, scope.OwnerID)
	if err != nil {
		log.Printf("Erro no overview: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	rng := service.ParseDateRange(q.Get("from"), q.Get("to"))
	var from, to *time.Time
	if rng.HasFrom {
		from = &rng.From
	}
	if rng.HasTo {
		to = &rng.To
	}
	byStatus, err1 := h.reports.LeadStatusCounts(ctx, ids, from, to)
	byCampaign, err2 := h.reports.CampaignLeadStats(ctx, ids, from, to)
	byRegion, err3 := h.reports.RegionLeadStats(ctx, ids, from, to)
	days, _ := strconv.Atoi(firstOr(q.Get("days"), "30"))
	byDay, err4 := h.reports.LeadDailyCounts(ctx, ids, days, from, to)
	if err := firstErr(err1, err2, err3, err4); err != nil {
		log.Printf("Erro no overview: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	total := 0
	for _, n := range byStatus {
		total += n
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"totals": map[string]any{
			"campaigns":       len(ids),
			"leads":           total,
			"by_status":       byStatus,
			"conversion_rate": service.ConversionRate(byStatus["converted"], total),
		},
		"by_campaign": byCampaign,
		"by_region":   byRegion,
		"by_day":      byDay,
		"range": map[string]any{
			"from": q.Get("from"),
			"to":   q.Get("to"),
		},
	})
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
