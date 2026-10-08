// Package model maps the existing sistema-leads PostgreSQL tables.
// JSON tags mirror the Sequelize output (snake_case) so the React frontend
// needs no change. Money/numeric columns scan into *string to preserve the
// exact "1500.10" shape node-postgres produces.
package model

import (
	"encoding/json"
	"time"
)

// Roles and statuses match the Postgres enums.
const (
	RoleUser       = "user"
	RoleSupervisor = "supervisor"
	RoleExecutive  = "executive"
	RoleAdmin      = "admin"

	UserPending   = "pending"
	UserActive    = "active"
	UserSuspended = "suspended"
)

type User struct {
	ID           string     `db:"id" json:"id"`
	Email        string     `db:"email" json:"email"`
	PasswordHash *string    `db:"password_hash" json:"-"`
	FirstName    string     `db:"first_name" json:"first_name"`
	LastName     string     `db:"last_name" json:"last_name"`
	Whatsapp     string     `db:"whatsapp" json:"whatsapp"`
	Role         string     `db:"role" json:"role"`
	Status       string     `db:"status" json:"status"`
	RegionID     *string    `db:"region_id" json:"region_id"`
	ApprovedBy   *string    `db:"approved_by" json:"approved_by"`
	ApprovedAt   *time.Time `db:"approved_at" json:"approved_at"`
	GoogleID     *string    `db:"google_id" json:"google_id"`
	FacebookID   *string    `db:"facebook_id" json:"facebook_id"`
	AvatarURL    *string    `db:"avatar_url" json:"avatar_url"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
	ResetToken   *string    `db:"reset_password_token" json:"-"`
	ResetExpires *time.Time `db:"reset_password_expires" json:"-"`
	State        *string    `db:"state" json:"state"`
	City         *string    `db:"city" json:"city"`
}

type Campaign struct {
	ID                      string          `db:"id" json:"id"`
	Title                   string          `db:"title" json:"title"`
	Description             *string         `db:"description" json:"description"`
	Status                  string          `db:"status" json:"status"`
	Budget                  *string         `db:"budget" json:"budget"`
	Platform                string          `db:"platform" json:"platform"`
	StartDate               *time.Time      `db:"start_date" json:"start_date"`
	EndDate                 *time.Time      `db:"end_date" json:"end_date"`
	EventDate               *time.Time      `db:"event_date" json:"event_date"`
	Location                *string         `db:"location" json:"location"`
	TargetAudience          *string         `db:"target_audience" json:"target_audience"`
	Objectives              *string         `db:"objectives" json:"objectives"`
	ApprovedAt              *time.Time      `db:"approved_at" json:"approved_at"`
	CompletedAt             *time.Time      `db:"completed_at" json:"completed_at"`
	UserID                  string          `db:"user_id" json:"user_id"`
	RegionID                *string         `db:"region_id" json:"region_id"`
	SupervisorID            *string         `db:"supervisor_id" json:"supervisor_id"`
	TrafficManagerID        *string         `db:"traffic_manager_id" json:"traffic_manager_id"`
	CreatedAt               time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt               time.Time       `db:"updated_at" json:"updated_at"`
	PublicLinkToken         *string         `db:"public_link_token" json:"public_link_token"`
	PublicLinkPassword      *string         `db:"public_link_password" json:"public_link_password"`
	PublicLinkActive        bool            `db:"public_link_active" json:"public_link_active"`
	Address                 *string         `db:"address" json:"address"`
	AddressNumber           *string         `db:"address_number" json:"address_number"`
	AddressNeighborhood     *string         `db:"address_neighborhood" json:"address_neighborhood"`
	AddressCity             *string         `db:"address_city" json:"address_city"`
	AddressState            *string         `db:"address_state" json:"address_state"`
	AddressZipcode          *string         `db:"address_zipcode" json:"address_zipcode"`
	MapsURL                 *string         `db:"maps_url" json:"maps_url"`
	PaymentStatus           *string         `db:"payment_status" json:"payment_status"`
	PaymentMethod           *string         `db:"payment_method" json:"payment_method"`
	PaymentGatewayID        *string         `db:"payment_gateway_id" json:"payment_gateway_id"`
	PaymentTxid             *string         `db:"payment_txid" json:"payment_txid"`
	PaymentAmount           *string         `db:"payment_amount" json:"payment_amount"`
	PaymentPaidAt           *time.Time      `db:"payment_paid_at" json:"payment_paid_at"`
	PaymentQRCode           *string         `db:"payment_qr_code" json:"payment_qr_code"`
	PaymentBoletoURL        *string         `db:"payment_boleto_url" json:"payment_boleto_url"`
	PaymentBoletoBarcode    *string         `db:"payment_boleto_barcode" json:"payment_boleto_barcode"`
	FormID                  *string         `db:"form_id" json:"form_id"`
	Latitude                *string         `db:"latitude" json:"latitude"`
	Longitude               *string         `db:"longitude" json:"longitude"`
	WhatsappConfirmationMsg *string         `db:"whatsapp_confirmation_msg" json:"whatsapp_confirmation_msg"`
	WhatsappVoucherMsg      *string         `db:"whatsapp_voucher_msg" json:"whatsapp_voucher_msg"`
	FormTitle               *string         `db:"form_title" json:"form_title"`
	FormDescription         *string         `db:"form_description" json:"form_description"`
	FormFields              json.RawMessage `db:"form_fields" json:"form_fields"`
	FormHeaderText          *string         `db:"form_header_text" json:"form_header_text"`
	FormCTAText             *string         `db:"form_cta_text" json:"form_cta_text"`
	PaymentURL              *string         `db:"payment_url" json:"payment_url"`
	DisplayID               int             `db:"display_id" json:"display_id"`
	RejectionReason         *string         `db:"rejection_reason" json:"rejection_reason"`
	EventTime               *string         `db:"event_time" json:"event_time"`
	Weekdays                json.RawMessage `db:"weekdays" json:"weekdays"`
	EventDates              json.RawMessage `db:"event_dates" json:"event_dates"`
	GoogleMapsLink          *string         `db:"google_maps_link" json:"google_maps_link"`
	LPTemplate              *string         `db:"lp_template" json:"lp_template"`
	PendingEdit             json.RawMessage `db:"pending_edit" json:"pending_edit"`
	Turmas                  json.RawMessage `db:"turmas" json:"turmas"`
	AutoRelationship        bool            `db:"auto_relationship" json:"auto_relationship"`
	AutoRelationshipCost    *string         `db:"auto_relationship_cost" json:"auto_relationship_cost"`
	AcceptedAt              *time.Time      `db:"accepted_at" json:"accepted_at"`
	ResponsibleName         *string         `db:"responsible_name" json:"responsible_name"`
	ResponsibleWhatsapp     *string         `db:"responsible_whatsapp" json:"responsible_whatsapp"`
	TemplateConfig          json.RawMessage `db:"template_config" json:"template_config"`
	Products                json.RawMessage `db:"products" json:"products"`
	SnackPrice              *string         `db:"snack_price" json:"snack_price"`
}

type CampaignDetail struct {
	ID                 string          `db:"id" json:"id"`
	CampaignID         string          `db:"campaign_id" json:"campaign_id"`
	AdCreativeText     *string         `db:"ad_creative_text" json:"ad_creative_text"`
	AdImageURL         *string         `db:"ad_image_url" json:"ad_image_url"`
	LandingPageURL     *string         `db:"landing_page_url" json:"landing_page_url"`
	CallToAction       *string         `db:"call_to_action" json:"call_to_action"`
	TargetDemographics json.RawMessage `db:"target_demographics" json:"target_demographics"`
	CustomFields       json.RawMessage `db:"custom_fields" json:"custom_fields"`
	CreatedAt          time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time       `db:"updated_at" json:"updated_at"`
}

type CampaignEvent struct {
	ID          string    `db:"id" json:"id"`
	CampaignID  string    `db:"campaign_id" json:"campaign_id"`
	Title       string    `db:"title" json:"title"`
	Description *string   `db:"description" json:"description"`
	EventDate   time.Time `db:"event_date" json:"event_date"`
	Location    *string   `db:"location" json:"location"`
	Capacity    *int      `db:"capacity" json:"capacity"`
	IsActive    bool      `db:"is_active" json:"is_active"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type EmailSchedule struct {
	ID            string     `db:"id" json:"id"`
	CampaignID    string     `db:"campaign_id" json:"campaign_id"`
	EventID       *string    `db:"event_id" json:"event_id"`
	EmailType     string     `db:"email_type" json:"email_type"`
	ScheduledFor  time.Time  `db:"scheduled_for" json:"scheduled_for"`
	SentAt        *time.Time `db:"sent_at" json:"sent_at"`
	RecipientType string     `db:"recipient_type" json:"recipient_type"`
	Status        string     `db:"status" json:"status"`
	ErrorMessage  *string    `db:"error_message" json:"error_message"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
}

type Form struct {
	ID           string          `db:"id" json:"id"`
	CampaignID   string          `db:"campaign_id" json:"campaign_id"`
	Slug         string          `db:"slug" json:"slug"`
	Title        string          `db:"title" json:"title"`
	Description  *string         `db:"description" json:"description"`
	IsActive     bool            `db:"is_active" json:"is_active"`
	CustomFields json.RawMessage `db:"custom_fields" json:"custom_fields"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
	PublicToken  string          `db:"public_token" json:"public_token"`
	ShortCode    *string         `db:"short_code" json:"short_code"`
}

type Lead struct {
	ID                  string          `db:"id" json:"id"`
	FormID              *string         `db:"form_id" json:"form_id"`
	FirstName           string          `db:"first_name" json:"first_name"`
	LastName            string          `db:"last_name" json:"last_name"`
	Whatsapp            string          `db:"whatsapp" json:"whatsapp"`
	Email               string          `db:"email" json:"email"`
	CheckinCode         string          `db:"checkin_code" json:"checkin_code"`
	Status              string          `db:"status" json:"status"`
	ConfirmationSentAt  *time.Time      `db:"confirmation_sent_at" json:"confirmation_sent_at"`
	QRSentAt            *time.Time      `db:"qr_sent_at" json:"qr_sent_at"`
	CheckinAt           *time.Time      `db:"checkin_at" json:"checkin_at"`
	Notes               *string         `db:"notes" json:"notes"`
	Metadata            json.RawMessage `db:"metadata" json:"metadata"`
	CreatedAt           time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time       `db:"updated_at" json:"updated_at"`
	ReminderSentAt      *time.Time      `db:"reminder_sent_at" json:"reminder_sent_at"`
	ConfirmedAt         *time.Time      `db:"confirmed_at" json:"confirmed_at"`
	VoucherSentAt       *time.Time      `db:"voucher_sent_at" json:"voucher_sent_at"`
	MotivationSentAt    *time.Time      `db:"motivation_sent_at" json:"motivation_sent_at"`
	ReminderEventSentAt *time.Time      `db:"reminder_event_sent_at" json:"reminder_event_sent_at"`
	AttendedAt          *time.Time      `db:"attended_at" json:"attended_at"`
	CancelledAt         *time.Time      `db:"cancelled_at" json:"cancelled_at"`
	City                *string         `db:"city" json:"city"`
	State               *string         `db:"state" json:"state"`
}

type MessageTemplate struct {
	ID         string    `db:"id" json:"id"`
	CampaignID string    `db:"campaign_id" json:"campaign_id"`
	Key        string    `db:"key" json:"key"`
	Label      string    `db:"label" json:"label"`
	Content    string    `db:"content" json:"content"`
	SortOrder  int       `db:"sort_order" json:"sort_order"`
	IsActive   bool      `db:"is_active" json:"is_active"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
	Phase      string    `db:"phase" json:"phase"`
	IsEditable bool      `db:"is_editable" json:"is_editable"`
}

type Notification struct {
	ID         string    `db:"id" json:"id"`
	UserID     string    `db:"user_id" json:"user_id"`
	Title      string    `db:"title" json:"title"`
	Message    string    `db:"message" json:"message"`
	Type       string    `db:"type" json:"type"`
	EntityType *string   `db:"entity_type" json:"entity_type"`
	EntityID   *string   `db:"entity_id" json:"entity_id"`
	IsRead     bool      `db:"is_read" json:"is_read"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

type Region struct {
	ID             string    `db:"id" json:"id"`
	Name           string    `db:"name" json:"name"`
	Code           string    `db:"code" json:"code"`
	Description    *string   `db:"description" json:"description"`
	IsActive       bool      `db:"is_active" json:"is_active"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
	ParentRegionID *string   `db:"parent_region_id" json:"parent_region_id"`
	// Below: real DB columns the Node API never exposes (its Region model
	// only knows the fields above plus the parent_region_id FK added by the
	// self belongsTo association). Kept mapped for future use.
	Country       string          `db:"country" json:"-"`
	Type          *string         `db:"type" json:"-"`
	States        json.RawMessage `db:"states" json:"-"`
	IBGEStateCode *int            `db:"ibge_state_code" json:"-"`
	IBGECityCodes json.RawMessage `db:"ibge_city_codes" json:"-"`
	Cities        json.RawMessage `db:"cities" json:"-"`
	GeoBounds     json.RawMessage `db:"geo_bounds" json:"-"`
	CenterLat     *string         `db:"center_lat" json:"-"`
	CenterLng     *string         `db:"center_lng" json:"-"`
}

type Setting struct {
	ID        string          `db:"id" json:"id"`
	Key       string          `db:"key" json:"key"`
	Value     json.RawMessage `db:"value" json:"value"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

type SystemLog struct {
	ID          string          `db:"id" json:"id"`
	UserID      *string         `db:"user_id" json:"user_id"`
	Action      string          `db:"action" json:"action"`
	EntityType  *string         `db:"entity_type" json:"entity_type"`
	EntityID    *string         `db:"entity_id" json:"entity_id"`
	Description *string         `db:"description" json:"description"`
	Metadata    json.RawMessage `db:"metadata" json:"metadata"`
	IPAddress   *string         `db:"ip_address" json:"ip_address"`
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at" json:"updated_at"`
}

type Upload struct {
	ID           string    `db:"id" json:"id"`
	FileName     string    `db:"file_name" json:"file_name"`
	OriginalName string    `db:"original_name" json:"original_name"`
	FilePath     string    `db:"file_path" json:"file_path"`
	FileSize     int       `db:"file_size" json:"file_size"`
	MimeType     string    `db:"mime_type" json:"mime_type"`
	UploadedBy   string    `db:"uploaded_by" json:"uploaded_by"`
	EntityType   *string   `db:"entity_type" json:"entity_type"`
	EntityID     *string   `db:"entity_id" json:"entity_id"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}
