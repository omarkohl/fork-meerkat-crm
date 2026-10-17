// Package contacts is the public entry of the contacts module: it builds the module from its
// dependencies and registers its HTTP operations. Everything else is under internal/
// (docs/adr/0012-module-layout.md).
package contacts

import (
	"meerkat/clock"
	"meerkat/config"
	httpadapter "meerkat/contacts/internal/adapters/http"
	"meerkat/contacts/internal/adapters/persistence"
	"meerkat/contacts/internal/adapters/webhooks"
	"meerkat/contacts/internal/app"
	"meerkat/services"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Deps are the concrete things the module is built on. DB and Config may be nil when only
// the operations' descriptions are needed (cmd/openapi): the handlers are then never run.
type Deps struct {
	DB     *gorm.DB
	Config *config.Config
}

// Module is the contacts module, ready to register.
type Module struct{ uc *app.UseCases }

// New wires the use cases to their adapters.
func New(d Deps) *Module {
	repo := persistence.New(d.DB)
	trigger := func(userID uint, event string, data any, clk clock.Clock) {
		services.TriggerWebhooks(d.DB, *d.Config, userID, event, data, clk)
	}
	return &Module{uc: app.New(repo, repo, webhooks.New(trigger))}
}

// Register adds the module's operations to a. group is the Gin group that carries the
// authentication middleware; nil means the API's default group.
func (m *Module) Register(a huma.API, group gin.IRoutes) {
	httpadapter.Register(a, group, m.uc)
}
