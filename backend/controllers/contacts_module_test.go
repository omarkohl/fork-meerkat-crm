package controllers

import (
	"meerkat/api"
	"meerkat/config"
	"meerkat/contacts"
	"meerkat/notes"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// mountContacts registers the contacts module's huma operations on router, the way
// routes.RegisterRoutes does, for tests of other controllers that need DELETE /contacts/:id.
func mountContacts(router *gin.Engine, db *gorm.DB, cfg *config.Config) {
	contacts.New(contacts.Deps{DB: db, Config: cfg}).Register(api.New(router.Group(""), router), router)
}

func mountNotes(router *gin.Engine, db *gorm.DB, cfg *config.Config) {
	notes.New(notes.Deps{DB: db, Config: cfg}).Register(api.New(router.Group(""), router), router)
}
