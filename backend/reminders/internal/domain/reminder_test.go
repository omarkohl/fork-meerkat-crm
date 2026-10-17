package domain_test

import (
	"testing"
	"time"

	"meerkat/reminders/internal/domain"

	"github.com/stretchr/testify/assert"
)

var now = time.Date(2026, 3, 10, 15, 30, 0, 0, time.UTC)

func TestStartOfDayKeepsTheDateAndLocation(t *testing.T) {
	berlin, _ := time.LoadLocation("Europe/Berlin")
	got := domain.StartOfDay(time.Date(2026, 3, 10, 23, 59, 0, 0, berlin))
	assert.Equal(t, time.Date(2026, 3, 10, 0, 0, 0, 0, berlin), got)
}

func TestCompletingAOnceReminderFinishesIt(t *testing.T) {
	r := domain.Reminder{Recurrence: domain.Once, RemindAt: now.AddDate(0, 0, -1)}

	finished := r.Complete(now)

	assert.True(t, finished)
	assert.True(t, r.Completed)
	assert.Equal(t, &now, r.LastSent)
}

func TestCompletingARecurringReminderSchedulesTheNextOccurrence(t *testing.T) {
	r := domain.Reminder{Recurrence: domain.Weekly, RemindAt: now.AddDate(0, 0, -3), EmailSent: true}

	finished := r.Complete(now)

	assert.False(t, finished)
	assert.False(t, r.Completed)
	assert.False(t, r.EmailSent)
	assert.Equal(t, time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC), r.RemindAt, "a week from today, as it was overdue")
}

func TestAReminderNotRescheduledFromCompletionStaysCompleted(t *testing.T) {
	never := false
	due := now.AddDate(0, 0, -3)
	r := domain.Reminder{Recurrence: domain.Weekly, RemindAt: due, ReoccurFromCompletion: &never}

	assert.False(t, r.Complete(now))
	assert.True(t, r.Completed)
	assert.Equal(t, due, r.RemindAt)
}

func TestAFutureReminderKeepsItsRhythmWhenCompletedEarly(t *testing.T) {
	r := domain.Reminder{Recurrence: domain.Monthly, RemindAt: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)}
	r.Complete(now)
	assert.Equal(t, time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), r.RemindAt)
}

func TestTheCompletionRecordNamesTheReminderAndContact(t *testing.T) {
	contact := domain.ContactID(4)
	r := domain.Reminder{ID: 9, UserID: 2, Message: "call", ContactID: &contact}

	c := r.Record(now)

	assert.Equal(t, domain.ContactID(4), c.ContactID)
	assert.Equal(t, domain.ReminderID(9), *c.ReminderID)
	assert.Equal(t, domain.UserID(2), c.UserID)
	assert.Equal(t, "call", c.Message)
	assert.Equal(t, now, c.CompletedAt)
}

func TestEditKeepsFlagsThatAreLeftOutAndDropsTheTimeOfDay(t *testing.T) {
	yes := true
	r := domain.Reminder{Message: "old", ByMail: &yes, ReoccurFromCompletion: &yes}
	contact := domain.ContactID(2)

	r.Edit(domain.Reminder{Message: "new", Recurrence: domain.Monthly, RemindAt: now, ContactID: &contact})

	assert.Equal(t, "new", r.Message)
	assert.Equal(t, time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), r.RemindAt)
	assert.Equal(t, &contact, r.ContactID)
	assert.True(t, *r.ByMail)
	assert.True(t, *r.ReoccurFromCompletion)

	no := false
	r.Edit(domain.Reminder{ByMail: &no})
	assert.False(t, *r.ByMail)
}
