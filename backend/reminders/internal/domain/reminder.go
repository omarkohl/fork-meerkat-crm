// Package domain is the vocabulary of the reminders module: the Reminder entity, its
// recurrence and what completing it means. It imports nothing but the standard library
// (docs/adr/0012-module-layout.md).
package domain

import (
	"errors"
	"time"
)

type UserID uint

type ReminderID uint

type ContactID uint

type CompletionID uint

var (
	ErrNotFound           = errors.New("reminder not found")
	ErrContactNotFound    = errors.New("contact not found")
	ErrCompletionNotFound = errors.New("reminder completion not found")
)

const (
	Once      = "once"
	Weekly    = "weekly"
	Monthly   = "monthly"
	Quarterly = "quarterly"
	SixMonths = "six-months"
	Yearly    = "yearly"
)

type Reminder struct {
	ID        ReminderID
	UserID    UserID
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	Message               string
	ByMail                *bool
	RemindAt              time.Time
	Recurrence            string
	ReoccurFromCompletion *bool
	Completed             bool
	EmailSent             bool
	LastSent              *time.Time
	ContactID             *ContactID
}

// Completion is the timeline record of a reminder that was done.
type Completion struct {
	ID          CompletionID
	UserID      UserID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
	ReminderID  *ReminderID
	ContactID   ContactID
	Message     string
	CompletedAt time.Time
}

// StartOfDay drops the time of day so a reminder is found when comparing with "until" dates.
func StartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Edit replaces what the user sets. by_mail and reoccur_from_completion left out keep their
// current values; the day, not the time of day, decides when the reminder is due.
func (r *Reminder) Edit(in Reminder) {
	r.Message, r.Recurrence, r.ContactID = in.Message, in.Recurrence, in.ContactID
	r.RemindAt = StartOfDay(in.RemindAt)
	if in.ByMail != nil {
		r.ByMail = in.ByMail
	}
	if in.ReoccurFromCompletion != nil {
		r.ReoccurFromCompletion = in.ReoccurFromCompletion
	}
}

func (r *Reminder) reoccursFromCompletion() bool {
	return r.ReoccurFromCompletion == nil || *r.ReoccurFromCompletion
}

// Complete marks the reminder done at now. A recurring one that reoccurs from completion is
// scheduled again instead; a "once" reminder is finished and reports true, to be deleted.
func (r *Reminder) Complete(now time.Time) (finished bool) {
	r.Completed = true
	r.LastSent = &now
	if r.reoccursFromCompletion() && r.Recurrence != Once {
		r.RemindAt = r.NextOccurrence(now)
		r.Completed = false
		r.EmailSent = false
	}
	return r.Recurrence == Once
}

// Record is the timeline entry for completing the reminder at now.
func (r Reminder) Record(now time.Time) Completion {
	c := Completion{UserID: r.UserID, Message: r.Message, CompletedAt: now}
	id := r.ID
	c.ReminderID = &id
	if r.ContactID != nil {
		c.ContactID = *r.ContactID
	}
	return c
}

// NextOccurrence is when the reminder is due next, calculated in UTC. A reminder that reoccurs
// from completion counts from now when it is overdue and from its due date when it is not.
func (r Reminder) NextOccurrence(now time.Time) time.Time {
	now = now.UTC()
	due := r.RemindAt.UTC()

	base := due
	if r.reoccursFromCompletion() && !due.After(now) {
		base = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	}

	switch r.Recurrence {
	case Weekly:
		return base.AddDate(0, 0, 7)
	case Monthly:
		return addMonths(base, 1)
	case Quarterly:
		return addMonths(base, 3)
	case SixMonths:
		return addMonths(base, 6)
	case Yearly:
		return addYears(base, 1)
	}
	return r.RemindAt
}

// addMonths clamps to the last day of the target month: Jan 31 + 1 month is Feb 28 or 29.
func addMonths(t time.Time, months int) time.Time {
	result := t.AddDate(0, months, 0)
	if result.Day() != t.Day() {
		result = result.AddDate(0, 0, -result.Day())
	}
	return result
}

func addYears(t time.Time, years int) time.Time {
	result := t.AddDate(years, 0, 0)
	if result.Day() != t.Day() {
		result = result.AddDate(0, 0, -result.Day())
	}
	return result
}
