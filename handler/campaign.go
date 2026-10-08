package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"igb-leads-go/authctx"
	"igb-leads-go/config"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// UserNoEmail is the supervisor/executive projection (no email).
type UserNoEmail struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// CampaignListRow mirrors the list include shape.
type CampaignListRow struct {
	model.Campaign
	User       *repository.UserMini `json:"user"`
	Region     *RegionMini          `json:"region"`
	Supervisor *UserNoEmail         `json:"supervisor"`
	Executive  *UserNoEmail         `json:"executive"`
}

// CampaignDetailView mirrors getById/create includes.
type CampaignDetailView struct {
	model.Campaign
	User    *repository.UserMini  `json:"user,omitempty"`
	Region  *model.Region         `json:"region"`
	Details *model.CampaignDetail `json:"details"`
	Forms   []model.Form          `json:"forms,omitempty"`
}

// CampaignWithTeam mirrors pickUp responses.
type CampaignWithTeam struct {
	model.Campaign
	User      *repository.UserMini `json:"user"`
	Executive *UserNoEmail         `json:"executive"`
}

// CampaignHandler mirrors CampaignService + CampaignController.
type CampaignHandler struct {
	cfg       *config.Config
	campaigns *repository.CampaignRepository
	details   *repository.DetailRepository
	forms     *repository.FormRepository
	leads     *repository.LeadRepository
	users     *repository.UserRepository
	regions   *repository.RegionRepository
	templates *repository.TemplateRepository
	notify    *repository.NotificationRepository
	logs      *repository.LogRepository
	auto      *service.AutoRelationship
}

func NewCampaignHandler(cfg *config.Config, campaigns *repository.CampaignRepository, details *repository.DetailRepository, forms *repository.FormRepository, leads *repository.LeadRepository, users *repository.UserRepository, regions *repository.RegionRepository, templates *repository.TemplateRepository, notify *repository.NotificationRepository, logs *repository.LogRepository, auto *service.AutoRelationship) *CampaignHandler {
	return &CampaignHandler{cfg: cfg, campaigns: campaigns, details: details, forms: forms, leads: leads, users: users, regions: regions, templates: templates, notify: notify, logs: logs, auto: auto}
}

func (h *CampaignHandler) log(action string, r *http.Request, entityType, entityID, description string, metadata map[string]any) {
	var meta []byte
	if metadata != nil {
		meta, _ = json.Marshal(metadata)
	}
	userID := authctx.UserID(r)
	var userPtr *string
	if userID != "" {
		userPtr = &userID
	}
	ip := requestIP(r)
	if err := h.logs.Insert(r.Context(), userPtr, action, &entityType, &entityID, &description, meta, &ip); err != nil {
		log.Printf("Falha ao gravar log do sistema: %v", err)
	}
}

func toNoEmail(m *repository.UserMini) *UserNoEmail {
	if m == nil {
		return nil
	}
	return &UserNoEmail{ID: m.ID, FirstName: m.FirstName, LastName: m.LastName}
}

func (h *CampaignHandler) minisFor(r *http.Request, ids ...*string) map[string]*repository.UserMini {
	uniq := []string{}
	for _, id := range ids {
		if id != nil && *id != "" {
			uniq = append(uniq, *id)
		}
	}
	minis, err := h.users.MinisByIDs(r.Context(), uniq)
	if err != nil {
		return map[string]*repository.UserMini{}
	}
	return minis
}

func checkCampaignAccess(c *model.Campaign, user *model.User) error {
	if user.Role == model.RoleAdmin {
		return nil
	}
	if user.Role == model.RoleUser && c.UserID != user.ID {
		return service.Forbidden("")
	}
	if user.Role == model.RoleSupervisor && (c.RegionID == nil || user.RegionID == nil || *c.RegionID != *user.RegionID) {
		return service.Forbidden("")
	}
	if user.Role == model.RoleExecutive && (c.TrafficManagerID == nil || *c.TrafficManagerID != user.ID) {
		return service.Forbidden("")
	}
	return nil
}

func checkOwnerOrAdmin(c *model.Campaign, user *model.User) error {
	if user.Role != model.RoleAdmin && c.UserID != user.ID {
		return service.Forbidden("")
	}
	return nil
}

