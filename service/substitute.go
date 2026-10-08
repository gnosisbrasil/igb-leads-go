package service

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"igb-leads-go/model"
)

// DefaultsFor returns the seed templates for the objectives,
// falling back to camara_publica like Node.
func DefaultsFor(objectives string) []DefaultTemplate {
	if t, ok := DefaultTemplates[objectives]; ok {
		return t
	}
	return DefaultTemplates["camara_publica"]
}

// HasCorruptedEmojis detects U+FFFD or invalid UTF-8 (Go's counterpart
// of Node's orphan-surrogate checks).
func HasCorruptedEmojis(s string) bool {
	if s == "" {
		return false
	}
	if strings.ContainsRune(s, '�') {
		return true
	}
	return !utf8.ValidString(s)
}

// CleanCorruptedEmojis strips U+FFFD and invalid bytes.
func CleanCorruptedEmojis(s string) string {
	if s == "" {
		return s
	}
	return strings.ReplaceAll(strings.ToValidUTF8(s, ""), "�", "")
}

func strptr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

var tipoEventoLabels = map[string]string{
	"camara_publica":  "Câmara Pública",
	"workshop":        "Workshop",
	"primeira_camara": "Curso de Gnosis",
	"leads_whatsapp":  "Grupo WhatsApp",
}

var dayNames = map[string]string{
	"seg": "Segunda", "ter": "Terça", "qua": "Quarta", "qui": "Quinta",
	"sex": "Sexta", "sab": "Sábado", "dom": "Domingo",
}

