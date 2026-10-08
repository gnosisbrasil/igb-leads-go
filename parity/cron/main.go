package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"igb-leads-go/config"
	"igb-leads-go/jobs"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

var failures int

func check(name string, cond bool) {
	if cond {
		fmt.Println("ok:", name)
	} else {
		failures++
		fmt.Println("FALHOU:", name)
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	pool, err := repository.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	leads := repository.NewLeadRepository(pool)
	forms := repository.NewFormRepository(pool)
	campaigns := repository.NewCampaignRepository(pool)
	templates := repository.NewTemplateRepository(pool)
	users := repository.NewUserRepository(pool)
	schedules := repository.NewScheduleRepository(pool)
	wa := service.NewWhatsAppClient(cfg.MeowURL, cfg.MeowAPIKey, cfg.MeowSession)
	emailSvc := service.NewEmailService(cfg)
	regions := repository.NewRegionRepository(pool)
	auto := service.NewAutoRelationship(leads, forms, campaigns, templates, regions, wa, cfg.FrontendURL, cfg.APIURL)
	sched := jobs.NewScheduler(schedules, campaigns, forms, leads, users, emailSvc, auto)

	meowCalls := func() int {
		b, _ := os.ReadFile("/tmp/mock-meow.log")
		n := 0
		for _, l := range splitLines(string(b)) {
			if l != "" {
				n++
			}
		}
		return n
	}

	// 1. reminders (evento amanhã)
	before := meowCalls()
	_ = auto.SendEventReminders(ctx)
	check("reminder carimbou", scalar(pool, "SELECT CASE WHEN reminder_event_sent_at IS NOT NULL THEN 't' ELSE 'f' END FROM leads WHERE email='rem@teste.com'") == "t")
	check("reminder enviou meow", meowCalls() == before+1)

	// 2. vouchers (evento hoje)
	before = meowCalls()
	_ = auto.SendDailyVouchers(ctx)
	check("voucher carimbou", scalar(pool, "SELECT CASE WHEN voucher_sent_at IS NOT NULL THEN 't' ELSE 'f' END FROM leads WHERE email='vouch@teste.com'") == "t")
	check("voucher enviou meow", meowCalls() == before+1)

	// 3. follow-ups (pediu ontem, evento futuro)
	before = meowCalls()
	_ = auto.SendDailyConfirmationFollowUps(ctx)
	check("followup reenviou meow", meowCalls() == before+1)

	// 4. custo: 3 leads -> R$ 5
	cost, err := auto.CalculateCampaignCost(ctx, os.Getenv("CID_COST"))
	check("custo 3 leads = 5", err == nil && cost != nil && *cost == "5")

	// 5. scheduler de email: pendente vencido -> enviado via SMTP mock
	sched.ProcessScheduledEmails()
	check("schedule enviado", scalar(pool, "SELECT status FROM email_schedules WHERE id='"+os.Getenv("SCHED_ID")+"'") == "sent")
	smtp, _ := os.ReadFile("/tmp/mock-smtp.log")
	check("smtp recebeu", len(smtp) > 100)

	// 6. agendamento: evento em 3d cria event_reminder_3d devido já (fix do
	// bug Node, onde a guarda scheduled>now impedia 7d/3d/1d para sempre)
	c3 := "c0000000-0000-0000-0000-000000000005"
	sched.ScheduleReminderEmails()
	check("agendou 3d", scalar(pool, "SELECT count(*) FROM email_schedules WHERE campaign_id='"+c3+"' AND email_type='event_reminder_3d'") == "1")
	check("agendou devido", scalar(pool, "SELECT count(*) FROM email_schedules WHERE campaign_id='"+c3+"' AND email_type='event_reminder_3d' AND scheduled_for <= now()") == "1")
	sched.ProcessScheduledEmails()
	check("3d enviado", scalar(pool, "SELECT status FROM email_schedules WHERE campaign_id='"+c3+"' AND email_type='event_reminder_3d'") == "sent")
	var n0 int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM email_schedules").Scan(&n0)
	sched.ScheduleReminderEmails()
	var n1 int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM email_schedules").Scan(&n1)
	check("sem reagendar (sem spam)", n0 == n1)

	if failures > 0 {
		os.Exit(1)
	}
	fmt.Println("CRON_OK")
}

func scalar(pool *pgxpool.Pool, q string) string {
	var s string
	_ = pool.QueryRow(context.Background(), q).Scan(&s)
	return s
}

func splitLines(s string) []string { return strings.Split(s, "\n") }
