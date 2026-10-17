package domain_test

import (
	"testing"

	"meerkat/contacts/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestArchiveAndUnarchive(t *testing.T) {
	c := domain.Contact{Firstname: "Alice"}
	assert.False(t, c.Archived)

	c.Archive()
	assert.True(t, c.Archived)

	c.Unarchive()
	assert.False(t, c.Archived)
}
