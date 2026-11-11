package model

import "time"

type Webhook struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	URL         string    `json:"url"`
	Secret      string    `json:"-"`
	Events      []string  `json:"events"`
	IsActive    bool      `json:"is_active"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateWebhookRequest struct {
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	Description string   `json:"description,omitempty"`
}

// Webhook delivery states. The deliveries table has carried status/attempts/
// next_retry_at since migration 000015 while nothing ever wrote a row.
const (
	WebhookDeliveryPending   = "pending"
	WebhookDeliveryDelivered = "delivered"
	WebhookDeliveryFailed    = "failed"
)

// WebhookDelivery is one attempt to deliver one event to one webhook.
type WebhookDelivery struct {
	ID           string     `json:"id"`
	WebhookID    string     `json:"webhook_id"`
	EventType    string     `json:"event_type"`
	Payload      []byte     `json:"-"`
	Status       string     `json:"status"`
	ResponseCode int        `json:"response_code,omitempty"`
	ResponseBody string     `json:"response_body,omitempty"`
	Attempts     int        `json:"attempts"`
	NextRetryAt  *time.Time `json:"next_retry_at,omitempty"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