func (h *CampaignHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	user := authctx.CurrentUser(r)
	f := repository.CampaignFilter{
		Status: q.Get("status"), RegionID: q.Get("region_id"),
		Page: page, Limit: limit, Search: q.Get("search"),
	}
	if uid := q.Get("user_id"); uid != "" && (user.Role == model.RoleAdmin || user.Role == model.RoleSupervisor) {
		f.UserID = uid
	}
	switch user.Role {
	case model.RoleUser:
		f.UserID = user.ID
	case model.RoleSupervisor:
		if user.RegionID != nil {
			f.RegionID = *user.RegionID
		}
	case model.RoleExecutive:
		f.TrafficManagerID = user.ID
	}
	campaigns, total, err := h.campaigns.List(r.Context(), f)
	if err != nil {
		WriteAppError(w, err, "Erro ao listar campanhas")
		return
	}
	userIDs := []string{}
	regionIDs := []string{}
	for i := range campaigns {
		userIDs = append(userIDs, campaigns[i].UserID)
		if campaigns[i].SupervisorID != nil {
			userIDs = append(userIDs, *campaigns[i].SupervisorID)
		}
		if campaigns[i].TrafficManagerID != nil {
			userIDs = append(userIDs, *campaigns[i].TrafficManagerID)
		}
		if campaigns[i].RegionID != nil {
			regionIDs = append(regionIDs, *campaigns[i].RegionID)
		}
	}
	minis, _ := h.users.MinisByIDs(r.Context(), userIDs)
	regions, _ := h.regions.ByIDs(r.Context(), regionIDs)
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	rows := make([]any, 0, len(campaigns))
	for i := range campaigns {
		c := campaigns[i]
		var region *RegionMini
		if c.RegionID != nil {
			region = toMini(regions[*c.RegionID])
		}
		rows = append(rows, CampaignListRow{
			Campaign: c, User: minis[c.UserID], Region: region,
			Supervisor: toNoEmail(minis[strOrEmpty(c.SupervisorID)]),
			Executive:  toNoEmail(minis[strOrEmpty(c.TrafficManagerID)]),
		})
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"data": rows,
		"pagination": map[string]any{
			"page": f.Page, "limit": f.Limit, "total": total,
			"totalPages": (total + f.Limit - 1) / f.Limit,
		},
	})
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (h *CampaignHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	c, err := h.campaigns.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteAppError(w, service.NotFound("Campanha não encontrada"), "Erro ao buscar campanha")
		return
	}
	if err := checkCampaignAccess(c, authctx.CurrentUser(r)); err != nil {
		WriteAppError(w, err, "Erro ao buscar campanha")
		return
	}
	view, err := h.detailView(r, c, true, true)
	if err != nil {
		WriteAppError(w, err, "Erro ao buscar campanha")
		return
	}
	WriteJSON(w, http.StatusOK, view)
}

