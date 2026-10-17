// Package webhooks implements the notes module's event port with the webhook service.
package webhooks

import (
	"context"
	"time"

	"meerkat/api"
	"meerkat/clock"
	httpadapter "meerkat/notes/internal/adapters/http"
	"meerkat/notes/internal/app"
	"meerkat/notes/internal/domain"
)

// Trigger sends the webhook deliveries subscribed to an event (services.TriggerWebhooks,
// bound to a database and configuration by the module's entry package).
type Trigger func(userID uint, event string, data any, clk clock.Clock)

type Publisher struct{ trigger Trigger }

func New(trigger Trigger) *Publisher { return &Publisher{trigger: trigger} }

var _ app.Events = (*Publisher)(nil)

func (p *Publisher) NoteCreated(ctx context.Context, actor domain.UserID, n domain.Note, _ time.Time) {
	go p.trigger(uint(actor), "note.created", httpadapter.NewNoteResponse(n), api.Clock(ctx))
}

func (p *Publisher) NoteUpdated(ctx context.Context, actor domain.UserID, n domain.Note, _ time.Time) {
	go p.trigger(uint(actor), "note.updated", httpadapter.NewNoteResponse(n), api.Clock(ctx))
}

func (p *Publisher) NoteDeleted(ctx context.Context, actor domain.UserID, id domain.NoteID, _ time.Time) {
	go p.trigger(uint(actor), "note.deleted", map[string]any{"id": uint(id)}, api.Clock(ctx))
}
