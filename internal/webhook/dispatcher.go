package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Webhook delivery.
 *
 * WebhookStore.GetByEvent had zero callers and webhook_deliveries was never
 * inserted into, so a user could register a webhook, see it listed, and never
 * receive anything. This is the deliverer.
 *
 * Two properties matter more than features here:
 *
 *   - The URL is user-supplied and the server POSTs to it, so ValidateURL runs
 *     before every attempt (not just at registration) to keep DNS rebinding from
 *     turning a stored URL into an internal request later.
 *   - Delivery is best-effort and must never fail the action that triggered it.
 *     A judged submission is final whether or not somebody's endpoint is up.
 */

// Store is the slice of the webhook store this package needs.
type Store interface {
	GetByEvent(ctx context.Context, eventType string) ([]model.Webhook, error)
	RecordDelivery(ctx context.Context, d *model.WebhookDelivery) error
}

// Dispatcher sends webhook events.
type Dispatcher struct {
	store  Store
	client *http.Client
	// timeout bounds one attempt. Deliveries run in the background, so a slow
	// endpoint costs a goroutine, never a user request.
	timeout time.Duration
}

func New(s Store) *Dispatcher {
	if s == nil {
		return nil
	}
	return &Dispatcher{
		store:   s,
		timeout: 10 * time.Second,
		client: &http.Client{
			Timeout: 10 * time.Second,
			// Redirects are refused: a 302 to http://169.254.169.254 would
			// sidestep ValidateURL, which only inspected the submitted URL.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Event is one thing that happened, to be delivered to matching webhooks.
type Event struct {
	Type    string
	Payload any
}

// Dispatch delivers an event to every webhook subscribed to it.
//
// It never returns an error: callers are request handlers whose real work is
// already done. Failures are logged and recorded per delivery.
func (d *Dispatcher) Dispatch(ctx context.Context, ev Event) {
	if d == nil || d.store == nil {
		return
	}
	hooks, err := d.store.GetByEvent(ctx, ev.Type)
	if err != nil {
		slog.Error("webhook: lookup failed", "event", ev.Type, "error", err)
		return
	}

	body, err := json.Marshal(map[string]any{
		"event":   ev.Type,
		"sent_at": time.Now().UTC(),
		"data":    ev.Payload,
	})
	if err != nil {
		slog.Error("webhook: marshal failed", "event", ev.Type, "error", err)
		return
	}

	for _, h := range hooks {
		if !h.IsActive {
			continue
		}
		d.deliver(ctx, h, ev.Type, body)
	}
}

// deliver sends one attempt and records it. Retries are intentionally not
// implemented in-process: the deliveries table carries status/attempts/
// next_retry_at precisely so a sweeper can retry later without holding a
// goroutine. Recording the row is what makes that possible, so it happens even
// when the send fails.
func (d *Dispatcher) deliver(ctx context.Context, h model.Webhook, eventType string, body []byte) {
	rec := &model.WebhookDelivery{
		WebhookID: h.ID,
		EventType: eventType,
		Payload:   body,
	}

	// Re-validate on every attempt: the hostname was attacker-chosen and DNS can
	// change between registration and delivery.
	if err := ValidateURL(h.URL); err != nil {
		rec.Status = model.WebhookDeliveryFailed
		rec.ResponseBody = err.Error()
		slog.Warn("webhook: refused target", "webhook_id", h.ID, "error", err)
		d.record(ctx, rec)
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		rec.Status = model.WebhookDeliveryFailed
		rec.ResponseBody = err.Error()
		d.record(ctx, rec)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AIOJ-Event", eventType)
	req.Header.Set("X-AIOJ-Signature", Sign(h.Secret, body))

	resp, err := d.client.Do(req)
	rec.Attempts = 1
	if err != nil {
		rec.Status = model.WebhookDeliveryFailed
		rec.ResponseBody = err.Error()
		d.record(ctx, rec)
		return
	}
	defer resp.Body.Close()

	// Bound the response we store: an endpoint could stream forever.
	preview, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	rec.ResponseCode = resp.StatusCode
	rec.ResponseBody = string(preview)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		now := time.Now().UTC()
		rec.Status = model.WebhookDeliveryDelivered
		rec.DeliveredAt = &now
	} else {
		rec.Status = model.WebhookDeliveryFailed
	}
	d.record(ctx, rec)
}

func (d *Dispatcher) record(ctx context.Context, rec *model.WebhookDelivery) {
	if err := d.store.RecordDelivery(ctx, rec); err != nil {
		slog.Error("webhook: record delivery failed", "webhook_id", rec.WebhookID, "error", err)
	}
}

// Sign returns the HMAC-SHA256 of body keyed by the webhook's secret, so a
// receiver can verify the call came from this server.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Verify reapplies Sign and compares in constant time. Exported for receivers'
// own tests, and used here to keep the two sides honest.
func Verify(secret string, body []byte, signature string) bool {
	return hmac.Equal([]byte(Sign(secret, body)), []byte(signature))
}
