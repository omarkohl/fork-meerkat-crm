// Package domain is the vocabulary of the notes module: the Note entity and its rules.
// It imports nothing but the standard library (docs/adr/0012-module-layout.md).
package domain

import (
	"errors"
	"time"
)

type UserID uint

type NoteID uint

type ContactID uint

// ErrNotFound means there is no such note for this actor; a note of another user looks the same.
var ErrNotFound = errors.New("note not found")

var ErrContactNotFound = errors.New("contact not found")

type Note struct {
	ID        NoteID
	UserID    UserID
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	Content   string
	Date      time.Time
	ContactID *ContactID
}

// Edit replaces the content and date. A note already about a contact stays with it when the
// edit names none: it can be moved to another contact but not made free-floating.
func (n *Note) Edit(content string, date time.Time, contact *ContactID) {
	n.Content, n.Date = content, date
	if contact != nil {
		n.ContactID = contact
	}
}
