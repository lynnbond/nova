package nova

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ─── Notifier System ──────────────────────────────────────────────────────

// Notifier defines how notifications are delivered.
type Notifier interface {
	// Deliver sends a notification to a user.
	Deliver(ctx context.Context, n *Notification, rawData any) error
	// Name returns the notifier name.
	Name() string
}

// NotifyEvent defines the event data passed to notifiers.
type NotifyEvent struct {
	Notification *Notification
	Entity       *Entity
	ProcessName  string
	ActTitle     string
	HandlerName  string
	Extra        map[string]any
}

// NotifyManager manages multiple notifiers.
type NotifyManager struct {
	notifiers []Notifier
}

// NewNotifyManager creates a new NotifyManager.
func NewNotifyManager(notifiers ...Notifier) *NotifyManager {
	return &NotifyManager{notifiers: notifiers}
}

// Register adds a notifier.
func (m *NotifyManager) Register(n Notifier) {
	m.notifiers = append(m.notifiers, n)
}

// Dispatch sends a notification through all registered notifiers.
func (m *NotifyManager) Dispatch(ctx context.Context, n *Notification, rawData any) {
	for _, notif := range m.notifiers {
		if err := notif.Deliver(ctx, n, rawData); err != nil {
			// Log but don't fail the submission
			fmt.Printf("⚠️  Notifier %s failed: %v\n", notif.Name(), err)
		}
	}
}

// ─── InApp Notifier ───────────────────────────────────────────────────────

// InAppNotifier stores notifications in the app's database.
type InAppNotifier struct {
	store Store
}

// NewInAppNotifier creates an in-app notifier.
func NewInAppNotifier(store Store) *InAppNotifier {
	return &InAppNotifier{store: store}
}

func (n *InAppNotifier) Name() string { return "inapp" }

func (n *InAppNotifier) Deliver(ctx context.Context, notif *Notification, rawData any) error {
	return n.store.CreateNotification(ctx, notif)
}

// ─── WebhookConfig ─────────────────────────────────────────────────────────

// WebhookConfig holds configuration for a webhook notifier.
type WebhookConfig struct {
	URL     string `json:"url"`
	Secret  string `json:"secret,omitempty"` // HMAC signing secret
	Enabled bool   `json:"enabled"`
}

// GlobalWebhookConfig is the global webhook configuration.
type GlobalWebhookConfig struct {
	OnTodoCreated    []WebhookConfig `json:"on_todo_created"`
	OnEntitySubmit   []WebhookConfig `json:"on_entity_submit"`
	OnEntityReturn   []WebhookConfig `json:"on_entity_return"`
	OnEntityComplete []WebhookConfig `json:"on_entity_complete"`
}

// ─── Webhook Notifier ─────────────────────────────────────────────────────

// WebhookNotifier sends notifications via HTTP POST.
type WebhookNotifier struct {
	configs []WebhookConfig
	client  *http.Client
}

// NewWebhookNotifier creates a webhook notifier.
func NewWebhookNotifier(configs []WebhookConfig) *WebhookNotifier {
	return &WebhookNotifier{
		configs: configs,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WebhookNotifier) Name() string { return "webhook" }

func (w *WebhookNotifier) Deliver(ctx context.Context, n *Notification, rawData any) error {
	body, err := json.Marshal(map[string]any{
		"notification": n,
		"data":         rawData,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("marshal webhook: %w", err)
	}
	for _, cfg := range w.configs {
		if !cfg.Enabled {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, "POST", cfg.URL, bytes.NewReader(body))
		if err != nil {
			fmt.Printf("⚠️  webhook req: %v\n", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Nova-Event", n.NotifType)
		if cfg.Secret != "" {
			req.Header.Set("X-Nova-Signature", cfg.Secret) // simplified; use HMAC in prod
		}
		resp, err := w.client.Do(req)
		if err != nil {
			fmt.Printf("⚠️  webhook %s: %v\n", cfg.URL, err)
			continue
		}
		resp.Body.Close()
	}
	return nil
}
