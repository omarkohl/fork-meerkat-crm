// Package app holds the use cases of the reminders module. It depends on ports (the interfaces
// below) and on the domain, never on GORM, Gin or huma (docs/adr/0012-module-layout.md).
package app

import (
	"context"
	"errors"
	"time"

	"meerkat/reminders/internal/domain"
)

const (
	upcomingWindowDays = 7
	upcomingMinimum    = 5
)

// Reminders reads reminders and completions. Every method is scoped to the actor.
type Reminders interface {
	Get(ctx context.Context, actor domain.UserID, id domain.ReminderID) (domain.Reminder, error)
	// All returns every reminder, due first.
	All(ctx context.Context, actor domain.UserID) ([]domain.Reminder, error)
	ForContact(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Reminder, error)
	// Open returns reminders not completed, due first, up to limit (0 for no limit) and due by until (zero for any time).
	Open(ctx context.Context, actor domain.UserID, until time.Time, limit int) ([]domain.Reminder, error)
	Completions(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Completion, error)
}

type Tx interface {
	Get(ctx context.Context, actor domain.UserID, id domain.ReminderID) (domain.Reminder, error)
	ContactExists(ctx context.Context, actor domain.UserID, id domain.ContactID) (bool, error)
	Create(ctx context.Context, actor domain.UserID, r domain.Reminder) (domain.Reminder, error)
	Update(ctx context.Context, actor domain.UserID, r domain.Reminder) (domain.Reminder, error)
	Delete(ctx context.Context, actor domain.UserID, id domain.ReminderID) error
	RecordCompletion(ctx context.Context, actor domain.UserID, c domain.Completion) error
	GetCompletion(ctx context.Context, actor domain.UserID, id domain.CompletionID) error
	DeleteCompletion(ctx context.Context, actor domain.UserID, id domain.CompletionID) error
}

type Store interface {
	InTx(ctx context.Context, fn func(Tx) error) error
}

type Stage string

const (
	StageRetrieve         Stage = "retrieve"
	StageContact          Stage = "contact"
	StageList             Stage = "list"
	StageCreate           Stage = "create"
	StageUpdate           Stage = "update"
	StageFinish           Stage = "finish"
	StageDelete           Stage = "delete"
	StageCompletions      Stage = "completions"
	StageGetCompletion    Stage = "get-completion"
	StageDeleteCompletion Stage = "delete-completion"
)

// Failure wraps an infrastructure error with the stage it happened in. A missing reminder,
// contact or completion is not a Failure: it is a domain error.
type Failure struct {
	Stage Stage
	Err   error
}

func (f *Failure) Error() string { return string(f.Stage) + ": " + f.Err.Error() }
func (f *Failure) Unwrap() error { return f.Err }

type UseCases struct {
	reminders Reminders
	store     Store
}

func New(reminders Reminders, store Store) *UseCases {
	return &UseCases{reminders: reminders, store: store}
}

func fail(stage Stage, err error) error {
	if err == nil {
		return nil
	}
	var f *Failure
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrContactNotFound) ||
		errors.Is(err, domain.ErrCompletionNotFound) || errors.As(err, &f) {
		return err
	}
	return &Failure{Stage: stage, Err: err}
}

func (u *UseCases) GetReminder(ctx context.Context, actor domain.UserID, id domain.ReminderID) (domain.Reminder, error) {
	r, err := u.reminders.Get(ctx, actor, id)
	if err != nil {
		return domain.Reminder{}, fail(StageRetrieve, err)
	}
	return r, nil
}

func (u *UseCases) AllReminders(ctx context.Context, actor domain.UserID) ([]domain.Reminder, error) {
	list, err := u.reminders.All(ctx, actor)
	if err != nil {
		return nil, fail(StageList, err)
	}
	return list, nil
}

func (u *UseCases) RemindersOfContact(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Reminder, error) {
	list, err := u.reminders.ForContact(ctx, actor, contact)
	if err != nil {
		return nil, fail(StageContact, err)
	}
	return list, nil
}

