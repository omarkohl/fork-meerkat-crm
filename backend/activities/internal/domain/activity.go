// Package domain is the vocabulary of the activities module: the Activity entity and its rules.
// It imports nothing but the standard library (docs/adr/0012-module-layout.md).
package domain

import (
	"errors"
	"time"
)

type UserID uint

type ActivityID uint

type ContactID uint

// ErrNotFound means there is no such activity for this actor; one of another user looks the same.
var ErrNotFound = errors.New("activity not found")

var ErrContactNotFound = errors.New("contact not found")

// ErrContactsNotFound means at least one of the named contacts is not the actor's.
var ErrContactsNotFound = errors.New("one or more contacts not found")

// ContactRef is the part of a contact an activity shows.
type ContactRef struct {
	ID        ContactID
	Firstname string
	Lastname  string
	Circles   []string
}

type Activity struct {
	ID        ActivityID
	UserID    UserID
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	Title       string
	Description string
	Location    string
	Date        time.Time
	Contacts    []ContactRef
}

func (a *Activity) Edit(title, description, location string, date time.Time) {
	a.Title, a.Description, a.Location, a.Date = title, description, location, date
}
