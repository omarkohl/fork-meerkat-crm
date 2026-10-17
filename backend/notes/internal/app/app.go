// Package app holds the use cases of the notes module. It depends on ports (the interfaces
// below) and on the domain, never on GORM, Gin or huma (docs/adr/0012-module-layout.md).
package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"meerkat/notes/internal/domain"
)

const (
	defaultLimit = 25
	maxLimit     = 100
)

// Filter selects free-floating notes. From and To are days; To is inclusive.
type Filter struct {
	Search string
	From   *time.Time
	To     *time.Time
	Page   int
	Limit  int
}

type Notes interface {
	Get(ctx context.Context, actor domain.UserID, id domain.NoteID) (domain.Note, error)
	ForContact(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Note, error)
	Unassigned(ctx context.Context, actor domain.UserID, f Filter) ([]domain.Note, int64, error)
}

type Tx interface {
	Get(ctx context.Context, actor domain.UserID, id domain.NoteID) (domain.Note, error)
	ContactExists(ctx context.Context, actor domain.UserID, id domain.ContactID) (bool, error)
	Create(ctx context.Context, actor domain.UserID, n domain.Note) (domain.Note, error)
	Update(ctx context.Context, actor domain.UserID, n domain.Note) (domain.Note, error)
	Delete(ctx context.Context, actor domain.UserID, id domain.NoteID) error
}

type Store interface {
	InTx(ctx context.Context, fn func(Tx) error) error
}

// Events tells the rest of the system what happened. It must not fail the use case.
type Events interface {
	NoteCreated(ctx context.Context, actor domain.UserID, n domain.Note, now time.Time)
	NoteUpdated(ctx context.Context, actor domain.UserID, n domain.Note, now time.Time)
	NoteDeleted(ctx context.Context, actor domain.UserID, id domain.NoteID, now time.Time)
}

type Stage string

const (
	StageRetrieve Stage = "retrieve"
	StageContact  Stage = "contact"
	StageList     Stage = "list"
	StageCreate   Stage = "create"
	StageUpdate   Stage = "update"
	StageDelete   Stage = "delete"
)

// Failure wraps an infrastructure error with the stage it happened in. A missing note or
// contact is not a Failure: it is a domain error.
type Failure struct {
	Stage Stage
	Err   error
}

func (f *Failure) Error() string { return string(f.Stage) + ": " + f.Err.Error() }
func (f *Failure) Unwrap() error { return f.Err }

type UseCases struct {
	notes  Notes
	store  Store
	events Events
}

func New(notes Notes, store Store, events Events) *UseCases {
	return &UseCases{notes: notes, store: store, events: events}
}

func isDomain(err error) bool {
	return errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrContactNotFound)
}

func fail(stage Stage, err error) error {
	if isDomain(err) {
		return err
	}
	return &Failure{Stage: stage, Err: err}
}

func (u *UseCases) GetNote(ctx context.Context, actor domain.UserID, id domain.NoteID) (domain.Note, error) {
	n, err := u.notes.Get(ctx, actor, id)
	if err != nil {
		return domain.Note{}, fail(StageRetrieve, err)
	}
	return n, nil
}

func (u *UseCases) NotesOfContact(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Note, error) {
	notes, err := u.notes.ForContact(ctx, actor, contact)
	if err != nil {
		return nil, fail(StageContact, err)
	}
	return notes, nil
}

// ListUnassigned returns a page of notes without a contact. Paging is normalised here.
func (u *UseCases) ListUnassigned(ctx context.Context, actor domain.UserID, f Filter) (notes []domain.Note, total int64, page, limit int, err error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = defaultLimit
	}
	f.Limit = min(f.Limit, maxLimit)
	f.Search = strings.ToLower(strings.TrimSpace(f.Search))
	if f.To != nil {
		next := f.To.AddDate(0, 0, 1)
		f.To = &next
	}
	notes, total, err = u.notes.Unassigned(ctx, actor, f)
	if err != nil {
		return nil, 0, 0, 0, fail(StageList, err)
	}
	return notes, total, f.Page, f.Limit, nil
}

func (u *UseCases) CreateNote(ctx context.Context, actor domain.UserID, content string, date time.Time, contact *domain.ContactID, now time.Time) (domain.Note, error) {
	var created domain.Note
	err := u.store.InTx(ctx, func(tx Tx) error {
		if contact != nil {
			if err := requireContact(ctx, tx, actor, *contact); err != nil {
				return err
			}
		}
		var err error
		created, err = tx.Create(ctx, actor, domain.Note{Content: content, Date: date, ContactID: contact})
		return err
	})
	if err != nil {
		return domain.Note{}, writeFailure(StageCreate, err)
	}
	u.events.NoteCreated(ctx, actor, created, now)
	return created, nil
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

func writeFailure(stage Stage, err error) error {
	var f *Failure
	if isDomain(err) || errors.As(err, &f) {
		return err
	}
	return &Failure{Stage: stage, Err: err}
}

func (u *UseCases) UpdateNote(ctx context.Context, actor domain.UserID, id domain.NoteID, content string, date time.Time, contact *domain.ContactID, now time.Time) (domain.Note, error) {
	var updated domain.Note
	err := u.store.InTx(ctx, func(tx Tx) error {
		n, err := tx.Get(ctx, actor, id)
		if err != nil {
			return fail(StageRetrieve, err)
		}
		if contact != nil {
			if err := requireContact(ctx, tx, actor, *contact); err != nil {
				return err
			}
		}
		n.Edit(content, date, contact)
		updated, err = tx.Update(ctx, actor, n)
		return err
	})
	if err != nil {
		return domain.Note{}, writeFailure(StageUpdate, err)
	}
	u.events.NoteUpdated(ctx, actor, updated, now)
	return updated, nil
}

func (u *UseCases) DeleteNote(ctx context.Context, actor domain.UserID, id domain.NoteID, now time.Time) error {
	err := u.store.InTx(ctx, func(tx Tx) error {
		if _, err := tx.Get(ctx, actor, id); err != nil {
			return fail(StageRetrieve, err)
		}
		return tx.Delete(ctx, actor, id)
	})
	if err != nil {
		return writeFailure(StageDelete, err)
	}
	u.events.NoteDeleted(ctx, actor, id, now)
	return nil
}
