// Package persistence implements the ports of the reminders module on top of GORM and the
// existing models. It is the only code of the module that imports gorm or models
// (docs/adr/0012-module-layout.md), and the place where every query is scoped to the actor.
package persistence

import (
	"context"
	"errors"
	"time"

	"meerkat/models"
	"meerkat/reminders/internal/app"
	"meerkat/reminders/internal/domain"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

var (
	_ app.Reminders = (*Repository)(nil)
	_ app.Store     = (*Repository)(nil)
)

func (r *Repository) Get(ctx context.Context, actor domain.UserID, id domain.ReminderID) (domain.Reminder, error) {
	return get(r.db.WithContext(ctx), actor, id)
}

func (r *Repository) All(ctx context.Context, actor domain.UserID) ([]domain.Reminder, error) {
	var found []models.Reminder
	err := r.db.WithContext(ctx).Where("user_id = ?", uint(actor)).Order("remind_at ASC").Find(&found).Error
	return reminders(found), err
}

func (r *Repository) ForContact(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Reminder, error) {
	db := r.db.WithContext(ctx)
	if ok, err := contactExists(db, actor, contact); err != nil {
		return nil, err
	} else if !ok {
		return nil, domain.ErrContactNotFound
	}
	var found []models.Reminder
	err := db.Where("user_id = ? AND contact_id = ?", uint(actor), uint(contact)).Order("id").Find(&found).Error
	return reminders(found), err
}

func (r *Repository) Open(ctx context.Context, actor domain.UserID, until time.Time, limit int) ([]domain.Reminder, error) {
	q := r.db.WithContext(ctx).Where("user_id = ? AND completed = ?", uint(actor), false)
	if !until.IsZero() {
		q = q.Where("remind_at <= ?", until)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	var found []models.Reminder
	err := q.Order("remind_at ASC").Find(&found).Error
	return reminders(found), err
}

func (r *Repository) Completions(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Completion, error) {
	db := r.db.WithContext(ctx)
	if ok, err := contactExists(db, actor, contact); err != nil {
		return nil, err
	} else if !ok {
		return nil, domain.ErrContactNotFound
	}
	var found []models.ReminderCompletion
	err := db.Where("user_id = ? AND contact_id = ?", uint(actor), uint(contact)).Order("completed_at DESC").Find(&found).Error
	out := make([]domain.Completion, len(found))
	for i, m := range found {
		out[i] = completionToDomain(m)
	}
	return out, err
}

func (r *Repository) InTx(ctx context.Context, fn func(app.Tx) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&txRepo{tx: tx})
	})
}

type txRepo struct{ tx *gorm.DB }

func (t *txRepo) Get(ctx context.Context, actor domain.UserID, id domain.ReminderID) (domain.Reminder, error) {
	return get(t.tx.WithContext(ctx), actor, id)
}

func (t *txRepo) ContactExists(ctx context.Context, actor domain.UserID, id domain.ContactID) (bool, error) {
	return contactExists(t.tx.WithContext(ctx), actor, id)
}

func (t *txRepo) Create(ctx context.Context, actor domain.UserID, r domain.Reminder) (domain.Reminder, error) {
	m := models.Reminder{UserID: uint(actor)}
	fill(&m, r)
	db := t.tx.WithContext(ctx)
	if err := db.Create(&m).Error; err != nil {
		return domain.Reminder{}, err
	}
	return get(db, actor, domain.ReminderID(m.ID))
}

