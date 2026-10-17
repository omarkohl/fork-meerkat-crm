// Package domain is the vocabulary of the contacts module: the Contact entity and its rules.
// It imports nothing but the standard library (docs/adr/0012-module-layout.md).
package domain

import (
	"errors"
	"time"
)

// UserID identifies the user who owns data; every operation is done by an actor.
type UserID uint

// ContactID identifies a contact.
type ContactID uint

// ErrNotFound means there is no such contact for this actor. It is also what a contact of
// another user looks like, so existence is never leaked.
var ErrNotFound = errors.New("contact not found")

// Value is a typed value: an email address, phone number, website or messaging handle.
type Value struct {
	Type  string
	Value string
}

// Address is a structured postal address.
type Address struct {
	Type    string
	Street  string
	City    string
	Region  string
	Postal  string
	Country string
}

// Contact is a person in a user's address book.
type Contact struct {
	ID        ContactID
	UserID    UserID
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	Firstname string
	Lastname  string
	Nickname  string
	Gender    string
	Email     string
	Phone     string
	Birthday  string
	Photo     string

	Address            string
	HowWeMet           string
	FoodPreference     string
	WorkInformation    string
	ContactInformation string
	Circles            []string

	Emails    []Value
	Phones    []Value
	Addresses []Address
	URLs      []Value
	IMPPs     []Value

	Prefix     string
	MiddleName string
	Suffix     string

	Organization string
	Department   string
	JobTitle     string
	Role         string

	Anniversary string
	Note        string

	CustomFields map[string]string

	Archived bool
}

// Archive hides the contact from the active lists. Archiving also ends the contact's
// reminders, which is a rule of the use case because reminders are another module's data.
func (c *Contact) Archive() { c.Archived = true }

// Unarchive brings an archived contact back.
func (c *Contact) Unarchive() { c.Archived = false }