func (h *CampaignHandler) detailView(r *http.Request, c *model.Campaign, withUser, withForms bool) (*CampaignDetailView, error) {
	view := &CampaignDetailView{Campaign: *c}
	if withUser {
		minis, err := h.users.MinisByIDs(r.Context(), []string{c.UserID})
		if err != nil {
			return nil, err
		}
		view.User = minis[c.UserID]
	}
	if c.RegionID != nil {
		region, err := h.regions.ByID(r.Context(), *c.RegionID)
		if err != nil && err != pgx.ErrNoRows {
			return nil, err
		}
		view.Region = region
	}
	detail, err := h.details.ByCampaign(r.Context(), c.ID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	view.Details = detail
	if withForms {
		forms, err := h.forms.ByCampaignIDs(r.Context(), []string{c.ID})
		if err != nil {
			return nil, err
		}
		view.Forms = forms
		if view.Forms == nil {
			view.Forms = []model.Form{}
		}
	}
	return view, nil
}

// jsonField decodes a body field into JSONB bytes (nil when absent/null).
func jsonField(raw map[string]any, key string) ([]byte, bool) {
	v, present := raw[key]
	if !present || v == nil {
		return nil, present
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, true
	}
	return b, true
}

func strField(raw map[string]any, key string) (*string, bool) {
	v, present := raw[key]
	if !present || v == nil {
		return nil, present
	}
	if s, ok := v.(string); ok {
		if s == "" {
			return nil, true
		}
		return &s, true
	}
	return nil, true
}

func (h *CampaignHandler) Create(w http.ResponseWriter, r *http.Request) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	getStr := func(key string) string {
		s, _ := raw[key].(string)
		return s
	}
	if getStr("title") == "" {
		WriteError(w, http.StatusBadRequest, "Título é obrigatório")
		return
	}
	userID := authctx.UserID(r)
	regionID, err := h.users.RegionIDOf(r.Context(), userID)
	if err != nil {
		WriteAppError(w, err, "Erro ao criar campanha")
		return
	}
	now := now()
	paid := "pending"
	cta := "Fazer Inscrição"
	c := &model.Campaign{
		ID: uuid.NewString(), CreatedAt: now, UpdatedAt: now,
		Title: getStr("title"), Status: "payment_pending",
		Platform: "meta_ads", LPTemplate: strPtr("generic"),
		UserID: userID, RegionID: regionID,
		PaymentStatus: &paid, FormCTAText: &cta,
	}
	if v := getStr("description"); v != "" {
		c.Description = &v
	}
	if v := getStr("budget"); v != "" {
		c.Budget = &v
	} else if n, ok := raw["budget"].(float64); ok && n != 0 {
		s := strconv.FormatFloat(n, 'f', 2, 64)
		c.Budget = &s
	}
	if v := getStr("platform"); v != "" {
		c.Platform = v
	}
	for _, dk := range []string{"start_date", "end_date", "event_date"} {
		if v := getStr(dk); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				if t2, err2 := time.Parse("2006-01-02", v); err2 == nil {
					t, err = t2, nil
				}
			}
			if err != nil {
				WriteError(w, http.StatusBadRequest, "Data inválida em "+dk)
				return
			}
			switch dk {
			case "start_date":
				c.StartDate = &t
			case "end_date":
				c.EndDate = &t
			case "event_date":
				c.EventDate = &t
			}
		}
	}
	optStr := []string{"event_time", "location", "target_audience", "objectives",
		"address", "address_number", "address_neighborhood", "address_city", "address_state",
		"address_zipcode", "maps_url", "google_maps_link", "latitude", "longitude",
		"responsible_name", "snack_price", "form_title", "form_description", "form_header_text", "payment_url"}
	for _, k := range optStr {
		if v := getStr(k); v != "" {
			setCampaignStr(c, k, v)
		}
	}
	if v := getStr("form_cta_text"); v != "" {
		c.FormCTAText = &v
	}
	if v := getStr("lp_template"); v != "" {
		c.LPTemplate = &v
	}
	if v := getStr("responsible_whatsapp"); v != "" {
		d := service.DigitsOnly(v)
		c.ResponsibleWhatsapp = &d
	}
	if b, ok := raw["auto_relationship"].(bool); ok {
		c.AutoRelationship = b
	}
	for _, jk := range []string{"weekdays", "event_dates", "turmas", "template_config", "products", "form_fields"} {
		if b, present := jsonField(raw, jk); present && len(b) > 0 {
			setCampaignJSON(c, jk, b)
		}
	}
	if v := getStr("whatsapp_confirmation_msg"); v != "" {
		c.WhatsappConfirmationMsg = &v
	}
	if v := getStr("whatsapp_voucher_msg"); v != "" {
		c.WhatsappVoucherMsg = &v
	}
	if err := h.campaigns.Create(r.Context(), c); err != nil {
		WriteAppError(w, err, "Erro ao criar campanha")
		return
	}
	h.log("CAMPAIGN_CREATED", r, "campaign", c.ID,
		fmt.Sprintf("Usuário %s criou a campanha #%d: %s", userID, c.DisplayID, c.Title),
		map[string]any{"display_id": c.DisplayID})

	if details, ok := raw["details"].(map[string]any); ok {
		d := &model.CampaignDetail{ID: uuid.NewString(), CampaignID: c.ID, CreatedAt: now, UpdatedAt: now}
		applyDetailFields(d, details)
		if err := h.details.Create(r.Context(), d); err != nil {
			WriteAppError(w, err, "Erro ao criar campanha")
			return
		}
	}

	slug := service.SlugifyCampaign(c.Title)
	if taken, err := h.forms.SlugTaken(r.Context(), slug); err != nil {
		WriteAppError(w, err, "Erro ao criar campanha")
		return
	} else if taken {
		slug = fmt.Sprintf("%s-%d", slug, now.UnixMilli())
	}
	formDesc := getStr("form_description")
	if formDesc == "" {
		formDesc = "Preencha o formulário para se inscrever"
	}
	if schedule := h.scheduleText(raw, c); schedule != "" {
		formDesc += " | " + schedule
	}
	formTitle := getStr("form_title")
	if formTitle == "" {
		formTitle = "Inscrição - " + c.Title
	}
	var customFields []byte
	if b, present := jsonField(raw, "form_fields"); present && len(b) > 0 {
		customFields = b
	} else {
		customFields = []byte("[]")
	}
	form := &model.Form{
		ID: uuid.NewString(), CampaignID: c.ID, Slug: slug, Title: formTitle,
		CustomFields: customFields, IsActive: false, CreatedAt: now, UpdatedAt: now,
		PublicToken: uuid.NewString(),
	}
	form.Description = &formDesc
	if err := h.forms.Create(r.Context(), form); err != nil {
		WriteAppError(w, err, "Erro ao criar campanha")
		return
	}

	objectives := getStr("objectives")
	if objectives == "" {
		objectives = "camara_publica"
	}
	for _, t := range service.DefaultsFor(objectives) {
		if _, err := h.templates.ByCampaignAndKey(r.Context(), c.ID, t.Key); err == nil {
			continue
		}
		tpl := &model.MessageTemplate{
			ID: uuid.NewString(), CampaignID: c.ID, Key: t.Key, Label: t.Label,
			Content: t.Content, SortOrder: t.SortOrder, IsActive: true,
			CreatedAt: now, UpdatedAt: now, Phase: "general", IsEditable: true,
		}
		if err := h.templates.Create(r.Context(), tpl); err != nil {
			WriteAppError(w, err, "Erro ao criar campanha")
			return
		}
	}

	fresh, err := h.campaigns.ByID(r.Context(), c.ID)
	if err != nil {
		WriteAppError(w, err, "Erro ao criar campanha")
		return
	}
	view, err := h.detailView(r, fresh, false, true)
	if err != nil {
		WriteAppError(w, err, "Erro ao criar campanha")
		return
	}
	WriteJSON(w, http.StatusCreated, view)
}

