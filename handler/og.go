package handler

import (
	"html"
	"net/http"
	"strings"

	"igb-leads-go/config"
	"igb-leads-go/repository"
)

// OGHandler mirrors OGController (link preview + redirect).
type OGHandler struct {
	cfg       *config.Config
	forms     *repository.FormRepository
	campaigns *repository.CampaignRepository
	leads     *repository.LeadRepository
}

func NewOGHandler(cfg *config.Config, forms *repository.FormRepository, campaigns *repository.CampaignRepository, leads *repository.LeadRepository) *OGHandler {
	return &OGHandler{cfg: cfg, forms: forms, campaigns: campaigns, leads: leads}
}

var objectiveLabels = map[string]string{
	"camara_publica":  "Palestra Pública",
	"primeira_camara": "Curso de Gnosis",
	"workshop":        "Workshop",
	"leads_whatsapp":  "Grupo WhatsApp",
}

func (h *OGHandler) Render(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	f, err := h.forms.ByPublicToken(r.Context(), token)
	if err != nil {
		f, err = h.forms.ByShortCode(r.Context(), token)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Formulário não encontrado</title></head><body><h1>Link inválido</h1></body></html>`))
			return
		}
	}
	var title, desc string
	var objectives string
	if c, err := h.campaigns.ByID(r.Context(), f.CampaignID); err == nil {
		objectives = strOrEmpty(c.Objectives)
		title = c.Title
		if c.Description != nil {
			desc = *c.Description
		}
	}
	if title == "" {
		title = f.Title
	}
	objective := objectiveLabels[objectives]
	if objective == "" {
		objective = "Evento"
	}
	title = title + " - " + objective
	if desc == "" {
		desc = "Evento gratuito. Inscreva-se!"
	}
	img := "https://leads.gnosisbrasil.com/logo.png"
	base := h.cfg.FrontendURL
	if base == "" {
		base = "https://leads.gnosisbrasil.com"
	}
	formURL := base + "/form/" + f.PublicToken
	html := `<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>` + title + `</title>
  <meta name="description" content="` + desc + `">
  <meta property="og:type" content="website">
  <meta property="og:url" content="` + formURL + `">
  <meta property="og:title" content="` + title + `">
  <meta property="og:description" content="` + desc + `">
  <meta property="og:image" content="` + img + `">
  <meta property="og:image:width" content="200">
  <meta property="og:image:height" content="200">
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:title" content="` + title + `">
  <meta name="twitter:description" content="` + desc + `">
  <meta name="twitter:image" content="` + img + `">
  <meta http-equiv="refresh" content="0;url=` + formURL + `">
</head>
<body>
  <script>window.location.href="` + formURL + `";</script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

// buildVoucherHTML renders the crawler page for a voucher link: og tags
// with the QR as image, then an instant redirect for humans.
func buildVoucherHTML(title, desc, voucherURL, qrURL string) string {
	title, desc = html.EscapeString(title), html.EscapeString(desc)
	return `<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>` + title + `</title>
  <meta name="description" content="` + desc + `">
  <meta property="og:type" content="website">
  <meta property="og:url" content="` + voucherURL + `">
  <meta property="og:title" content="` + title + `">
  <meta property="og:description" content="` + desc + `">
  <meta property="og:image" content="` + qrURL + `">
  <meta property="og:image:width" content="600">
  <meta property="og:image:height" content="600">
  <meta http-equiv="refresh" content="0;url=` + voucherURL + `">
</head>
<body>
  <script>window.location.href="` + voucherURL + `";</script>
</body>
</html>`
}

// Voucher serves /api/voucher/{code}: WhatsApp/TG crawlers get og tags
// with the scannable QR, humans bounce to the frontend voucher page.
func (h *OGHandler) Voucher(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	lead, err := h.leads.ByCheckinCode(r.Context(), code)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Voucher não encontrado</title></head><body><h1>Link inválido</h1></body></html>`))
		return
	}
	title := "Seu voucher"
	if lead.FormID != nil {
		if f, ferr := h.forms.ByID(r.Context(), *lead.FormID); ferr == nil {
			if c, cerr := h.campaigns.ByID(r.Context(), f.CampaignID); cerr == nil {
				title = "Seu voucher - " + c.Title
			}
		}
	}
	base := strings.TrimSuffix(h.cfg.FrontendURL, "/")
	if base == "" {
		base = "https://leads.gnosisbrasil.com"
	}
	api := strings.TrimSuffix(h.cfg.APIURL, "/")
	voucherURL := base + "/checkin/" + lead.CheckinCode
	qrURL := api + "/api/qr/" + lead.CheckinCode + "?size=600"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(buildVoucherHTML(title, "Apresente este QR Code na entrada do evento.", voucherURL, qrURL)))
}
