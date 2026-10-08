package handler

import (
	"net/http"

	"igb-leads-go/config"
	"igb-leads-go/repository"
)

// OGHandler mirrors OGController (link preview + redirect).
type OGHandler struct {
	cfg       *config.Config
	forms     *repository.FormRepository
	campaigns *repository.CampaignRepository
}

func NewOGHandler(cfg *config.Config, forms *repository.FormRepository, campaigns *repository.CampaignRepository) *OGHandler {
	return &OGHandler{cfg: cfg, forms: forms, campaigns: campaigns}
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
