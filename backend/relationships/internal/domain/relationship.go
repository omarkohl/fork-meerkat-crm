// Package domain is the vocabulary of the relationships module: relationships between contacts
// and the network graph built from them. It imports nothing but the standard library
// (docs/adr/0012-module-layout.md).
package domain

import (
	"errors"
	"time"
)

type UserID uint

type RelationshipID uint

type ContactID uint

var (
	ErrNotFound        = errors.New("relationship not found")
	ErrContactNotFound = errors.New("contact not found")
	ErrRelatedNotFound = errors.New("related contact not found")
)

// ContactRef is the part of a contact a relationship shows.
type ContactRef struct {
	ID        ContactID
	Firstname string
	Lastname  string
	Gender    string
	Birthday  string
}

type Relationship struct {
	ID        RelationshipID
	UserID    UserID
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	ContactID        ContactID
	RelatedContactID *ContactID
	Name             string
	Type             string
	Gender           string
	Birthday         string

	RelatedContact *ContactRef
	SourceContact  *ContactRef
}

// Edit replaces what the user sets. The contact the relationship belongs to never changes.
func (r *Relationship) Edit(in Relationship) {
	r.Name, r.Type, r.Gender, r.Birthday, r.RelatedContactID = in.Name, in.Type, in.Gender, in.Birthday, in.RelatedContactID
}
