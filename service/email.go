package service

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"

	"igb-leads-go/config"
	"igb-leads-go/model"
)

// EmailService sends transactional mail over SMTP.
// Without credentials it skips sends (logged), like the Node backend.
type EmailService struct {
	cfg *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{cfg: cfg}
}

// SendPasswordResetEmail mirrors the Node template and subject.
func (s *EmailService) SendPasswordResetEmail(to, userName, resetURL string) error {
	if !s.cfg.SMTPConfigured() {
		log.Print("Email não configurado. Pulando envio de reset.")
		return nil
	}
	body := strings.ReplaceAll(resetTemplate, "${userName}", userName)
	body = strings.ReplaceAll(body, "${resetUrl}", resetURL)
	return s.send(to, "🔐 Recuperação de Senha - Sistema de Leads", body)
}

func (s *EmailService) send(to, subject, html string) error {
	from := fmt.Sprintf("%q <%s>", s.cfg.SMTPFromName, s.cfg.SMTPFrom)
	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" + html
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)

	if s.cfg.SMTPSecure {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.cfg.SMTPHost})
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, s.cfg.SMTPHost)
		if err != nil {
			return fmt.Errorf("smtp client: %w", err)
		}
		defer c.Close()
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
		if err := c.Mail(s.cfg.SMTPFrom); err != nil {
			return fmt.Errorf("smtp mail: %w", err)
		}
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("smtp rcpt: %w", err)
		}
		w, err := c.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		defer w.Close()
		if _, err := w.Write([]byte(msg)); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		return nil
	}
	if err := smtp.SendMail(addr, auth, s.cfg.SMTPFrom, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}

const resetTemplate = `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"></head>
<body style="font-family: 'Avenir', Arial, sans-serif; background: #f5f5f5; margin: 0; padding: 0;">
  <div style="max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="background: linear-gradient(135deg, #2350A0 0%, #1C4080 100%); padding: 30px 20px; text-align: center; border-radius: 12px 12px 0 0;">
      <h1 style="color: white; margin: 0; font-size: 28px;">🔐 Recuperação de Senha</h1>
    </div>
    <div style="background: #ffffff; padding: 30px 20px; border-radius: 0 0 12px 12px; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">
      <p style="font-size: 16px; color: #333;">Olá <strong style="color: #2350A0;">${userName}</strong>,</p>
      <p style="font-size: 16px; color: #333;">Recebemos uma solicitação para redefinir sua senha no Sistema de Leads.</p>
      <div style="text-align: center; margin: 30px 0;">
        <a href="${resetUrl}"
           style="display: inline-block; padding: 14px 28px; background: #ECC805; color: #2350A0; text-decoration: none; border-radius: 8px; font-weight: 600; font-size: 16px;">
          Redefinir Senha
        </a>
      </div>
      <div style="background: #FFF9E6; border-left: 4px solid #ECC805; padding: 20px; margin: 20px 0; border-radius: 8px;">
        <p style="margin: 0; color: #666; font-size: 14px;">
          Este link expira em <strong>1 hora</strong>. Se você não solicitou esta alteração, ignore este email.
        </p>
      </div>
      <p style="font-size: 14px; color: #999; word-break: break-all;">
        Ou copie e cole este link no navegador:<br>
        ${resetUrl}
      </p>
      <div style="text-align: center; padding-top: 20px; border-top: 2px solid #ECC805;">
        <p style="color: #666; font-size: 14px; margin: 0;"><strong style="color: #2350A0;">Equipe Sistema de Leads - Gnosis Brasil</strong></p>
      </div>
    </div>
  </div>
</body>
</html>`

// VoucherLead is a row in the reminder email.
type VoucherLead struct {
	Name        string
	Whatsapp    string
	CheckinCode string
	CheckinURL  string
}

// SendVoucherReminderEmail mirrors the scheduler reminder.
func (s *EmailService) SendVoucherReminderEmail(user *model.User, campaign *model.Campaign, leads []model.Lead) error {
	if !s.cfg.SMTPConfigured() {
		log.Print("Email não configurado. Pulando envio.")
		return nil
	}
	rows := make([]VoucherLead, 0, len(leads))
	for _, l := range leads {
		rows = append(rows, VoucherLead{
			Name: l.FirstName + " " + l.LastName, Whatsapp: l.Whatsapp,
			CheckinCode: l.CheckinCode,
			CheckinURL:  strings.TrimSuffix(s.cfg.FrontendURL, "/") + "/checkin/" + l.CheckinCode,
		})
	}
	subject := "🔔 Lembrete: Envie os vouchers - " + campaign.Title
	return s.send(user.Email, subject, renderVoucherReminder(s.cfg.FrontendURL, user, campaign, rows))
}

