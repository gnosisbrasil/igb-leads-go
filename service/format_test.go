package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"igb-leads-go/model"
)

func TestSlugifyCampaign(t *testing.T) {
	got := SlugifyCampaign("Palestra Pública São Paulo!")
	if got != "palestra-publica-sao-paulo" {
		t.Fatalf("got %q", got)
	}
}

func TestSlugifyFormKeepsQuirk(t *testing.T) {
	// Node form-create does NOT strip accents (verified against Node).
	got := SlugifyForm("Palestra Pública São Paulo")
	if got != "palestra-p-blica-s-o-paulo" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskWhatsApp(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"11988887777":     "(11) 98888-7777",
		"1133334444":      "(11) 3333-4444",
		"(11) 98888-7777": "(11) 98888-7777",
		"12345":           "12345",
	}
	for in, want := range cases {
		if got := MaskWhatsApp(in); got != want {
			t.Fatalf("MaskWhatsApp(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEncodeURIComponent(t *testing.T) {
	// Vectors verified against Node's encodeURIComponent.
	if got := EncodeURIComponent("Olá mundo! ✅ Tudo bem?"); got != "Ol%C3%A1%20mundo!%20%E2%9C%85%20Tudo%20bem%3F" {
		t.Fatalf("got %q", got)
	}
	if got := EncodeURIComponent("a+b=c&d/e"); got != "a%2Bb%3Dc%26d%2Fe" {
		t.Fatalf("got %q", got)
	}
}

func TestFunnelStatusPriority(t *testing.T) {
	now := time.Now()
	lead := &model.Lead{ConfirmationSentAt: &now, ConfirmedAt: &now, VoucherSentAt: &now}
	if got := FunnelStatusOf(lead); got.Step != "voucher_sent" || got.Key != "ve" {
		t.Fatalf("got %+v", got)
	}
	lead.ReminderEventSentAt = &now
	if got := FunnelStatusOf(lead); got.Step != "reminder_event_sent" {
		t.Fatalf("got %+v", got)
	}
	lead.CancelledAt = &now
	if got := FunnelStatusOf(lead); got.Step != "cancelled" {
		t.Fatalf("got %+v", got)
	}
	if got := FunnelStatusOf(&model.Lead{}); got.Step != "new" {
		t.Fatalf("got %+v", got)
	}
}

func TestCorruptedEmojis(t *testing.T) {
	if !HasCorruptedEmojis("olá \uFFFD mundo") {
		t.Fatal("U+FFFD deve detectar")
	}
	if !HasCorruptedEmojis("abc\xffdef") {
		t.Fatal("UTF-8 inválido deve detectar")
	}
	if HasCorruptedEmojis("Olá ✅ mundo 🎫") {
		t.Fatal("emojis válidos não devem detectar")
	}
	if got := CleanCorruptedEmojis("a\uFFFDb\xffc"); got != "abc" {
		t.Fatalf("got %q", got)
	}
}

func TestSubstitutePatterns(t *testing.T) {
	lead := &model.Lead{FirstName: "Maria", LastName: "Silva", Email: "m@x.co", Whatsapp: "11988887777", CheckinCode: "ABC123"}
	city, addr := "Campinas", "Rua X"
	campaign := &model.Campaign{Title: "Palestra", Objectives: strp("camara_publica"), AddressCity: &city, Address: &addr}
	got := SubstitutePatterns("{{saudacao}} {{inscrito_nome}} {{inscrito_nome_completo}} em {{tipo_evento}} {{titulo_campanha}} @ {{cidade}} {{endereco_completo}} {{url_voucher}} {{codigo_checkin}}", lead, campaign, "", "https://front", "https://api")
	want := "Olá MARIA MARIA SILVA em Câmara Pública Palestra @ Campinas Rua X, Campinas https://api/api/voucher/ABC123 ABC123"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestSubstituteRecoversCorrupted(t *testing.T) {
	campaign := &model.Campaign{Title: "T", Objectives: strp("camara_publica")}
	got := SubstitutePatterns("lixo \uFFFD", &model.Lead{FirstName: "A"}, campaign, "pedido_confirmacao", "", "")
	if HasCorruptedEmojis(got) {
		t.Fatal("deveria recuperar do padrão")
	}
	if !strings.Contains(got, "Recebemos sua inscrição") {
		t.Fatalf("got %q", got)
	}
}

func strp(s string) *string { return &s }

func TestConversionRate(t *testing.T) {
	// parseFloat(((c/t)*100).toFixed(2)): numeric, 0 when empty.
	if got := ConversionRate(0, 0); got != 0 {
		t.Fatalf("empty: got %v", got)
	}
	if got := ConversionRate(1, 2); got != 50 {
		t.Fatalf("1/2: got %v", got)
	}
	if got := ConversionRate(1, 3); got != 33.33 {
		t.Fatalf("1/3: got %v", got)
	}
	if got := ConversionRate(2, 3); got != 66.67 {
		t.Fatalf("2/3: got %v", got)
	}
}

func TestConversionRateString(t *testing.T) {
	// Executive report: string with 2 decimals, numeric 0 when empty.
	if got := ConversionRateString(0, 0); got != 0 {
		t.Fatalf("empty: got %#v", got)
	}
	if got := ConversionRateString(1, 2); got != "50.00" {
		t.Fatalf("1/2: got %#v", got)
	}
	if got := ConversionRateString(1, 3); got != "33.33" {
		t.Fatalf("1/3: got %#v", got)
	}
}

func TestSubstituteTurmaDates(t *testing.T) {
	turmas := json.RawMessage(`[{"title":"Turma 1","event_date":"2026-05-25","event_time":"20:00","weekdays":[{"day":"seg","time":"20:00"},{"day":"qua","time":"20:30"}]},{"title":"Turma 2","event_date":"2026-05-26","event_time":"19:00","weekdays":[]}]`)
	campaign := &model.Campaign{Title: "Curso", Objectives: strp("primeira_camara"), Turmas: turmas}
	lead := &model.Lead{FirstName: "A", LastName: "B", Metadata: json.RawMessage(`{"turma":"Turma 1"}`)}
	got := SubstitutePatterns("{{turma}} | {{data_evento}} | {{hora_evento}} | {{dias_semana}}", lead, campaign, "", "", "")
	want := "Turma 1 | 25/05/2026 | 20:00 | Segunda às 20:00, Quarta às 20:30"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestSubstituteTurmaSemMatchListaTodas(t *testing.T) {
	turmas := json.RawMessage(`[{"title":"Turma 1","event_date":"2026-05-25","event_time":"20:00"},{"title":"Turma 2","event_date":"2026-05-26","event_time":"19:00"}]`)
	campaign := &model.Campaign{Title: "Curso", Objectives: strp("primeira_camara"), Turmas: turmas}
	got := SubstitutePatterns("{{turma}} | {{data_evento}} | {{hora_evento}}", &model.Lead{FirstName: "A"}, campaign, "", "", "")
	want := "Turma 1, Turma 2 | Turma 1: 25/05/2026, Turma 2: 26/05/2026 | 20:00, 19:00"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestSubstituteDatasGeraisTemPrioridade(t *testing.T) {
	turmas := json.RawMessage(`[{"title":"Turma 1","event_date":"2026-05-25","event_time":"20:00"}]`)
	campaign := &model.Campaign{
		Title: "Curso", Objectives: strp("primeira_camara"), Turmas: turmas,
		EventDates: json.RawMessage(`[{"date":"2026-06-01","time":"10:00"}]`), EventTime: strp("10:00"),
	}
	got := SubstitutePatterns("{{data_evento}} | {{hora_evento}}", &model.Lead{FirstName: "A"}, campaign, "", "", "")
	want := "01/06/2026 às 10:00 | 10:00"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
