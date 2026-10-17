package domain_test

import (
	"testing"
	"time"

	"meerkat/activities/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestEditReplacesTheDescribingFieldsButNotTheContacts(t *testing.T) {
	a := domain.Activity{Title: "old", Contacts: []domain.ContactRef{{ID: 1}}}
	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	a.Edit("new", "d", "l", day)

	assert.Equal(t, "new", a.Title)
	assert.Equal(t, "d", a.Description)
	assert.Equal(t, "l", a.Location)
	assert.Equal(t, day, a.Date)
	assert.Len(t, a.Contacts, 1)
}
