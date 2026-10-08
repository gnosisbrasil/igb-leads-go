// Package jobs holds the cron scheduler: email processing plus the
// AutoRelationship routines. Schedules mirror app.js (server-local,
// UTC in production).
package jobs

import (
	"context"
	"log"
	"math"
	"time"

	"github.com/robfig/cron/v3"

	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// Scheduler wires every recurring routine.
type Scheduler struct {
	cron      *cron.Cron
	schedules *repository.ScheduleRepository
	campaigns *repository.CampaignRepository
	forms     *repository.FormRepository
	leads     *repository.LeadRepository
	users     *repository.UserRepository
	email     *service.EmailService
	auto      *service.AutoRelationship
}

func NewScheduler(schedules *repository.ScheduleRepository, campaigns *repository.CampaignRepository, forms *repository.FormRepository, leads *repository.LeadRepository, users *repository.UserRepository, email *service.EmailService, auto *service.AutoRelationship) *Scheduler {
	return &Scheduler{
		cron: cron.New(), schedules: schedules, campaigns: campaigns,
		forms: forms, leads: leads, users: users, email: email, auto: auto,
	}
}

// Start registers the routines. Every handler swallows its errors after
// logging, like the Node crons.
func (s *Scheduler) Start() {
	log.Print("📅 Iniciando email scheduler...")
	must := func(spec string, job func()) {
		if _, err := s.cron.AddFunc(spec, job); err != nil {
			log.Fatalf("cron %q: %v", spec, err)
		}
	}
	must("0 * * * *", func() { s.ProcessScheduledEmails() })
	must("0 */6 * * *", func() { s.ScheduleReminderEmails() })
	must("0 10 * * *", func() {
		log.Print("⏰ Executando follow-ups diários de confirmação...")
		_ = s.auto.SendDailyConfirmationFollowUps(context.Background())
	})
	must("0 8 * * *", func() {
		log.Print("⏰ Executando lembretes de eventos...")
		_ = s.auto.SendEventReminders(context.Background())
	})
	must("0 7 * * *", func() {
		log.Print("⏰ Executando envio diário de vouchers...")
		_ = s.auto.SendDailyVouchers(context.Background())
	})
	s.cron.Start()
	log.Print("✅ Email scheduler iniciado")
}

// Stop halts the scheduler, waiting for running jobs.
func (s *Scheduler) Stop() context.Context {
	return s.cron.Stop()
}

// ProcessScheduledEmails sends due pending emails (hourly).
func (s *Scheduler) ProcessScheduledEmails() {
	ctx := context.Background()
	due, err := s.schedules.DuePending(ctx)
	if err != nil {
		log.Printf("Erro ao processar emails agendados: %v", err)
		return
	}
	for i := range due {
		if err := s.sendScheduledEmail(ctx, &due[i]); err != nil {
			log.Printf("Erro ao enviar email: %v", err)
			_ = s.schedules.MarkFailed(ctx, due[i].ID, err.Error())
			continue
		}
		_ = s.schedules.MarkSent(ctx, due[i].ID)
	}
}

func (s *Scheduler) sendScheduledEmail(ctx context.Context, sched *model.EmailSchedule) error {
	campaign, err := s.campaigns.ByID(ctx, sched.CampaignID)
	if err != nil {
		return errNotFound("Campanha ou usuário não encontrado")
	}
	// NOTE: Node joins campaign without the owner, so campaign.user is
	// always missing and every scheduled email fails. The owner lookup
	// below implements the evident intent.
	owner, err := s.users.ByID(ctx, campaign.UserID)
	if err != nil {
		return errNotFound("Campanha ou usuário não encontrado")
	}
	formIDs, err := s.forms.IDsByCampaignIDs(ctx, []string{campaign.ID})
	if err != nil {
		return err
	}
	all, err := s.leads.ByFormIDs(ctx, formIDs)
	if err != nil {
		return err
	}
	leads := make([]model.Lead, 0, len(all))
	for i := range all {
		if all[i].Status == "new" || all[i].Status == "confirmed" {
			leads = append(leads, all[i])
		}
	}
	return s.email.SendVoucherReminderEmail(owner, campaign, leads)
}

type scheduleError string

func (e scheduleError) Error() string { return string(e) }

func errNotFound(msg string) error { return scheduleError(msg) }

// ScheduleReminderEmails queues 7d/3d/1d/day-of reminders (every 6h).
func (s *Scheduler) ScheduleReminderEmails() {
	ctx := context.Background()
	now := time.Now().UTC()
	campaigns, err := s.campaigns.InProgressWithEventBetween(ctx, now, now.Add(7*24*time.Hour))
	if err != nil {
		log.Printf("Erro ao agendar lembretes: %v", err)
		return
	}
	for i := range campaigns {
		s.checkAndScheduleReminders(ctx, &campaigns[i], now)
	}
}

func (s *Scheduler) checkAndScheduleReminders(ctx context.Context, campaign *model.Campaign, now time.Time) {
	if campaign.EventDate == nil {
		return
	}
	daysUntil := int(math.Ceil(campaign.EventDate.Sub(now).Hours() / 24))
	for _, reminder := range []struct {
		days int
		typ  string
	}{
		{7, "event_reminder_7d"},
		{3, "event_reminder_3d"},
		{1, "event_reminder_1d"},
		{0, "event_day"},
	} {
		if daysUntil == reminder.days {
			s.scheduleIfNotExists(ctx, campaign, reminder.typ, now)
		}
	}
}

func (s *Scheduler) scheduleIfNotExists(ctx context.Context, campaign *model.Campaign, emailType string, now time.Time) {
	exists, err := s.schedules.ExistsScheduled(ctx, campaign.ID, emailType)
	if err != nil || exists {
		return
	}
	eventDate := *campaign.EventDate
	var scheduled time.Time
	switch emailType {
	case "event_reminder_7d":
		scheduled = eventDate.Add(-7 * 24 * time.Hour)
	case "event_reminder_3d":
		scheduled = eventDate.Add(-3 * 24 * time.Hour)
	case "event_reminder_1d":
		scheduled = eventDate.Add(-24 * time.Hour)
	case "event_day":
		y, m, d := eventDate.Date()
		scheduled = time.Date(y, m, d, 8, 0, 0, 0, eventDate.Location())
	default:
		return
	}
	// A reminder whose time has come is due now (sent at the next
	// hourly run). The old `scheduled.After(now)` guard could never pass
	// for 7d/3d/1d, so those reminders silently never existed (Node bug).
	if scheduled.Before(now) {
		scheduled = now
	}
	_ = s.schedules.Create(ctx, campaign.ID, emailType, scheduled)
}
