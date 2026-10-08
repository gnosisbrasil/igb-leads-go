// igb-leads-go is a Go port of the sistema-leads backend
// (Gnosis/igb-sistema-leads/apps/backend), keeping the same routes,
// logic and JSON shapes against the existing PostgreSQL schema.
//
// Differences from the Node version, by design:
//   - Pix only via Efí (no credit card, no boleto, no simulation mode)
//   - WhatsApp via the unofficial meow API instead of Meta Cloud API
//   - missing credentials fail loudly at startup or at use time
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"igb-leads-go/authctx"
	"igb-leads-go/config"
	"igb-leads-go/handler"
	"igb-leads-go/jobs"
	"igb-leads-go/middleware"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// chain applies middlewares around h, outermost first.
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func main() {
	// All timestamps serialize in Zulu, exactly like the Node API.
	time.Local = time.UTC
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := repository.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()
	log.Print("conexão com PostgreSQL estabelecida")

	users := repository.NewUserRepository(pool)
	regions := repository.NewRegionRepository(pool)
	notify := repository.NewNotificationRepository(pool)
	campaigns := repository.NewCampaignRepository(pool)
	details := repository.NewDetailRepository(pool)
	forms := repository.NewFormRepository(pool)
	leads := repository.NewLeadRepository(pool)
	templates := repository.NewTemplateRepository(pool)
	logs := repository.NewLogRepository(pool)
	schedules := repository.NewScheduleRepository(pool)
	reports := repository.NewReportRepository(pool)
	uploads := repository.NewUploadRepository(pool)
	tokens := service.NewTokenService(cfg.JWTSecret, cfg.JWTExpiresIn, cfg.JWTRefreshSecret, cfg.JWTRefreshExpiresIn)
	captcha := service.NewCaptchaVerifier(cfg.TurnstileSecret, cfg.NodeEnv)
	email := service.NewEmailService(cfg)
	whatsapp := service.NewWhatsAppClient(cfg.MeowURL, cfg.MeowAPIKey, cfg.MeowSession)
	auto := service.NewAutoRelationship(leads, forms, campaigns, templates, regions, whatsapp, cfg.FrontendURL, cfg.APIURL)
	efi := service.NewEfiClient(service.EfiConfig{
		ClientID: cfg.EfiClientID, ClientSecret: cfg.EfiClientSecret,
		CertBase64: cfg.EfiCertBase64, PixKey: cfg.EfiPixKey,
		Sandbox: cfg.EfiSandbox, BaseURL: cfg.EfiBaseURL,
	})

	authHandler := handler.NewAuthHandler(cfg, users, regions, tokens, captcha, email)
	userHandler := handler.NewUserHandler(cfg, users, regions, notify)
	regionHandler := handler.NewRegionHandler(regions, users)
	campaignHandler := handler.NewCampaignHandler(cfg, campaigns, details, forms, leads, users, regions, templates, notify, logs, auto)
	formHandler := handler.NewFormHandler(cfg, forms, campaigns)
	leadHandler := handler.NewLeadHandler(cfg, leads, forms, campaigns, logs, auto)
	publicHandler := handler.NewPublicHandler(cfg, campaigns, forms, leads, templates, whatsapp)
	ogHandler := handler.NewOGHandler(cfg, forms, campaigns)
	paymentHandler := handler.NewPaymentHandler(cfg, campaigns, users, notify, efi, campaignHandler.AdvanceToWaitingExecutive)
	reportHandler := handler.NewReportHandler(reports, forms, users, regions, logs, campaigns)
	notificationHandler := handler.NewNotificationHandler(notify)
	uploadHandler := handler.NewUploadHandler(cfg, uploads)
	templateHandler := handler.NewTemplateHandler(cfg, templates, leads, forms, campaigns, whatsapp)
	waHandler := handler.NewWhatsAppHandler(cfg, regions, templates, campaigns, leads, whatsapp)

	auth := middleware.Auth(users, tokens)
	role := middleware.RoleCheck
	capt := authHandler.Captcha
	authLimiter := service.NewRateLimiter(15*time.Minute, 10)
	passwordLimiter := service.NewRateLimiter(time.Hour, 5)

	service.SyncTemplates(ctx, campaigns, templates)
	if !efi.IsConfigured() {
		log.Print("⚠️  Efí Pix não configurada: cobranças retornarão erro explícito")
	}
	if !whatsapp.IsConfigured() {
		log.Print("⚠️  WhatsApp (meow) não configurado: envios serão pulados com aviso")
	}
	if cfg.SMTPHost != "" && !cfg.SMTPConfigured() {
		log.Print("⚠️  SMTP incompleto: e-mails serão pulados com aviso")
	}
	scheduler := jobs.NewScheduler(schedules, campaigns, forms, leads, users, email, auto)
	scheduler.Start()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health(pool))
	mux.HandleFunc("GET /api/config", handler.APIConfig(cfg.TurnstileSiteKey))
	mux.HandleFunc("GET /api/qr/{checkin_code}", leadHandler.ServeQRImage)
	mux.HandleFunc("GET /api/og/{token}", ogHandler.Render)

	// Auth (mirrors auth.routes.js).
	mux.Handle("POST /api/auth/register", authLimiter.Middleware("Muitas tentativas. Tente novamente em 15 minutos.",
		capt(http.HandlerFunc(authHandler.Register))))
	mux.Handle("POST /api/auth/login", authLimiter.Middleware("Muitas tentativas. Tente novamente em 15 minutos.",
		capt(http.HandlerFunc(authHandler.Login))))
	mux.Handle("GET /api/auth/me", chain(http.HandlerFunc(authHandler.Me(authctx.UserID)), auth))
	mux.HandleFunc("GET /api/auth/google", authHandler.Google)
	mux.HandleFunc("GET /api/auth/google/callback", authHandler.GoogleCallback)
	mux.Handle("GET /api/auth/connect/google", chain(http.HandlerFunc(authHandler.Connect(authctx.UserID)), auth))
	mux.HandleFunc("GET /api/auth/connect/google/callback", authHandler.ConnectCallback)
	mux.Handle("POST /api/auth/disconnect/google", chain(http.HandlerFunc(authHandler.Disconnect(authctx.UserID)), auth))
	mux.Handle("POST /api/auth/forgot-password", passwordLimiter.Middleware("Muitas solicitações. Tente novamente em 1 hora.",
		http.HandlerFunc(authHandler.ForgotPassword)))
	mux.Handle("POST /api/auth/reset-password", passwordLimiter.Middleware("Muitas solicitações. Tente novamente em 1 hora.",
		http.HandlerFunc(authHandler.ResetPassword)))
	mux.HandleFunc("POST /api/auth/refresh", authHandler.Refresh)

	// Users (mirrors users.routes.js).
	staff := role(model.RoleAdmin, model.RoleSupervisor, model.RoleExecutive)
	approver := role(model.RoleAdmin, model.RoleSupervisor)
	adminOnly := role(model.RoleAdmin)
	mux.Handle("GET /api/users", chain(http.HandlerFunc(userHandler.List), auth, staff))
	mux.Handle("GET /api/users/pending", chain(http.HandlerFunc(userHandler.ListPending), auth, approver))
	mux.Handle("POST /api/users", chain(http.HandlerFunc(userHandler.Create), auth, staff))
	mux.Handle("GET /api/users/{id}", chain(http.HandlerFunc(userHandler.GetByID), auth))
	mux.Handle("PATCH /api/users/{id}/approve", chain(http.HandlerFunc(userHandler.Approve), auth, approver))
	mux.Handle("PATCH /api/users/{id}/suspend", chain(http.HandlerFunc(userHandler.Suspend), auth, adminOnly))
	mux.Handle("PATCH /api/users/{id}/reactivate", chain(http.HandlerFunc(userHandler.Reactivate), auth, adminOnly))
	mux.Handle("PUT /api/users/{id}", chain(http.HandlerFunc(userHandler.Update), auth))
	mux.Handle("DELETE /api/users/{id}", chain(http.HandlerFunc(userHandler.Delete), auth, staff))

	// Regions (mirrors regions.routes.js).
	mux.Handle("GET /api/regions", chain(http.HandlerFunc(regionHandler.List), auth))
	mux.Handle("GET /api/regions/{id}", chain(http.HandlerFunc(regionHandler.GetByID), auth))
	mux.Handle("POST /api/regions", chain(http.HandlerFunc(regionHandler.Create), auth, adminOnly))
	mux.Handle("PUT /api/regions/{id}", chain(http.HandlerFunc(regionHandler.Update), auth, adminOnly))
	mux.Handle("DELETE /api/regions/{id}", chain(http.HandlerFunc(regionHandler.Delete), auth, adminOnly))
	mux.Handle("POST /api/regions/{id}/assign-supervisor", chain(http.HandlerFunc(regionHandler.AssignSupervisor), auth, adminOnly))
	mux.Handle("GET /api/regions/{id}/whatsapp/status", chain(http.HandlerFunc(waHandler.Status), auth))
	mux.Handle("GET /api/regions/{id}/whatsapp/qr", chain(http.HandlerFunc(waHandler.QR), auth))
	mux.Handle("POST /api/regions/{id}/whatsapp/disconnect", chain(http.HandlerFunc(waHandler.Disconnect), auth))
	mux.Handle("POST /api/message-templates/test-send", chain(http.HandlerFunc(waHandler.TestSend), auth))

	// Campaigns (mirrors campaigns.routes.js). Public first.
	mux.HandleFunc("POST /api/campaigns/public/{token}/leads", publicHandler.GetLeads)
	mux.HandleFunc("POST /api/campaigns/public/{token}/leads/{leadId}/{action}", publicHandler.LeadAction)
	mux.HandleFunc("POST /api/campaigns/public/{token}/leads/{leadId}/whatsapp/{templateId}", publicHandler.WhatsAppLink)
	mux.HandleFunc("POST /api/campaigns/public/{token}/checkin", publicHandler.Checkin)
	mux.HandleFunc("POST /api/campaigns/public/{token}/templates", publicHandler.ListTemplates)
	mux.HandleFunc("POST /api/campaigns/public/{token}/templates/create", publicHandler.CreateTemplate)
	mux.HandleFunc("PUT /api/campaigns/public/{token}/templates/{templateId}", publicHandler.UpdateTemplate)
	mux.HandleFunc("DELETE /api/campaigns/public/{token}/templates/{templateId}", publicHandler.DeleteTemplate)
	mux.HandleFunc("POST /api/campaigns/public/{token}/templates/seed-defaults", publicHandler.SeedTemplates)
	mux.Handle("GET /api/campaigns", chain(http.HandlerFunc(campaignHandler.List), auth))
	mux.Handle("GET /api/campaigns/available", chain(http.HandlerFunc(campaignHandler.ListAvailable), auth))
	mux.Handle("GET /api/campaigns/{id}", chain(http.HandlerFunc(campaignHandler.GetByID), auth))
	mux.Handle("GET /api/campaigns/{id}/health", chain(http.HandlerFunc(campaignHandler.Health), auth))
	mux.Handle("GET /api/campaigns/{id}/audit", chain(http.HandlerFunc(campaignHandler.Audit), auth))
	mux.Handle("POST /api/campaigns", chain(http.HandlerFunc(campaignHandler.Create), auth))
	mux.Handle("PUT /api/campaigns/{id}", chain(http.HandlerFunc(campaignHandler.Update), auth))
	mux.Handle("PUT /api/campaigns/{id}/goal", chain(http.HandlerFunc(campaignHandler.Goal), auth))
	mux.Handle("DELETE /api/campaigns/{id}", chain(http.HandlerFunc(campaignHandler.Delete), auth))
	mux.Handle("POST /api/campaigns/{id}/submit", chain(http.HandlerFunc(campaignHandler.Submit), auth))
	mux.Handle("POST /api/campaigns/{id}/mark-paid", chain(http.HandlerFunc(campaignHandler.MarkAsPaid), auth, adminOnly))
	pickup := role(model.RoleExecutive, model.RoleSupervisor, model.RoleAdmin)
	mux.Handle("POST /api/campaigns/{id}/pick-up", chain(http.HandlerFunc(campaignHandler.PickUp), auth, pickup))
	mux.Handle("POST /api/campaigns/{id}/reject", chain(http.HandlerFunc(campaignHandler.Reject), auth, staff))
	mux.Handle("POST /api/campaigns/{id}/assign", chain(http.HandlerFunc(campaignHandler.AssignExecutive), auth, approver))
	mux.Handle("POST /api/campaigns/{id}/start", chain(http.HandlerFunc(campaignHandler.Start), auth))
	mux.Handle("POST /api/campaigns/{id}/complete", chain(http.HandlerFunc(campaignHandler.Complete), auth))
	mux.Handle("POST /api/campaigns/{id}/cancel", chain(http.HandlerFunc(campaignHandler.Cancel), auth))
	mux.Handle("POST /api/campaigns/{id}/to-draft", chain(http.HandlerFunc(campaignHandler.ToDraft), auth))
	mux.Handle("POST /api/campaigns/{id}/propose-edit", chain(http.HandlerFunc(campaignHandler.ProposeEdit), auth))
	mux.Handle("POST /api/campaigns/{id}/approve-edit", chain(http.HandlerFunc(campaignHandler.ApproveEdit), auth))
	mux.Handle("POST /api/campaigns/{id}/reject-edit", chain(http.HandlerFunc(campaignHandler.RejectEdit), auth))
	mux.Handle("POST /api/campaigns/{id}/public-link", chain(http.HandlerFunc(publicHandler.GenerateLink), auth))
	mux.Handle("DELETE /api/campaigns/{id}/public-link", chain(http.HandlerFunc(publicHandler.RevokeLink), auth))
	mux.Handle("GET /api/campaigns/{id}/public-status", chain(http.HandlerFunc(publicHandler.GetStatus), auth))

	// Forms (mirrors forms.routes.js).
	mux.HandleFunc("GET /api/forms/token/{token}", formHandler.GetByToken)
	mux.HandleFunc("GET /api/forms/slug/{slug}", formHandler.GetBySlug)
	mux.HandleFunc("GET /api/forms/s/{shortCode}", formHandler.RedirectShortCode)
	mux.Handle("PUT /api/forms/{id}/short-code", chain(http.HandlerFunc(formHandler.SetShortCode), auth))
	mux.Handle("DELETE /api/forms/{id}/short-code", chain(http.HandlerFunc(formHandler.RemoveShortCode), auth))
	mux.Handle("GET /api/forms", chain(http.HandlerFunc(formHandler.List), auth))
	mux.Handle("GET /api/forms/{id}", chain(http.HandlerFunc(formHandler.GetByID), auth))
	mux.Handle("POST /api/forms", chain(http.HandlerFunc(formHandler.Create), auth))
	mux.Handle("PUT /api/forms/{id}", chain(http.HandlerFunc(formHandler.Update), auth))
	mux.Handle("DELETE /api/forms/{id}", chain(http.HandlerFunc(formHandler.Delete), auth))

	// Payment (mirrors payment.routes.js).
	mux.HandleFunc("POST /api/payment/webhook", paymentHandler.Webhook)
	mux.HandleFunc("POST /api/payment/webhook/pix", paymentHandler.Webhook)
	mux.Handle("GET /api/payment/config", chain(http.HandlerFunc(paymentHandler.Config), auth))
	mux.Handle("POST /api/payment/campaign/{id}", chain(http.HandlerFunc(paymentHandler.Initiate), auth))
	mux.Handle("GET /api/payment/campaign/{id}/status", chain(http.HandlerFunc(paymentHandler.Status), auth))
	mux.Handle("POST /api/payment/campaign/{id}/topup", chain(http.HandlerFunc(paymentHandler.TopUp), auth))

	// Leads (mirrors leads.routes.js). Public first.
	mux.HandleFunc("POST /api/leads/public/{token}", leadHandler.Create)
	mux.HandleFunc("GET /api/leads/confirm/{code}", leadHandler.ValidateConfirm)
	mux.HandleFunc("POST /api/leads/confirm/{code}", leadHandler.DoConfirm)
	mux.HandleFunc("GET /api/leads/checkin/{code}", leadHandler.ValidateCheckin)
	mux.HandleFunc("POST /api/leads/checkin/{code}", leadHandler.DoCheckin)
	mux.HandleFunc("GET /api/leads/qr/{checkin_code}", leadHandler.ServeQRImage)
	mux.Handle("GET /api/leads", chain(http.HandlerFunc(leadHandler.List), auth))
	mux.Handle("POST /api/leads", chain(http.HandlerFunc(leadHandler.CreateManual), auth))
	mux.Handle("GET /api/leads/history", chain(http.HandlerFunc(leadHandler.History), auth))
	mux.Handle("GET /api/leads/states", chain(http.HandlerFunc(leadHandler.GetStates), auth))
	mux.Handle("GET /api/leads/search", chain(http.HandlerFunc(leadHandler.Search), auth))
	mux.Handle("GET /api/leads/export/{campaign_id}", chain(http.HandlerFunc(leadHandler.Export), auth))
	mux.Handle("GET /api/leads/{id}", chain(http.HandlerFunc(leadHandler.GetByID), auth))
	mux.Handle("PUT /api/leads/{id}", chain(http.HandlerFunc(leadHandler.Update), auth))
	mux.Handle("PATCH /api/leads/{id}/status", chain(http.HandlerFunc(leadHandler.UpdateStatus), auth))
	mux.Handle("DELETE /api/leads/{id}", chain(http.HandlerFunc(leadHandler.Delete), auth))
	mux.Handle("POST /api/leads/{id}/{rest...}", chain(http.HandlerFunc(leadHandler.PostDispatch), auth))

	// Reports (mirrors reports.routes.js).
	mux.Handle("GET /api/reports/admin", chain(http.HandlerFunc(reportHandler.AdminReport), auth, adminOnly))
	mux.Handle("GET /api/reports/system-logs", chain(http.HandlerFunc(reportHandler.SystemLogs), auth, adminOnly))
	mux.Handle("GET /api/reports/supervisor", chain(http.HandlerFunc(reportHandler.SupervisorReport), auth, role(model.RoleAdmin, model.RoleSupervisor)))
	mux.Handle("GET /api/reports/executive", chain(http.HandlerFunc(reportHandler.ExecutiveReport), auth, role(model.RoleAdmin, model.RoleExecutive)))
	mux.Handle("GET /api/reports/overview", chain(http.HandlerFunc(reportHandler.Overview), auth))

	// Notifications (mirrors notifications.routes.js).
	mux.Handle("GET /api/notifications", chain(http.HandlerFunc(notificationHandler.List), auth))
	mux.Handle("GET /api/notifications/unread-count", chain(http.HandlerFunc(notificationHandler.UnreadCount), auth))
	mux.Handle("PATCH /api/notifications/{id}/read", chain(http.HandlerFunc(notificationHandler.MarkAsRead), auth))
	mux.Handle("POST /api/notifications/mark-all-read", chain(http.HandlerFunc(notificationHandler.MarkAllAsRead), auth))

	// Upload (mirrors upload.routes.js).
	mux.Handle("POST /api/upload/image", chain(http.HandlerFunc(uploadHandler.UploadImage), auth))
	mux.Handle("DELETE /api/upload/{id}", chain(http.HandlerFunc(uploadHandler.Delete), auth))

	// Message templates (mirrors message-templates.routes.js).
	mux.Handle("GET /api/message-templates", chain(http.HandlerFunc(templateHandler.List), auth))
	mux.Handle("GET /api/message-templates/lead/{lead_id}/links", chain(http.HandlerFunc(templateHandler.GenerateAllLinks), auth))
	mux.Handle("GET /api/message-templates/{template_id}/lead/{lead_id}/link", chain(http.HandlerFunc(templateHandler.GenerateLink), auth))
	mux.Handle("GET /api/message-templates/{id}", chain(http.HandlerFunc(templateHandler.GetByID), auth))
	mux.Handle("POST /api/message-templates", chain(http.HandlerFunc(templateHandler.Create), auth))
	mux.Handle("PUT /api/message-templates/{id}", chain(http.HandlerFunc(templateHandler.Update), auth))
	mux.Handle("DELETE /api/message-templates/{id}", chain(http.HandlerFunc(templateHandler.Delete), auth))
	mux.Handle("POST /api/message-templates/campaign/{campaign_id}/seed-defaults", chain(http.HandlerFunc(templateHandler.SeedDefaults), auth))

	// Uploaded files (mirrors express.static /uploads, maxAge 7d).
	uploadFiles := http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadDir)))
	mux.Handle("GET /uploads/{path...}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		uploadFiles.ServeHTTP(w, r)
	}))
	mux.Handle("HEAD /uploads/{path...}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		uploadFiles.ServeHTTP(w, r)
	}))

	app := middleware.Logger(middleware.CORS(cfg.CORSOrigin)(middleware.JSON(mux)))

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: app}
	go func() {
		log.Printf("servidor rodando na porta %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	sig := <-stop
	log.Printf("%s recebido, encerrando servidor...", sig)
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("erro ao encerrar: %v", err)
	}
	<-scheduler.Stop().Done()
	pool.Close()
	log.Print("conexões encerradas")
}