func (t *txRepo) Update(ctx context.Context, actor domain.UserID, r domain.Reminder) (domain.Reminder, error) {
	db := t.tx.WithContext(ctx)
	m := models.Reminder{Model: gorm.Model{ID: uint(r.ID), CreatedAt: r.CreatedAt}, UserID: uint(actor)}
	fill(&m, r)
	m.Completed, m.EmailSent, m.LastSent = r.Completed, r.EmailSent, r.LastSent
	res := db.Model(&m).Where("user_id = ?", uint(actor)).
		Select("message", "by_mail", "remind_at", "recurrence", "reoccur_from_completion", "completed", "email_sent", "last_sent", "contact_id").
		Updates(&m)
	if res.Error != nil {
		return domain.Reminder{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Reminder{}, domain.ErrNotFound
	}
	return get(db, actor, r.ID)
}

func (t *txRepo) Delete(ctx context.Context, actor domain.UserID, id domain.ReminderID) error {
	return t.tx.WithContext(ctx).Where("user_id = ?", uint(actor)).Delete(&models.Reminder{}, uint(id)).Error
}

func (t *txRepo) RecordCompletion(ctx context.Context, actor domain.UserID, c domain.Completion) error {
	m := models.ReminderCompletion{UserID: uint(actor), ContactID: uint(c.ContactID), Message: c.Message, CompletedAt: c.CompletedAt}
	if c.ReminderID != nil {
		id := uint(*c.ReminderID)
		m.ReminderID = &id
	}
	return t.tx.WithContext(ctx).Create(&m).Error
}

func (t *txRepo) GetCompletion(ctx context.Context, actor domain.UserID, id domain.CompletionID) error {
	var m models.ReminderCompletion
	err := t.tx.WithContext(ctx).Where("user_id = ?", uint(actor)).First(&m, uint(id)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrCompletionNotFound
	}
	return err
}

func (t *txRepo) DeleteCompletion(ctx context.Context, actor domain.UserID, id domain.CompletionID) error {
	return t.tx.WithContext(ctx).Where("user_id = ?", uint(actor)).Delete(&models.ReminderCompletion{}, uint(id)).Error
}

func get(db *gorm.DB, actor domain.UserID, id domain.ReminderID) (domain.Reminder, error) {
	var m models.Reminder
	err := db.Where("user_id = ?", uint(actor)).First(&m, uint(id)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Reminder{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Reminder{}, err
	}
	return toDomain(m), nil
}

func contactExists(db *gorm.DB, actor domain.UserID, id domain.ContactID) (bool, error) {
	var n int64
	err := db.Model(&models.Contact{}).Where("user_id = ? AND id = ?", uint(actor), uint(id)).Count(&n).Error
	return n > 0, err
}

func fill(m *models.Reminder, r domain.Reminder) {
	m.Message, m.ByMail, m.RemindAt, m.Recurrence, m.ReoccurFromCompletion = r.Message, r.ByMail, r.RemindAt, r.Recurrence, r.ReoccurFromCompletion
	if r.ContactID != nil {
		id := uint(*r.ContactID)
		m.ContactID = &id
	}
}

func reminders(found []models.Reminder) []domain.Reminder {
	out := make([]domain.Reminder, len(found))
	for i, m := range found {
		out[i] = toDomain(m)
	}
	return out
}

func deletedAt(d gorm.DeletedAt) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

func toDomain(m models.Reminder) domain.Reminder {
	r := domain.Reminder{
		ID: domain.ReminderID(m.ID), UserID: domain.UserID(m.UserID), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		DeletedAt: deletedAt(m.DeletedAt), Message: m.Message, ByMail: m.ByMail, RemindAt: m.RemindAt,
		Recurrence: m.Recurrence, ReoccurFromCompletion: m.ReoccurFromCompletion,
		Completed: m.Completed, EmailSent: m.EmailSent, LastSent: m.LastSent,
	}
	if m.ContactID != nil {
		id := domain.ContactID(*m.ContactID)
		r.ContactID = &id
	}
	return r
}

func completionToDomain(m models.ReminderCompletion) domain.Completion {
	c := domain.Completion{
		ID: domain.CompletionID(m.ID), UserID: domain.UserID(m.UserID), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		DeletedAt: deletedAt(m.DeletedAt), ContactID: domain.ContactID(m.ContactID), Message: m.Message, CompletedAt: m.CompletedAt,
	}
	if m.ReminderID != nil {
		id := domain.ReminderID(*m.ReminderID)
		c.ReminderID = &id
	}
	return c
}
