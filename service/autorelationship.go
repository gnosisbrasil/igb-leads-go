package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"igb-leads-go/model"
	"igb-leads-go/repository"
)

// costPer100Leads mirrors COST_PER_100_LEADS (R$ 5 per block).
const costPer100Leads = 5

// keyFor maps the old Meta template names to message_templates keys.
// primeira_camara seeds lembrete_proxima_aula instead of lembrete_evento,
// hence the reminder fallback below.
var keyFor = map[string]string{
	"gnosis_pedido_confirmacao": "pedido_confirmacao",
	"gnosis_agradecimento":      "confirmacao_presenca",
	"gnosis_voucher_v2":         "envio_voucher",
	"gnosis_lembrete":           "lembrete_evento",
}

var keyFallback = map[string]string{
	"gnosis_lembrete": "lembrete_proxima_aula",
}

// AutoRelationship ports AutoRelationshipService onto the meow API.
// Instead of Meta template messages, it sends the campaign's own
// message_templates (pattern-substituted) as plain text. Stamps advance
// exactly like Node, including when sending is skipped.
type AutoRelationship struct {
	leads     *repository.LeadRepository
	forms     *repository.FormRepository
	campaigns *repository.CampaignRepository
	templates *repository.TemplateRepository
	regions   *repository.RegionRepository
	wa        *WhatsAppClient
	frontend  string
	apiURL    string
}

func NewAutoRelationship(leads *repository.LeadRepository, forms *repository.FormRepository, campaigns *repository.CampaignRepository, templates *repository.TemplateRepository, regions *repository.RegionRepository, wa *WhatsAppClient, frontendURL, apiURL string) *AutoRelationship {
	return &AutoRelationship{leads: leads, forms: forms, campaigns: campaigns, templates: templates, regions: regions, wa: wa, frontend: frontendURL, apiURL: apiURL}
}

// sessionFor resolves the team's meow session, falling back to default.
func (a *AutoRelationship) sessionFor(ctx context.Context, campaign *model.Campaign) string {
	if campaign.RegionID != nil && a.regions != nil {
		if reg, err := a.regions.ByID(ctx, *campaign.RegionID); err == nil && reg.WhatsappSession != nil && *reg.WhatsappSession != "" {
			return *reg.WhatsappSession
		}
	}
	return a.wa.DefaultSession()
}

func (a *AutoRelationship) campaignOf(ctx context.Context, lead *model.Lead) *model.Campaign {
	if lead.FormID == nil {
		return nil
	}
	form, err := a.forms.ByID(ctx, *lead.FormID)
	if err != nil {
		return nil
	}
	campaign, err := a.campaigns.ByID(ctx, form.CampaignID)
	if err != nil {
		return nil
	}
	return campaign
}