// UpcomingReminders returns everything due within a week when that is more than five
// reminders, otherwise the next five open ones whenever they are due.
func (u *UseCases) UpcomingReminders(ctx context.Context, actor domain.UserID, now time.Time) ([]domain.Reminder, error) {
	soon, err := u.reminders.Open(ctx, actor, now.AddDate(0, 0, upcomingWindowDays), 0)
	if err != nil {
		return nil, fail(StageList, err)
	}
	if len(soon) > upcomingMinimum {
		return soon, nil
	}
	next, err := u.reminders.Open(ctx, actor, time.Time{}, upcomingMinimum)
	if err != nil {
		return nil, fail(StageList, err)
	}
	return next, nil
}

func requireContact(ctx context.Context, tx Tx, actor domain.UserID, id domain.ContactID) error {
	ok, err := tx.ContactExists(ctx, actor, id)
	if err != nil {
		return &Failure{Stage: StageContact, Err: err}
	}
	if !ok {
		return domain.ErrContactNotFound
	}
	return nil
}

// CreateReminder stores a reminder about the contact for the actor.
func (u *UseCases) CreateReminder(ctx context.Context, actor domain.UserID, contact domain.ContactID, draft domain.Reminder) (domain.Reminder, error) {
	var created domain.Reminder
	err := u.store.InTx(ctx, func(tx Tx) error {
		if err := requireContact(ctx, tx, actor, contact); err != nil {
			return err
		}
		var r domain.Reminder
		r.Edit(draft)
		r.ContactID = &contact
		var err error
		created, err = tx.Create(ctx, actor, r)
		return err
	})
	if err != nil {
		return domain.Reminder{}, fail(StageCreate, err)
	}
	return created, nil
}

func (u *UseCases) UpdateReminder(ctx context.Context, actor domain.UserID, id domain.ReminderID, edit domain.Reminder) (domain.Reminder, error) {
	var updated domain.Reminder
	err := u.store.InTx(ctx, func(tx Tx) error {
		r, err := tx.Get(ctx, actor, id)
		if err != nil {
			return fail(StageRetrieve, err)
		}
		if edit.ContactID != nil {
			if err := requireContact(ctx, tx, actor, *edit.ContactID); err != nil {
				return err
			}
		}
		r.Edit(edit)
		updated, err = tx.Update(ctx, actor, r)
		return err
	})
	if err != nil {
		return domain.Reminder{}, fail(StageUpdate, err)
	}
	return updated, nil
}

// Completed is the outcome of completing or skipping a reminder.
type Completed struct {
	Reminder domain.Reminder
	Finished bool
	// RecordErr is why the timeline entry could not be written; the completion still stands.
	RecordErr error
}

// CompleteReminder completes a reminder at now and, unless skip, records it in the timeline.
func (u *UseCases) CompleteReminder(ctx context.Context, actor domain.UserID, id domain.ReminderID, skip bool, now time.Time) (Completed, error) {
	var out Completed
	err := u.store.InTx(ctx, func(tx Tx) error {
		r, err := tx.Get(ctx, actor, id)
		if err != nil {
			return fail(StageRetrieve, err)
		}
		out.Finished = r.Complete(now)
		if !skip {
			out.RecordErr = tx.RecordCompletion(ctx, actor, r.Record(now))
		}
		if out.Finished {
			if err := tx.Delete(ctx, actor, id); err != nil {
				return &Failure{Stage: StageFinish, Err: err}
			}
			return nil
		}
		out.Reminder, err = tx.Update(ctx, actor, r)
		return err
	})
	if err != nil {
		return Completed{}, fail(StageUpdate, err)
	}
	return out, nil
}

func (u *UseCases) DeleteReminder(ctx context.Context, actor domain.UserID, id domain.ReminderID) error {
	err := u.store.InTx(ctx, func(tx Tx) error {
		if _, err := tx.Get(ctx, actor, id); err != nil {
			return fail(StageRetrieve, err)
		}
		return tx.Delete(ctx, actor, id)
	})
	return fail(StageDelete, err)
}

func (u *UseCases) CompletionsOfContact(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Completion, error) {
	list, err := u.reminders.Completions(ctx, actor, contact)
	if err != nil {
		return nil, fail(StageCompletions, err)
	}
	return list, nil
}

func (u *UseCases) DeleteCompletion(ctx context.Context, actor domain.UserID, id domain.CompletionID) error {
	err := u.store.InTx(ctx, func(tx Tx) error {
		if err := tx.GetCompletion(ctx, actor, id); err != nil {
			return fail(StageGetCompletion, err)
		}
		return tx.DeleteCompletion(ctx, actor, id)
	})
	return fail(StageDeleteCompletion, err)
}
