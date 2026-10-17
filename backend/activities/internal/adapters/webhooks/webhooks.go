// Package webhooks implements the activities module's event port with the webhook service.
package webhooks

import (
	"context"
	"time"

	httpadapter "meerkat/activities/internal/adapters/http"
	"meerkat/activities/internal/app"
	"meerkat/activities/internal/domain"
	"meerkat/api"
	"meerkat/clock"
)

// Trigger sends the webhook deliveries subscribed to an event (services.TriggerWebhooks,
// bound to a database and configuration by the module's entry package).
type Trigger func(userID uint, event string, data any, clk clock.Clock)

type Publisher struct{ trigger Trigger }

func New(trigger Trigger) *Publisher { return &Publisher{trigger: trigger} }

var _ app.Events = (*Publisher)(nil)

func (p *Publisher) ActivityCreated(ctx context.Context, actor domain.UserID, a domain.Activity, _ time.Time) {
	go p.trigger(uint(actor), "activity.created", httpadapter.NewActivityResponse(a), api.Clock(ctx))
}

func (p *Publisher) ActivityUpdated(ctx context.Context, actor domain.UserID, a domain.Activity, _ time.Time) {
	go p.trigger(uint(actor), "activity.updated", httpadapter.NewActivityResponse(a), api.Clock(ctx))
}

func (p *Publisher) ActivityDeleted(ctx context.Context, actor domain.UserID, id domain.ActivityID, _ time.Time) {
	go p.trigger(uint(actor), "activity.deleted", map[string]any{"id": uint(id)}, api.Clock(ctx))
}
