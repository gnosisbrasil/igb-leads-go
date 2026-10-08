package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"igb-leads-go/authctx"
	"igb-leads-go/config"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// WhatsAppHandler exposes the team's meow session (QR pairing, status)
// and template test sends. Admins manage any region; supervisors only
// their own.
type WhatsAppHandler struct {
	cfg       *config.Config
	regions   *repository.RegionRepository
	templates *repository.TemplateRepository
	campaigns *repository.CampaignRepository
	leads     *repository.LeadRepository
	wa        *service.WhatsAppClient
}

func NewWhatsAppHandler(cfg *config.Config, regions *repository.RegionRepository, templates *repository.TemplateRepository, campaigns *repository.CampaignRepository, leads *repository.LeadRepository, wa *service.WhatsAppClient) *WhatsAppHandler {
	return &WhatsAppHandler{cfg: cfg, regions: regions, templates: templates, campaigns: campaigns, leads: leads, wa: wa}
}

func canManageRegion(user *model.User, regionID string) bool {
	if user == nil {
		return false
	}
	if user.Role == model.RoleAdmin {
		return true
	}
	return user.Role == model.RoleSupervisor && user.RegionID != nil && *user.RegionID == regionID
}

// ensureSession returns the region's session, creating it on meow when new.
func (h *WhatsAppHandler) ensureSession(r *http.Request, region *model.Region) (string, error) {
	if region.WhatsappSession != nil && *region.WhatsappSession != "" {
		return *region.WhatsappSession, nil
	}
	session := service.SessionIDForRegion(region.Code)
	if err := h.wa.Connect(session); err != nil {
		return "", err
	}
	if err := h.regions.UpdateWhatsApp(r.Context(), region.ID, &session, nil, nil); err != nil {
		log.Printf("Erro ao salvar sessão WhatsApp da região %s: %v", region.ID, err)
	}
	return session, nil
}

// Status reports the team's session state (public within the app).
func (h *WhatsAppHandler) Status(w http.ResponseWriter, r *http.Request) {
	region, err := h.regions.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Região não encontrada")
		return
	}
	out := map[string]any{"connected": false, "session": nil, "phone": nil, "status": "disconnected"}
	if region.WhatsappSession == nil || *region.WhatsappSession == "" {
		WriteJSON(w, http.StatusOK, out)
		return
	}
	st, err := h.wa.SessionStatus(*region.WhatsappSession)
	if err != nil {
		log.Printf("Erro ao consultar sessão %s: %v", *region.WhatsappSession, err)
		out["status"] = "error"
		WriteJSON(w, http.StatusOK, out)
		return
	}
	connected := st.Status == "connected"
	out["session"] = *region.WhatsappSession
	out["status"] = st.Status
	out["connected"] = connected
	if connected {
		out["phone"] = st.Phone
		if region.WhatsappPhone == nil || *region.WhatsappPhone != st.Phone {
			now := time.Now().UTC()
			_ = h.regions.UpdateWhatsApp(r.Context(), region.ID, region.WhatsappSession, &st.Phone, &now)
		}
	}
	WriteJSON(w, http.StatusOK, out)
}

// QR returns the pairing QR image for the team's session.
func (h *WhatsAppHandler) QR(w http.ResponseWriter, r *http.Request) {
	user := authctx.CurrentUser(r)
	region, err := h.regions.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Região não encontrada")
		return
	}
	if !canManageRegion(user, region.ID) {
		WriteError(w, http.StatusForbidden, "Sem permissão para esta equipe")
		return
	}
	session, err := h.ensureSession(r, region)
	if err != nil {
		log.Printf("Erro ao criar sessão %s: %v", region.Code, err)
		WriteError(w, http.StatusBadGateway, "Falha ao falar com o serviço WhatsApp")
		return
	}
	st, qrImage, err := h.wa.SessionQR(session)
	if err != nil {
		log.Printf("Erro ao obter QR %s: %v", session, err)
		WriteError(w, http.StatusBadGateway, "Falha ao obter QR Code")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"session": session, "status": st.Status, "phone": st.Phone, "qrImage": qrImage,
	})
}

// Disconnect drops the team's session connection.
func (h *WhatsAppHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	user := authctx.CurrentUser(r)
	region, err := h.regions.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Região não encontrada")
		return
	}
	if !canManageRegion(user, region.ID) {
		WriteError(w, http.StatusForbidden, "Sem permissão para esta equipe")
		return
	}
	if region.WhatsappSession != nil && *region.WhatsappSession != "" {
		if err := h.wa.Disconnect(*region.WhatsappSession); err != nil {
			log.Printf("Erro ao desconectar %s: %v", *region.WhatsappSession, err)
		}
		_ = h.regions.UpdateWhatsApp(r.Context(), region.ID, region.WhatsappSession, nil, nil)
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Sessão desconectada"})
}

// TestSend delivers a template to an arbitrary number for preview.
func (h *WhatsAppHandler) TestSend(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TemplateID string `json:"template_id"`
		To         string `json:"to"`
		LeadID     string `json:"lead_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	to := service.DigitsOnly(body.To)
	if len(to) < 10 {
		WriteError(w, http.StatusBadRequest, "Número de destino inválido")
		return
	}
	tpl, err := h.templates.ByID(r.Context(), body.TemplateID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Template não encontrado")
		return
	}
	campaign, err := h.campaigns.ByID(r.Context(), tpl.CampaignID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Campanha não encontrada")
		return
	}
	user := authctx.CurrentUser(r)
	regionID := ""
	if campaign.RegionID != nil {
		regionID = *campaign.RegionID
	}
	allowed := user != nil && (user.Role == model.RoleAdmin ||
		(user.Role == model.RoleSupervisor && user.RegionID != nil && *user.RegionID == regionID) ||
		(user.Role == model.RoleExecutive && campaign.TrafficManagerID != nil && *campaign.TrafficManagerID == user.ID))
	if !allowed {
		WriteError(w, http.StatusForbidden, "Sem permissão para esta campanha")
		return
	}
	lead := &model.Lead{}
	if body.LeadID != "" {
		if l, err := h.leads.ByID(r.Context(), body.LeadID); err == nil {
			lead = l
		}
	}
	message := service.SubstitutePatterns(tpl.Content, lead, campaign, tpl.Key, h.cfg.FrontendURL, h.cfg.APIURL)
	session := h.wa.DefaultSession()
	if regionID != "" {
		if reg, err := h.regions.ByID(r.Context(), regionID); err == nil && reg.WhatsappSession != nil && *reg.WhatsappSession != "" {
			session = *reg.WhatsappSession
		}
	}
	if err := h.wa.SendTextTo(session, to, message); err != nil {
		log.Printf("Erro no envio de teste (%s): %v", tpl.Key, err)
		WriteError(w, http.StatusBadGateway, "Falha no envio: "+err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Mensagem de teste enviada", "session": session})
}
