package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"igb-leads-go/authctx"
	"igb-leads-go/config"
	"igb-leads-go/model"
	"igb-leads-go/repository"
	"igb-leads-go/service"
)

// PaymentHandler mirrors PaymentController, Pix-only via Efí.
// Differences from Node, by design: no card/boleto, no simulation mode
// (unconfigured gateway or upstream failure answers an explicit error),
// and the webhook speaks the real Pix {pix:[...]} format.
type PaymentHandler struct {
	cfg       *config.Config
	campaigns *repository.CampaignRepository
	users     *repository.UserRepository
	notify    *repository.NotificationRepository
	efi       *service.EfiClient
	advance   func(r *http.Request, c *model.Campaign) error
}

func NewPaymentHandler(cfg *config.Config, campaigns *repository.CampaignRepository, users *repository.UserRepository, notify *repository.NotificationRepository, efi *service.EfiClient, advance func(r *http.Request, c *model.Campaign) error) *PaymentHandler {
	return &PaymentHandler{cfg: cfg, campaigns: campaigns, users: users, notify: notify, efi: efi, advance: advance}
}

func round2(amount float64) float64 {
	return math.Round(amount*100) / 100
}

func newTxid(prefix, campaignID string) string {
	hex := strings.ToUpper(strings.ReplaceAll(campaignID, "-", ""))
	if len(hex) > 15 {
		hex = hex[:15]
	}
	return fmt.Sprintf("%s%s%d", prefix, hex, time.Now().UnixMilli())
}

