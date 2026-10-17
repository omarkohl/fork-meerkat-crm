package domain_test

import (
	"testing"
	"time"

	"meerkat/notes/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestEditKeepsTheContactWhenNoneIsGiven(t *testing.T) {
	was := domain.ContactID(3)
	n := domain.Note{Content: "old", ContactID: &was}
	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	n.Edit("new", day, nil)
	assert.Equal(t, "new", n.Content)
	assert.Equal(t, day, n.Date)
	assert.Equal(t, &was, n.ContactID)

	moved := domain.ContactID(4)
	n.Edit("new", day, &moved)
	assert.Equal(t, &moved, n.ContactID)
}
