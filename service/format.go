package service

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"igb-leads-go/model"
)

// DigitsOnly strips non-digit runes (whatsapp normalization).
func DigitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SlugifyCampaign mirrors the campaign-create slug: lowercase, accents
// stripped (NFD), non-alphanumerics collapsed to '-', trimmed.
func SlugifyCampaign(title string) string {
	t, _, _ := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), strings.ToLower(title))
	return collapseSlug(t)
}

// SlugifyForm mirrors the form-create slug: like campaigns but WITHOUT
// accent stripping (quirk preserved for parity).
func SlugifyForm(title string) string {
	return collapseSlug(strings.ToLower(title))
}

func collapseSlug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteRune('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// FormatDateBR renders DD/MM/YYYY like toLocaleDateString('pt-BR').
// The process runs in UTC, matching the production containers.
func FormatDateBR(t time.Time) string {
	return t.UTC().Format("02/01/2006")
}

// FormatDateTimeBR renders "DD/MM/YYYY, HH:MM:SS" like toLocaleString('pt-BR').
func FormatDateTimeBR(t time.Time) string {
	return t.UTC().Format("02/01/2006, 15:04:05")
}

// ParseEventDate parses a "YYYY-MM-DD" event_dates item.
func ParseEventDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// MaskWhatsApp formats (XX) XXXX-XXXX / (XX) XXXXX-XXXX.
func MaskWhatsApp(phone string) string {
	if phone == "" {
		return ""
	}
	cleaned := DigitsOnly(phone)
	switch len(cleaned) {
	case 10:
		return "(" + cleaned[:2] + ") " + cleaned[2:6] + "-" + cleaned[6:]
	case 11:
		return "(" + cleaned[:2] + ") " + cleaned[2:7] + "-" + cleaned[7:]
	default:
		return phone
	}
}

// EncodeURIComponent mirrors JavaScript's encodeURIComponent.
func EncodeURIComponent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 128 && (r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' ||
			r == '-' || r == '_' || r == '.' || r == '!' || r == '~' || r == '*' || r == '\'' || r == '(' || r == ')') {
			b.WriteRune(r)
			continue
		}
		for _, bb := range []byte(string(r)) {
			const hex = "0123456789ABCDEF"
			b.WriteByte('%')
			b.WriteByte(hex[bb>>4])
			b.WriteByte(hex[bb&0x0F])
		}
	}
	return b.String()
}

// FunnelStep mirrors Lead.getFunnelStatus().
type FunnelStep struct {
	Step  string `json:"step"`
	Label string `json:"label"`
	Key   string `json:"key"`
}

// FunnelStatusOf returns the lead's funnel step.
func FunnelStatusOf(lead *model.Lead) FunnelStep {
	switch {
	case lead.CancelledAt != nil:
		return FunnelStep{Step: "cancelled", Label: "Cancelado", Key: "x"}
	case lead.AttendedAt != nil:
		return FunnelStep{Step: "attended", Label: "Compareceu", Key: "a"}
	case lead.ReminderEventSentAt != nil:
		return FunnelStep{Step: "reminder_event_sent", Label: "Lembrete Enviado", Key: "re"}
	case lead.MotivationSentAt != nil:
		return FunnelStep{Step: "motivation_sent", Label: "Motivação Enviada", Key: "me"}
	case lead.VoucherSentAt != nil:
		return FunnelStep{Step: "voucher_sent", Label: "Voucher Enviado", Key: "ve"}
	case lead.ConfirmedAt != nil:
		return FunnelStep{Step: "confirmed", Label: "Confirmado", Key: "c"}
	case lead.ReminderSentAt != nil:
		return FunnelStep{Step: "reminder_sent", Label: "Lembrete Conf. Enviado", Key: "rpce"}
	case lead.ConfirmationSentAt != nil:
		return FunnelStep{Step: "confirmation_sent", Label: "Pedido Conf. Enviado", Key: "pce"}
	default:
		return FunnelStep{Step: "new", Label: "Inscrito sem Pedido", Key: "i"}
	}
}

// MarkField maps a mark action to its column and optional status.
type MarkField struct {
	Column string
	Status string
}

// MarkFieldMap mirrors MARK_FIELD_MAP, plus qr-sent which the Node map
// lacks (its mark-qr-sent route always 400s there).
var MarkFieldMap = map[string]MarkField{
	"confirmation-sent":   {Column: "confirmation_sent_at"},
	"reminder-sent":       {Column: "reminder_sent_at"},
	"confirmed":           {Column: "confirmed_at", Status: "confirmed"},
	"voucher-sent":        {Column: "voucher_sent_at"},
	"motivation-sent":     {Column: "motivation_sent_at"},
	"reminder-event-sent": {Column: "reminder_event_sent_at"},
	"attended":            {Column: "attended_at", Status: "present"},
	"cancelled":           {Column: "cancelled_at", Status: "lost"},
	"qr-sent":             {Column: "qr_sent_at"},
}

// RevertRule mirrors REVERT_MAP.
type RevertRule struct {
	Clear  []string
	Status string
}

// RevertMap mirrors REVERT_MAP.
var RevertMap = map[string]RevertRule{
	"attended":            {Clear: []string{"attended_at", "voucher_sent_at"}, Status: "confirmed"},
	"voucher_sent":        {Clear: []string{"voucher_sent_at"}, Status: "confirmed"},
	"confirmed":           {Clear: []string{"confirmed_at", "reminder_event_sent_at"}, Status: "new"},
	"confirmation_sent":   {Clear: []string{"confirmation_sent_at"}, Status: "new"},
	"reminder_sent":       {Clear: []string{"reminder_sent_at"}, Status: "new"},
	"motivation_sent":     {Clear: []string{"motivation_sent_at"}, Status: "new"},
	"reminder_event_sent": {Clear: []string{"reminder_event_sent_at"}, Status: "confirmed"},
	"cancelled":           {Clear: []string{"cancelled_at"}, Status: "new"},
}

// ConversionRate mirrors parseFloat(((converted/total)*100).toFixed(2)),
// with 0 when there are no leads (admin/supervisor reports).
func ConversionRate(converted, total int) float64 {
	if total <= 0 {
		return 0
	}
	f, _ := strconv.ParseFloat(strconv.FormatFloat(float64(converted)/float64(total)*100, 'f', 2, 64), 64)
	return f
}

// ConversionRateString mirrors ((converted/total)*100).toFixed(2) as a
// string, with numeric 0 when there are no leads (executive report).
func ConversionRateString(converted, total int) any {
	if total <= 0 {
		return 0
	}
	return strconv.FormatFloat(float64(converted)/float64(total)*100, 'f', 2, 64)
}
