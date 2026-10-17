// Package relationships is the public entry of the relationships module: it builds the module from its
// dependencies and registers its HTTP operations. Everything else is under internal/
// (docs/adr/0012-module-layout.md).
package relationships

import (
	"meerkat/config"
	httpadapter "meerkat/relationships/internal/adapters/http"
	"meerkat/relationships/internal/adapters/persistence"
	"meerkat/relationships/internal/app"

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

type Module struct{ uc *app.UseCases }

func New(d Deps) *Module {
	repo := persistence.New(d.DB)
	return &Module{uc: app.New(repo, repo, repo)}
}

// Register adds the module's operations to a. group is the Gin group that carries the
// authentication middleware.
func (m *Module) Register(a huma.API, group gin.IRoutes) {
	httpadapter.Register(a, group, m.uc)
}