// scheduleText mirrors the form description schedule suffix.
func (h *CampaignHandler) scheduleText(raw map[string]any, c *model.Campaign) string {
	parts := []string{}
	if eds, ok := raw["event_dates"].([]any); ok && len(eds) > 0 {
		for _, e := range eds {
			m, _ := e.(map[string]any)
			if m == nil {
				continue
			}
			date, _ := m["date"].(string)
			if date == "" {
				continue
			}
			t, err := service.ParseEventDate(date)
			if err != nil {
				continue
			}
			if tm, _ := m["time"].(string); tm != "" {
				parts = append(parts, service.FormatDateBR(t)+" às "+tm)
			} else {
				parts = append(parts, service.FormatDateBR(t))
			}
		}
	} else if c.EventDate != nil {
		if c.EventTime != nil && *c.EventTime != "" {
			parts = append(parts, service.FormatDateBR(*c.EventDate)+" às "+*c.EventTime)
		} else {
			parts = append(parts, service.FormatDateBR(*c.EventDate))
		}
	}
	if wds, ok := raw["weekdays"].([]any); ok && len(wds) > 0 {
		dayMap := map[string]string{"seg": "Segunda", "ter": "Terça", "qua": "Quarta", "qui": "Quinta", "sex": "Sexta", "sab": "Sábado", "dom": "Domingo"}
		days := []string{}
		for _, w := range wds {
			if d, ok := w.(string); ok {
				if n, ok := dayMap[d]; ok {
					days = append(days, n)
				} else {
					days = append(days, d)
				}
			}
		}
		s := "Toda " + strings.Join(days, " e ")
		if c.EventTime != nil && *c.EventTime != "" {
			s += " às " + *c.EventTime
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " · ")
}

func strPtr(s string) *string { return &s }

func setCampaignStr(c *model.Campaign, key, v string) {
	switch key {
	case "location":
		c.Location = &v
	case "target_audience":
		c.TargetAudience = &v
	case "objectives":
		c.Objectives = &v
	case "event_time":
		c.EventTime = &v
	case "address":
		c.Address = &v
	case "address_number":
		c.AddressNumber = &v
	case "address_neighborhood":
		c.AddressNeighborhood = &v
	case "address_city":
		c.AddressCity = &v
	case "address_state":
		c.AddressState = &v
	case "address_zipcode":
		c.AddressZipcode = &v
	case "maps_url":
		c.MapsURL = &v
	case "google_maps_link":
		c.GoogleMapsLink = &v
	case "latitude":
		c.Latitude = &v
	case "longitude":
		c.Longitude = &v
	case "responsible_name":
		c.ResponsibleName = &v
	case "snack_price":
		c.SnackPrice = &v
	case "form_title":
		c.FormTitle = &v
	case "form_description":
		c.FormDescription = &v
	case "form_header_text":
		c.FormHeaderText = &v
	case "payment_url":
		c.PaymentURL = &v
	}
}

func setCampaignJSON(c *model.Campaign, key string, b []byte) {
	switch key {
	case "weekdays":
		c.Weekdays = b
	case "event_dates":
		c.EventDates = b
	case "turmas":
		c.Turmas = b
	case "template_config":
		c.TemplateConfig = b
	case "products":
		c.Products = b
	case "form_fields":
		c.FormFields = b
	}
}

func applyDetailFields(d *model.CampaignDetail, m map[string]any) {
	if v, ok := m["ad_creative_text"].(string); ok && v != "" {
		d.AdCreativeText = &v
	}
	if v, ok := m["ad_image_url"].(string); ok && v != "" {
		d.AdImageURL = &v
	}
	if v, ok := m["landing_page_url"].(string); ok && v != "" {
		d.LandingPageURL = &v
	}
	if v, ok := m["call_to_action"].(string); ok && v != "" {
		d.CallToAction = &v
	}
	if v, present := m["target_demographics"]; present && v != nil {
		if b, err := json.Marshal(v); err == nil {
			d.TargetDemographics = b
		}
	}
	if v, present := m["custom_fields"]; present && v != nil {
		if b, err := json.Marshal(v); err == nil {
			d.CustomFields = b
		}
	}
}