func isSameDay(c *model.Campaign, now time.Time) bool {
	if c.EventDate == nil {
		return false
	}
	y1, m1, d1 := now.Date()
	y2, m2, d2 := c.EventDate.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// sendTemplate delivers the DB template mapped from the Meta name.
// Missing template or unconfigured API skips loudly but still lets the
// caller stamp, mirroring Node.
func (a *AutoRelationship) sendTemplate(ctx context.Context, lead *model.Lead, campaign *model.Campaign, metaName string) error {
	if !a.wa.IsConfigured() {
		log.Printf("AutoRelationship: WhatsApp não configurado, pulando %s para lead %s", metaName, lead.ID)
		return nil
	}
	tpl, err := a.templates.ByCampaignAndKey(ctx, campaign.ID, keyFor[metaName])
	if err != nil || !tpl.IsActive {
		if fb, ok := keyFallback[metaName]; ok {
			tpl, err = a.templates.ByCampaignAndKey(ctx, campaign.ID, fb)
		}
	}
	if err != nil || !tpl.IsActive {
		log.Printf("AutoRelationship: template %s ausente/inativo na campanha %s", keyFor[metaName], campaign.ID)
		return nil
	}
	message := SubstitutePatterns(tpl.Content, lead, campaign, tpl.Key, a.frontend, a.apiURL)
	phone := "55" + DigitsOnly(lead.Whatsapp)
	return a.wa.SendTextTo(a.sessionFor(ctx, campaign), phone, message)
}

func (a *AutoRelationship) stamp(ctx context.Context, leadID, column string) {
	_ = a.leads.UpdateFields(ctx, leadID, map[string]any{column: time.Now().UTC()})
}

// HandleNewLead runs after a public lead inscription.
func (a *AutoRelationship) HandleNewLead(ctx context.Context, lead *model.Lead) error {
	campaign := a.campaignOf(ctx, lead)
	if campaign == nil || !campaign.AutoRelationship {
		return nil
	}
	if isSameDay(campaign, time.Now().UTC()) {
		return nil
	}
	if err := a.sendTemplate(ctx, lead, campaign, "gnosis_pedido_confirmacao"); err != nil {
		log.Printf("AutoRelationship.handleNewLead error: %v", err)
		return nil
	}
	a.stamp(ctx, lead.ID, "confirmation_sent_at")
	return nil
}

// HandleConfirmed runs after a lead confirmation.
func (a *AutoRelationship) HandleConfirmed(ctx context.Context, lead *model.Lead) error {
	campaign := a.campaignOf(ctx, lead)
	if campaign == nil || !campaign.AutoRelationship {
		return nil
	}
	if err := a.sendTemplate(ctx, lead, campaign, "gnosis_agradecimento"); err != nil {
		log.Printf("AutoRelationship.handleConfirmed error: %v", err)
		return nil
	}
	if isSameDay(campaign, time.Now().UTC()) {
		_ = a.SendVoucher(ctx, lead, campaign)
	}
	return nil
}

// SendVoucher delivers the voucher text (with QR/checkin URLs) via meow.
func (a *AutoRelationship) SendVoucher(ctx context.Context, lead *model.Lead, campaign *model.Campaign) error {
	if err := a.sendTemplate(ctx, lead, campaign, "gnosis_voucher_v2"); err != nil {
		log.Printf("AutoRelationship.sendVoucher error: %v", err)
		return nil
	}
	a.stamp(ctx, lead.ID, "voucher_sent_at")
	return nil
}

// SendDailyConfirmationFollowUps re-asks yesterday+ leads (10h cron).
func (a *AutoRelationship) SendDailyConfirmationFollowUps(ctx context.Context) error {
	leads, err := a.leads.FollowUpCandidates(ctx, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		log.Printf("AutoRelationship.sendDailyConfirmationFollowUps error: %v", err)
		return nil
	}
	now := time.Now().UTC()
	for i := range leads {
		lead := &leads[i]
		campaign := a.campaignOf(ctx, lead)
		if campaign == nil || !campaign.AutoRelationship {
			continue
		}
		if campaign.EventDate == nil || !campaign.EventDate.After(now) {
			continue
		}
		if err := a.sendTemplate(ctx, lead, campaign, "gnosis_pedido_confirmacao"); err != nil {
			log.Printf("AutoRelationship.sendDailyConfirmationFollowUps error: %v", err)
		}
	}
	return nil
}

func dayRange(offset int, now time.Time) (time.Time, time.Time) {
	y, m, d := now.AddDate(0, 0, offset).Date()
	start := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	return start, start.Add(24*time.Hour - time.Nanosecond)
}

// SendEventReminders notifies confirmed leads of tomorrow's events (8h cron).
func (a *AutoRelationship) SendEventReminders(ctx context.Context) error {
	now := time.Now().UTC()
	from, to := dayRange(1, now)
	campaigns, err := a.campaigns.AutoBetween(ctx, from, to)
	if err != nil {
		log.Printf("AutoRelationship.sendEventReminders error: %v", err)
		return nil
	}
	for i := range campaigns {
		leads, err := a.leads.StampedLeads(ctx, campaigns[i].ID, "reminder_event_sent_at")
		if err != nil {
			log.Printf("AutoRelationship.sendEventReminders error: %v", err)
			continue
		}
		for j := range leads {
			if err := a.sendTemplate(ctx, &leads[j], &campaigns[i], "gnosis_lembrete"); err != nil {
				log.Printf("AutoRelationship.sendEventReminders error: %v", err)
				continue
			}
			a.stamp(ctx, leads[j].ID, "reminder_event_sent_at")
		}
	}
	return nil
}

// SendDailyVouchers delivers vouchers for today's events (7h cron).
func (a *AutoRelationship) SendDailyVouchers(ctx context.Context) error {
	now := time.Now().UTC()
	from, to := dayRange(0, now)
	campaigns, err := a.campaigns.AutoBetween(ctx, from, to)
	if err != nil {
		log.Printf("AutoRelationship.sendDailyVouchers error: %v", err)
		return nil
	}
	for i := range campaigns {
		leads, err := a.leads.StampedLeads(ctx, campaigns[i].ID, "voucher_sent_at")
		if err != nil {
			log.Printf("AutoRelationship.sendDailyVouchers error: %v", err)
			continue
		}
		for j := range leads {
			_ = a.SendVoucher(ctx, &leads[j], &campaigns[i])
		}
	}
	return nil
}

// CalculateCampaignCost computes ceil(leads/100)*5 for the completion.
// A nil cost means "leave auto_relationship_cost untouched".
func (a *AutoRelationship) CalculateCampaignCost(ctx context.Context, campaignID string) (*string, error) {
	n, err := a.leads.CountByCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	blocks := (n + 99) / 100
	cost := fmt.Sprintf("%d", blocks*costPer100Leads)
	return &cost, nil
}