func (h *PaymentHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Method       string `json:"method"`
		CustomerInfo *struct {
			Name  string `json:"name"`
			CPF   string `json:"cpf"`
			CNPJ  string `json:"cnpj"`
			Email string `json:"email"`
		} `json:"customer_info"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := r.PathValue("id")
	log.Printf("Payment: iniciando pagamento campanha %s, método: %s", id, body.Method)

	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Campanha não encontrada")
		return
	}
	user := authctx.CurrentUser(r)
	if c.UserID != authctx.UserID(r) && user.Role != model.RoleAdmin {
		WriteError(w, http.StatusForbidden, "Acesso negado")
		return
	}
	if c.PaymentStatus != nil && *c.PaymentStatus == "paid" {
		WriteError(w, http.StatusBadRequest, "Campanha já está paga")
		return
	}
	var amount float64
	if c.Budget != nil {
		amount, _ = strconv.ParseFloat(*c.Budget, 64)
	}
	if math.IsNaN(amount) || amount <= 0 {
		WriteError(w, http.StatusBadRequest, "Orçamento da campanha deve ser um valor numérico maior que zero")
		return
	}
	amount = round2(amount)

	if body.Method != "pix" {
		WriteError(w, http.StatusBadRequest, "Método de pagamento inválido")
		return
	}
	if !h.efi.IsConfigured() {
		WriteError(w, http.StatusServiceUnavailable, "Gateway de pagamento não configurado")
		return
	}
	payer := service.Payer{}
	if body.CustomerInfo != nil {
		payer = service.Payer{Name: body.CustomerInfo.Name, CPF: service.DigitsOnly(body.CustomerInfo.CPF), CNPJ: service.DigitsOnly(body.CustomerInfo.CNPJ)}
	}
	if payer.Name == "" {
		payer.Name = strings.TrimSpace(user.FirstName + " " + user.LastName)
	}
	txid := newTxid("CAMP", c.ID)
	pix, err := h.efi.CreatePixCharge(r.Context(), amount, "Campanha: "+c.Title, txid, payer)
	if err != nil {
		log.Printf("Payment: EFI PIX falhou: %v", err)
		WriteError(w, http.StatusBadGateway, "Falha ao gerar pagamento Pix")
		return
	}
	paymentData := map[string]any{
		"payment_method":     "pix",
		"payment_gateway_id": pix.Txid,
		"payment_txid":       txid,
		"payment_amount":     fmt.Sprintf("%.2f", amount),
		"payment_qr_code":    pix.QRCode,
		"payment_status":     "pending",
	}
	if err := h.campaigns.UpdateFields(r.Context(), id, paymentData); err != nil {
		log.Printf("Payment: erro crítico ao iniciar pagamento: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro ao processar pagamento")
		return
	}
	log.Printf("Payment: pagamento iniciado para campanha %s, método pix", c.ID)
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Pagamento iniciado", "method": "pix", "amount": amount, "txid": txid,
		"payment_method": "pix", "payment_gateway_id": pix.Txid, "payment_txid": txid,
		"payment_amount": amount, "payment_qr_code": pix.QRCode, "payment_status": "pending",
	})
}

// PaymentStatusView mirrors the getPaymentStatus projection.
type PaymentStatusView struct {
	ID                   string     `json:"id"`
	Title                string     `json:"title"`
	PaymentStatus        *string    `json:"payment_status"`
	PaymentMethod        *string    `json:"payment_method"`
	PaymentAmount        *string    `json:"payment_amount"`
	PaymentPaidAt        *time.Time `json:"payment_paid_at"`
	PaymentQRCode        *string    `json:"payment_qr_code"`
	PaymentBoletoURL     *string    `json:"payment_boleto_url"`
	PaymentBoletoBarcode *string    `json:"payment_boleto_barcode"`
	PaymentTxid          *string    `json:"payment_txid"`
}

func (h *PaymentHandler) Status(w http.ResponseWriter, r *http.Request) {
	c, err := h.campaigns.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Campanha não encontrada")
		return
	}
	if c.PaymentStatus != nil && *c.PaymentStatus == "pending" && c.PaymentTxid != nil && h.efi.IsConfigured() {
		if st, _ := h.efi.VerifyPayment(r.Context(), *c.PaymentTxid); st != nil {
			switch st.Status {
			case "CONCLUIDA":
				now := now()
				if err := h.campaigns.UpdateFields(r.Context(), c.ID, map[string]any{
					"payment_status": "paid", "payment_paid_at": now,
				}); err == nil {
					paid := "paid"
					c.PaymentStatus = &paid
					c.PaymentPaidAt = &now
				}
			case "REMOVIDA_PELO_USUARIO_RECEBEDOR", "REMOVIDA_PELO_PSP":
				if err := h.campaigns.UpdateFields(r.Context(), c.ID, map[string]any{"payment_status": "cancelled"}); err == nil {
					cancelled := "cancelled"
					c.PaymentStatus = &cancelled
				}
			}
		}
	}
	WriteJSON(w, http.StatusOK, PaymentStatusView{
		ID: c.ID, Title: c.Title, PaymentStatus: c.PaymentStatus, PaymentMethod: c.PaymentMethod,
		PaymentAmount: c.PaymentAmount, PaymentPaidAt: c.PaymentPaidAt, PaymentQRCode: c.PaymentQRCode,
		PaymentBoletoURL: c.PaymentBoletoURL, PaymentBoletoBarcode: c.PaymentBoletoBarcode, PaymentTxid: c.PaymentTxid,
	})
}

// PixWebhook mirrors the real Efí notification payload {pix:[...]}.
// Every claimed txid is re-verified against Efí before the campaign is
// marked paid (verify-before-trust: the payload alone proves nothing).
// Decidable outcomes acknowledge 200; when verification itself fails the
// handler answers 502 so Efí retries the notification later.
func (h *PaymentHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pix []struct {
			Txid       string `json:"txid"`
			EndToEndID string `json:"endToEndId"`
		} `json:"pix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	needRetry := false
	for _, p := range body.Pix {
		if p.Txid == "" {
			continue
		}
		c, err := h.campaigns.ByPaymentTxid(r.Context(), p.Txid)
		if err != nil {
			log.Printf("Webhook EFI: txid desconhecido %s", p.Txid)
			continue
		}
		if c.PaymentStatus == nil || *c.PaymentStatus != "pending" {
			continue
		}
		if !h.efi.IsConfigured() {
			log.Printf("Webhook EFI: gateway não configurado, ignorando %s", p.Txid)
			continue
		}
		st, err := h.efi.GetCobStatus(r.Context(), p.Txid)
		if errors.Is(err, service.ErrCobNotFound) {
			log.Printf("Webhook EFI: txid %s não existe na Efí, ignorando", p.Txid)
			continue
		}
		if err != nil {
			log.Printf("Webhook EFI: falha ao confirmar %s junto à Efí: %v", p.Txid, err)
			needRetry = true
			continue
		}
		if st != "CONCLUIDA" {
			log.Printf("Webhook EFI: txid %s com status %s, mantendo pendente", p.Txid, st)
			continue
		}
		if err := h.campaigns.UpdateFields(r.Context(), c.ID, map[string]any{
			"payment_status": "paid", "payment_paid_at": now(),
		}); err != nil {
			log.Printf("Webhook EFI: erro ao dar baixa %s: %v", p.Txid, err)
			continue
		}
		log.Printf("Webhook EFI: pagamento confirmado para campanha %s", c.ID)
		if c.Status == "payment_pending" {
			if err := h.advance(r, c); err != nil {
				log.Printf("Webhook EFI: erro ao avançar campanha %s: %v", c.ID, err)
			}
		}
		h.notify.NotifyPaymentConfirmed(r.Context(), c.ID, c.Title, c.UserID)
	}
	if needRetry {
		WriteError(w, http.StatusBadGateway, "Falha ao confirmar pagamento junto à Efí")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "received"})
}