func renderVoucherReminder(frontendURL string, user *model.User, campaign *model.Campaign, leads []VoucherLead) string {
	items := strings.Builder{}
	for _, lead := range leads {
		items.WriteString(`
      <tr>
        <td style="padding: 12px; border-bottom: 1px solid #ecc805;">
          <strong>` + lead.Name + `</strong><br>
          <span style="color: #666; font-size: 14px;">` + lead.Whatsapp + `</span>
        </td>
        <td style="padding: 12px; border-bottom: 1px solid #ecc805;">
          <code style="background: #f5f5f5; padding: 4px 8px; border-radius: 4px;">` + lead.CheckinCode + `</code>
        </td>
      </tr>
    `)
	}
	eventDate := ""
	if campaign.EventDate != nil {
		eventDate = FormatDateBR(*campaign.EventDate)
	}
	location := "A definir"
	if campaign.Location != nil && *campaign.Location != "" {
		location = *campaign.Location
	}
	return `
      <!DOCTYPE html>
      <html>
      <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>Lembrete de Vouchers</title>
      </head>
      <body style="font-family: 'Avenir', Arial, sans-serif; background: #f5f5f5; margin: 0; padding: 0;">
        <div style="max-width: 600px; margin: 0 auto; padding: 20px;">
          <div style="background: linear-gradient(135deg, #ECC805 0%, #DEC400 100%); padding: 30px 20px; text-align: center; border-radius: 12px 12px 0 0;">
            <h1 style="color: #2350A0; margin: 0; font-size: 28px; font-weight: 700;">
              🔔 Lembrete de Vouchers
            </h1>
          </div>
          <div style="background: #ffffff; padding: 30px 20px; border-radius: 0 0 12px 12px; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">
            <p style="font-size: 16px; color: #333; margin-bottom: 20px;">
              Olá <strong style="color: #2350A0;">` + user.FirstName + `</strong>,
            </p>
            <p style="font-size: 16px; color: #333; margin-bottom: 30px;">
              O evento está próximo e você precisa enviar os vouchers para suas leads confirmadas!
            </p>
            <div style="background: #FFF9E6; border-left: 4px solid #ECC805; padding: 20px; margin-bottom: 30px; border-radius: 8px;">
              <h3 style="margin-top: 0; color: #2350A0;">📅 Detalhes do Evento:</h3>
              <ul style="list-style: none; padding: 0; margin: 0;">
                <li style="padding: 8px 0;"><strong style="color: #C84119;">Campanha:</strong> ` + campaign.Title + `</li>
                <li style="padding: 8px 0;"><strong style="color: #C84119;">Data:</strong> ` + eventDate + `</li>
                <li style="padding: 8px 0;"><strong style="color: #C84119;">Local:</strong> ` + location + `</li>
                <li style="padding: 8px 0;"><strong style="color: #C84119;">Leads Confirmados:</strong> ` + fmt.Sprintf("%d", len(leads)) + `</li>
              </ul>
            </div>
            <div style="text-align: center; margin: 30px 0;">
              <a href="` + strings.TrimSuffix(frontendURL, "/") + `/campaigns/` + campaign.ID + `/leads"
                 style="display: inline-block; padding: 14px 28px; background: #2350A0; color: white; text-decoration: none; border-radius: 8px; font-weight: 600; font-size: 16px;">
                📋 Ver Lista de Leads e Enviar Vouchers
              </a>
            </div>
            <div style="background: #f5f5f5; padding: 20px; border-radius: 8px; margin-bottom: 30px;">
              <h3 style="margin-top: 0; color: #2350A0;">📱 Lista de Leads:</h3>
              <table style="width: 100%; border-collapse: collapse;">
                <thead>
                  <tr style="background: #2350A0;">
                    <th style="padding: 12px; color: white; text-align: left;">Lead</th>
                    <th style="padding: 12px; color: white; text-align: left;">Código Voucher</th>
                  </tr>
                </thead>
                <tbody>
                  ` + items.String() + `
                </tbody>
              </table>
            </div>
            <div style="background: #E8F0F8; padding: 20px; border-radius: 8px; margin-bottom: 20px;">
              <h4 style="margin-top: 0; color: #2350A0;">📱 Como enviar:</h4>
              <ol style="margin: 0; padding-left: 20px;">
                <li style="padding: 8px 0;">Acesse o sistema e clique em "Ver Lista de Leads"</li>
                <li style="padding: 8px 0;">Para cada lead, clique em "Enviar Voucher"</li>
                <li style="padding: 8px 0;">O sistema abrirá o WhatsApp com a mensagem pronta</li>
                <li style="padding: 8px 0;">Envie a mensagem manualmente</li>
                <li style="padding: 8px 0;">Marque como "Enviado" no sistema</li>
              </ol>
            </div>
            <div style="text-align: center; padding-top: 20px; border-top: 2px solid #ECC805;">
              <p style="color: #666; font-size: 14px; margin: 0;">
                <strong style="color: #2350A0;">Equipe Sistema de Leads - Gnosis Brasil</strong>
              </p>
              <p style="color: #999; font-size: 12px; margin: 10px 0 0 0;">
                © ` + fmt.Sprintf("%d", time.Now().Year()) + ` Gnosis Brasil. Todos os direitos reservados.
              </p>
            </div>
          </div>
        </div>
      </body>
      </html>
    `
}
