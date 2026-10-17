// Package persistence implements the ports of the contacts module on top of GORM and the
// existing models. It is the only code of the module that imports gorm or models
// (docs/adr/0012-module-layout.md), and the place where every query is scoped to the actor.
package persistence

import (
	"context"
	"errors"

	"meerkat/contacts/internal/app"
	"meerkat/contacts/internal/domain"
	"meerkat/models"

	"gorm.io/gorm"
)

// Repository implements app.Contacts and app.Store.
type Repository struct{ db *gorm.DB }

// New returns a repository on db.
func New(db *gorm.DB) *Repository { return &Repository{db: db} }

var (
	_ app.Contacts = (*Repository)(nil)
	_ app.Store    = (*Repository)(nil)
)

// Get implements app.Contacts.
func (r *Repository) Get(ctx context.Context, actor domain.UserID, id domain.ContactID) (domain.Contact, error) {
	var m models.Contact
	err := r.db.WithContext(ctx).Where("user_id = ?", uint(actor)).First(&m, uint(id)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Contact{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Contact{}, err
	}
	return toDomain(m), nil
}

// Circles implements app.Contacts.
func (r *Repository) Circles(ctx context.Context, actor domain.UserID) ([]string, error) {
	var names []string
	// Raw SQL: the circles are a JSON array column.
	err := r.db.WithContext(ctx).Raw(`SELECT DISTINCT json_each.value AS circle
	               FROM contacts, json_each(contacts.circles)
	               WHERE contacts.user_id = ? AND contacts.deleted_at IS NULL`, uint(actor)).Scan(&names).Error
	return names, err
}

// InTx implements app.Store.
func (r *Repository) InTx(ctx context.Context, fn func(app.Tx) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&txRepo{tx: tx})
	})
}

type txRepo struct{ tx *gorm.DB }

func (t *txRepo) DeleteReminders(ctx context.Context, actor domain.UserID, id domain.ContactID) error {
	return t.tx.WithContext(ctx).Where("contact_id = ? AND user_id = ?", uint(id), uint(actor)).Delete(&models.Reminder{}).Error
}

func (t *txRepo) SetArchived(ctx context.Context, actor domain.UserID, id domain.ContactID, archived bool) error {
	return t.tx.WithContext(ctx).Model(&models.Contact{}).
		Where("id = ? AND user_id = ?", uint(id), uint(actor)).Update("archived", archived).Error
}

func (t *txRepo) MoveToTrash(ctx context.Context, actor domain.UserID, id domain.ContactID) error {
	return models.TrashContact(t.tx.WithContext(ctx), uint(actor), uint(id))
}

func toDomain(m models.Contact) domain.Contact {
	c := domain.Contact{
		ID:        domain.ContactID(m.ID),
		UserID:    domain.UserID(m.UserID),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,

		Firstname: m.Firstname, Lastname: m.Lastname, Nickname: m.Nickname, Gender: m.Gender,
		Email: m.Email, Phone: m.Phone, Birthday: m.Birthday, Photo: m.Photo,

		Address: m.Address, HowWeMet: m.HowWeMet, FoodPreference: m.FoodPreference,
		WorkInformation: m.WorkInformation, ContactInformation: m.ContactInformation,
		Circles: m.Circles,

		Prefix: m.Prefix, MiddleName: m.MiddleName, Suffix: m.Suffix,
		Organization: m.Organization, Department: m.Department, JobTitle: m.JobTitle, Role: m.Role,
		Anniversary: m.Anniversary, Note: m.Note,
		CustomFields: m.CustomFields,
		Archived:     m.Archived,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		c.DeletedAt = &t
	}
	for _, e := range m.Emails {
		c.Emails = append(c.Emails, domain.Value{Type: e.Type, Value: e.Value})
	}
	for _, p := range m.Phones {
		c.Phones = append(c.Phones, domain.Value{Type: p.Type, Value: p.Value})
	}
	for _, u := range m.URLs {
		c.URLs = append(c.URLs, domain.Value{Type: u.Type, Value: u.Value})
	}
	for _, i := range m.IMPPs {
		c.IMPPs = append(c.IMPPs, domain.Value{Type: i.Type, Value: i.Value})
	}
	for _, a := range m.Addresses {
		c.Addresses = append(c.Addresses, domain.Address{Type: a.Type, Street: a.Street, City: a.City, Region: a.Region, Postal: a.Postal, Country: a.Country})
	}
	return c
}