func (h *PaymentHandler) Config(w http.ResponseWriter, r *http.Request) {
	configured := h.efi.IsConfigured()
	sandbox := h.efi.IsSandbox()
	msg := "Gateway não configurado"
	if configured {
		if sandbox {
			msg = "Modo Sandbox"
		} else {
			msg = "Produção"
		}
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"methods": []string{"pix"}, "sandbox": sandbox,
		"configured": configured, "message": msg,
	})
}

func (h *PaymentHandler) TopUp(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Amount       float64 `json:"amount"`
		CustomerInfo *struct {
			Name string `json:"name"`
			CPF  string `json:"cpf"`
			CNPJ string `json:"cnpj"`
		} `json:"customer_info"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "Corpo inválido")
		return
	}
	if body.Amount <= 0 || math.IsNaN(body.Amount) {
		WriteError(w, http.StatusBadRequest, "Valor inválido")
		return
	}
	id := r.PathValue("id")
	c, err := h.campaigns.ByID(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Campanha não encontrada")
		return
	}
	user := authctx.CurrentUser(r)
	if c.UserID != authctx.UserID(r) && user.Role != model.RoleAdmin {
		WriteError(w, http.StatusForbidden, "Acesso negado")
		return
	}
	if !h.efi.IsConfigured() {
		WriteError(w, http.StatusServiceUnavailable, "Gateway de pagamento não configurado")
		return
	}
	finalAmount := round2(body.Amount)
	txid := newTxid("TOP", c.ID)
	payer := service.Payer{}
	if body.CustomerInfo != nil {
		payer = service.Payer{Name: body.CustomerInfo.Name, CPF: service.DigitsOnly(body.CustomerInfo.CPF), CNPJ: service.DigitsOnly(body.CustomerInfo.CNPJ)}
	}
	if payer.Name == "" {
		payer.Name = strings.TrimSpace(user.FirstName + " " + user.LastName)
	}
	pix, err := h.efi.CreatePixCharge(r.Context(), finalAmount, "Adicional: "+c.Title, txid, payer)
	if err != nil {
		log.Printf("Payment: EFI topup falhou: %v", err)
		WriteError(w, http.StatusBadGateway, "Falha ao gerar pagamento Pix")
		return
	}
	var current float64
	if c.Budget != nil {
		current, _ = strconv.ParseFloat(*c.Budget, 64)
	}
	total := current + finalAmount
	if err := h.campaigns.UpdateFields(r.Context(), id, map[string]any{
		"budget":             fmt.Sprintf("%.2f", total),
		"payment_gateway_id": pix.Txid,
		"payment_txid":       txid,
		"payment_qr_code":    pix.QRCode,
	}); err != nil {
		log.Printf("Payment: erro no crédito adicional: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro ao processar crédito adicional")
		return
	}
	h.notify.NotifyTopUp(r.Context(), h.users, c.ID, c.Title, c.TrafficManagerID, c.RegionID, c.UserID, finalAmount)
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Crédito adicional processado", "amount": finalAmount, "total_budget": total,
		"payment_gateway_id": pix.Txid, "payment_txid": txid, "payment_qr_code": pix.QRCode,
	})
}