func fullAddress(c *model.Campaign) string {
	parts := []string{}
	if strptr(c.Address) != "" {
		parts = append(parts, strptr(c.Address))
	}
	if strptr(c.AddressNumber) != "" {
		parts = append(parts, "nº "+strptr(c.AddressNumber))
	}
	if strptr(c.AddressNeighborhood) != "" {
		parts = append(parts, strptr(c.AddressNeighborhood))
	}
	if strptr(c.AddressCity) != "" {
		parts = append(parts, strptr(c.AddressCity))
	}
	if strptr(c.AddressState) != "" {
		parts = append(parts, strptr(c.AddressState))
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	if strptr(c.Location) != "" {
		return strptr(c.Location)
	}
	return strptr(c.Address)
}

func eventDatesText(c *model.Campaign) string {
	if len(c.EventDates) > 0 {
		var items []struct {
			Date string `json:"date"`
			Time string `json:"time"`
		}
		if err := json.Unmarshal(c.EventDates, &items); err == nil && len(items) > 0 {
			parts := []string{}
			for _, ed := range items {
				if ed.Date == "" {
					continue
				}
				t, err := ParseEventDate(ed.Date)
				if err != nil {
					continue
				}
				if ed.Time != "" {
					parts = append(parts, FormatDateBR(t)+" às "+ed.Time)
				} else {
					parts = append(parts, FormatDateBR(t))
				}
			}
			return strings.Join(parts, ", ")
		}
	}
	if c.EventDate != nil {
		return FormatDateBR(*c.EventDate)
	}
	if c.StartDate != nil {
		s := FormatDateBR(*c.StartDate)
		if c.EndDate != nil {
			s += " a " + FormatDateBR(*c.EndDate)
		}
		return s
	}
	return ""
}

func weekdaysText(c *model.Campaign) string {
	return weekdaysRawText(c.Weekdays)
}

func weekdaysRawText(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var raw []any
	if err := json.Unmarshal(data, &raw); err != nil || len(raw) == 0 {
		return ""
	}
	dayName := func(d string) string {
		if n, ok := dayNames[d]; ok {
			return n
		}
		return d
	}
	if _, isObj := raw[0].(map[string]any); isObj {
		type wd struct {
			Day  string
			Time string
		}
		items := []wd{}
		for _, r := range raw {
			m, _ := r.(map[string]any)
			if m == nil {
				continue
			}
			d, _ := m["day"].(string)
			t, _ := m["time"].(string)
			items = append(items, wd{Day: d, Time: t})
		}
		if len(items) == 0 {
			return ""
		}
		sameTime := true
		for _, it := range items {
			if it.Time != items[0].Time {
				sameTime = false
				break
			}
		}
		if sameTime && items[0].Time != "" {
			days := []string{}
			for _, it := range items {
				days = append(days, dayName(it.Day))
			}
			return strings.Join(days, ", ") + " às " + items[0].Time
		}
		parts := []string{}
		for _, it := range items {
			if it.Time != "" {
				parts = append(parts, dayName(it.Day)+" às "+it.Time)
			} else {
				parts = append(parts, dayName(it.Day))
			}
		}
		return strings.Join(parts, ", ")
	}
	days := []string{}
	for _, r := range raw {
		if d, ok := r.(string); ok {
			days = append(days, dayName(d))
		}
	}
	return strings.Join(days, ", ")
}

type turmaSchedule struct {
	Title     string          `json:"title"`
	EventDate string          `json:"event_date"`
	EventTime string          `json:"event_time"`
	Weekdays  json.RawMessage `json:"weekdays"`
}

func parseTurmas(data json.RawMessage) []turmaSchedule {
	if len(data) == 0 {
		return nil
	}
	var out []turmaSchedule
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	kept := out[:0]
	for _, t := range out {
		if strings.TrimSpace(t.Title) != "" {
			kept = append(kept, t)
		}
	}
	return kept
}

func leadTurmaName(lead *model.Lead) string {
	if lead == nil || len(lead.Metadata) == 0 {
		return ""
	}
	var meta struct {
		Turma string `json:"turma"`
	}
	if err := json.Unmarshal(lead.Metadata, &meta); err != nil {
		return ""
	}
	return strings.TrimSpace(meta.Turma)
}

func turmaDateBR(date string) string {
	if date == "" {
		return ""
	}
	if t, err := ParseEventDate(date); err == nil {
		return FormatDateBR(t)
	}
	return date
}

// turmaTexts resolves date/time/name/weekdays from the campaign turmas.
// The lead's own turma wins; a single turma is used directly; with
// several turmas and no match every option is listed so the message
// never goes out with blank date/time.
func turmaTexts(campaign *model.Campaign, lead *model.Lead) (date, time, name, weekdays string) {
	turmas := parseTurmas(campaign.Turmas)
	if len(turmas) == 0 {
		return "", "", "", ""
	}
	want := strings.ToLower(leadTurmaName(lead))
	if want != "" {
		for _, t := range turmas {
			if strings.ToLower(strings.TrimSpace(t.Title)) == want {
				return turmaDateBR(t.EventDate), strings.TrimSpace(t.EventTime), strings.TrimSpace(t.Title), weekdaysRawText(t.Weekdays)
			}
		}
	}
	if len(turmas) == 1 {
		t := turmas[0]
		return turmaDateBR(t.EventDate), strings.TrimSpace(t.EventTime), strings.TrimSpace(t.Title), weekdaysRawText(t.Weekdays)
	}
	dates, times, names := []string{}, []string{}, []string{}
	seenTime := map[string]bool{}
	for _, t := range turmas {
		title := strings.TrimSpace(t.Title)
		names = append(names, title)
		if d := turmaDateBR(t.EventDate); d != "" {
			dates = append(dates, title+": "+d)
		}
		if tm := strings.TrimSpace(t.EventTime); tm != "" && !seenTime[tm] {
			seenTime[tm] = true
			times = append(times, tm)
		}
	}
	return strings.Join(dates, ", "), strings.Join(times, ", "), strings.Join(names, ", "), ""
}

// SubstitutePatterns mirrors MessageTemplateController.substitutePatterns.
func SubstitutePatterns(content string, lead *model.Lead, campaign *model.Campaign, templateKey, frontendURL, apiURL string) string {
	if content == "" {
		return ""
	}
	if HasCorruptedEmojis(content) && campaign != nil {
		objectives := strptr(campaign.Objectives)
		if objectives == "" {
			objectives = "camara_publica"
		}
		match := ""
		for _, t := range DefaultsFor(objectives) {
			if templateKey != "" && t.Key == templateKey {
				match = t.Content
				break
			}
		}
		if match != "" {
			content = match
		} else {
			content = CleanCorruptedEmojis(content)
		}
	}

	values := map[string]string{
		"{{nome_instituicao}}": "Gnosis Brasil",
		"{{saudacao}}":         "Olá",
	}
	if lead != nil {
		values["{{inscrito_nome}}"] = strings.ToUpper(lead.FirstName)
		values["{{inscrito_sobrenome}}"] = strings.ToUpper(lead.LastName)
		values["{{inscrito_nome_completo}}"] = strings.ToUpper(lead.FirstName) + " " + strings.ToUpper(lead.LastName)
		values["{{codigo_checkin}}"] = lead.CheckinCode
		values["{{email}}"] = lead.Email
		values["{{whatsapp}}"] = lead.Whatsapp
		if lead.CheckinCode != "" {
			values["{{url_voucher}}"] = strings.TrimSuffix(frontendURL, "/") + "/checkin/" + lead.CheckinCode
			values["{{url_qrcode_imagem}}"] = strings.TrimSuffix(apiURL, "/") + "/api/qr/" + lead.CheckinCode
		} else {
			values["{{url_voucher}}"] = ""
			values["{{url_qrcode_imagem}}"] = ""
		}
	}
	if campaign != nil {
		objectives := strptr(campaign.Objectives)
		tipo, ok := tipoEventoLabels[objectives]
		if !ok {
			tipo = objectives
		}
		values["{{tipo_evento}}"] = tipo
		values["{{titulo_campanha}}"] = campaign.Title
		values["{{cidade}}"] = strptr(campaign.AddressCity)
		if strptr(campaign.Location) != "" {
			values["{{local}}"] = strptr(campaign.Location)
		} else {
			values["{{local}}"] = strptr(campaign.Address)
		}
		values["{{numero}}"] = strptr(campaign.AddressNumber)
		values["{{bairro}}"] = strptr(campaign.AddressNeighborhood)
		values["{{endereco_completo}}"] = fullAddress(campaign)
		values["{{google_maps_link}}"] = strptr(campaign.GoogleMapsLink)
		dateText, timeText, turmaName, turmaWeekdays := turmaTexts(campaign, lead)
		values["{{turma}}"] = turmaName
		values["{{data_evento}}"] = firstNonEmpty(eventDatesText(campaign), dateText)
		values["{{hora_evento}}"] = firstNonEmpty(strptr(campaign.EventTime), timeText)
		values["{{dias_semana}}"] = firstNonEmpty(weekdaysText(campaign), turmaWeekdays)
	}
	// Patterns referencing a nil side resolve to empty, like Node's
	// `value || ''` on undefined.
	for pattern, value := range values {
		content = strings.ReplaceAll(content, pattern, value)
	}
	return content
}
