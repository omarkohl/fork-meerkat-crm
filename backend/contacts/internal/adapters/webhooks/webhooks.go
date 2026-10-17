// Package webhooks implements the contacts module's event port with the webhook service.
package webhooks

import (
	"context"
	"time"

	"meerkat/api"
	"meerkat/clock"
	httpadapter "meerkat/contacts/internal/adapters/http"
	"meerkat/contacts/internal/app"
	"meerkat/contacts/internal/domain"
)

// Trigger sends the webhook deliveries subscribed to an event (services.TriggerWebhooks,
// bound to a database and configuration by the module's entry package).
type Trigger func(userID uint, event string, data any, clk clock.Clock)

// Publisher sends webhook deliveries for contact events.
type Publisher struct{ trigger Trigger }

// New returns a publisher.
func New(trigger Trigger) *Publisher { return &Publisher{trigger: trigger} }

var _ app.Events = (*Publisher)(nil)

// ContactCreated implements app.Events.
func (p *Publisher) ContactCreated(ctx context.Context, actor domain.UserID, c domain.Contact, _ time.Time) {
	go p.trigger(uint(actor), "contact.created", httpadapter.NewContactResponse(c), api.Clock(ctx))
}

// ContactUpdated implements app.Events.
func (p *Publisher) ContactUpdated(ctx context.Context, actor domain.UserID, c domain.Contact, _ time.Time) {
	go p.trigger(uint(actor), "contact.updated", httpadapter.NewContactResponse(c), api.Clock(ctx))
}

// ContactDeleted implements app.Events. Deliveries run in the background; the clock is read
// again when they are claimed and finished.
func (p *Publisher) ContactDeleted(ctx context.Context, actor domain.UserID, id domain.ContactID, _ time.Time) {
	go p.trigger(uint(actor), "contact.deleted", map[string]any{"id": uint(id)}, api.Clock(ctx))
}
