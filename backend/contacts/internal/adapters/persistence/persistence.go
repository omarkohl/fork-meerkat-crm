// Package persistence implements the ports of the contacts module on top of GORM and the
// existing models. It is the only code of the module that imports gorm or models
// (docs/adr/0012-module-layout.md), and the place where every query is scoped to the actor.
package persistence

import (
	"context"
	"errors"
	"slices"
	"time"

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
	return first(r.db.WithContext(ctx), actor, id)
}

var showable = []string{"ID", "firstname", "lastname", "nickname", "gender", "email", "phone", "birthday", "address", "how_we_met", "food_preference", "work_information", "contact_information", "circles", "photo", "custom_fields", "archived", "emails", "phones", "addresses", "urls", "impps", "prefix", "middle_name", "suffix", "organization", "department", "job_title", "role", "anniversary", "note"}

func first(db *gorm.DB, actor domain.UserID, id domain.ContactID) (domain.Contact, error) {
	var m models.Contact
	err := db.Where("user_id = ?", uint(actor)).First(&m, uint(id)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Contact{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Contact{}, err
	}
	return toDomain(m), nil
}

// Show implements app.Contacts.
func (r *Repository) Show(ctx context.Context, actor domain.UserID, id domain.ContactID, fields []string) (domain.Contact, error) {
	var columns []string
	for _, f := range fields {
		if slices.Contains(showable, f) {
			columns = append(columns, f)
		}
	}
	q := r.db.WithContext(ctx).Where("user_id = ?", uint(actor))
	if len(columns) > 0 {
		q = q.Select(columns)
	} else {
		q = q.Omit("photo_thumbnail").Preload("Relationships", "relationships.user_id = ?", uint(actor)).
			Preload("Notes", "notes.user_id = ?", uint(actor)).
			Preload("Activities", "activities.user_id = ?", uint(actor)).
			Preload("Reminders", "reminders.user_id = ?", uint(actor))
	}
	var m models.Contact
	err := q.First(&m, uint(id)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Contact{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Contact{}, err
	}
	c := toDomain(m)
	if len(columns) == 0 && c.Relationships == nil {
		c.Relationships = []domain.Relationship{}
	}
	return c, nil
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

func (t *txRepo) Get(ctx context.Context, actor domain.UserID, id domain.ContactID) (domain.Contact, error) {
	return first(t.tx.WithContext(ctx), actor, id)
}

func (t *txRepo) Create(ctx context.Context, actor domain.UserID, c domain.Contact) (domain.Contact, error) {
	m := models.Contact{UserID: uint(actor)}
	fill(&m, c)
	if err := t.tx.WithContext(ctx).Create(&m).Error; err != nil {
		return domain.Contact{}, err
	}
	return toDomain(m), nil
}

func (t *txRepo) Update(ctx context.Context, actor domain.UserID, c domain.Contact) (domain.Contact, error) {
	var m models.Contact
	err := t.tx.WithContext(ctx).Where("user_id = ?", uint(actor)).First(&m, uint(c.ID)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Contact{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Contact{}, err
	}
	fill(&m, c)
	if err := t.tx.WithContext(ctx).Save(&m).Error; err != nil {
		return domain.Contact{}, err
	}
	return toDomain(m), nil
}

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

// fill copies the fields a user edits onto m; identity, photo and archive state are not touched.
func fill(m *models.Contact, c domain.Contact) {
	m.Firstname, m.Lastname, m.Nickname, m.Gender = c.Firstname, c.Lastname, c.Nickname, c.Gender
	m.Email, m.Phone, m.Birthday = c.Email, c.Phone, c.Birthday
	m.Address, m.HowWeMet, m.FoodPreference = c.Address, c.HowWeMet, c.FoodPreference
	m.WorkInformation, m.ContactInformation = c.WorkInformation, c.ContactInformation
	m.Circles, m.CustomFields = c.Circles, c.CustomFields
	m.Prefix, m.MiddleName, m.Suffix = c.Prefix, c.MiddleName, c.Suffix
	m.Organization, m.Department, m.JobTitle, m.Role = c.Organization, c.Department, c.JobTitle, c.Role
	m.Anniversary, m.Note = c.Anniversary, c.Note
	m.Emails, m.Phones, m.URLs, m.IMPPs, m.Addresses = nil, nil, nil, nil, nil
	for _, e := range c.Emails {
		m.Emails = append(m.Emails, models.ContactEmail{Type: e.Type, Value: e.Value})
	}
	for _, p := range c.Phones {
		m.Phones = append(m.Phones, models.ContactPhone{Type: p.Type, Value: p.Value})
	}
	for _, u := range c.URLs {
		m.URLs = append(m.URLs, models.ContactURL{Type: u.Type, Value: u.Value})
	}
	for _, i := range c.IMPPs {
		m.IMPPs = append(m.IMPPs, models.ContactIMPP{Type: i.Type, Value: i.Value})
	}
	for _, a := range c.Addresses {
		m.Addresses = append(m.Addresses, models.ContactAddress{Type: a.Type, Street: a.Street, City: a.City, Region: a.Region, Postal: a.Postal, Country: a.Country})
	}
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
	for _, r := range m.Relationships {
		rel := domain.Relationship{
			ID: r.ID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ContactID: domain.ContactID(r.ContactID),
			Name: r.Name, Type: r.Type, Birthday: r.Birthday, Gender: r.Gender,
		}
		if r.DeletedAt.Valid {
			t := r.DeletedAt.Time
			rel.DeletedAt = &t
		}
		if r.RelatedContactID != nil {
			id := domain.ContactID(*r.RelatedContactID)
			rel.RelatedContactID = &id
		}
		c.Relationships = append(c.Relationships, rel)
	}
	for _, n := range m.Notes {
		note := domain.Note{ID: n.ID, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt, DeletedAt: deletedAt(n.DeletedAt), Content: n.Content, Date: n.Date}
		note.ContactID = contactID(n.ContactID)
		c.Notes = append(c.Notes, note)
	}
	for _, a := range m.Activities {
		c.Activities = append(c.Activities, domain.Activity{
			ID: a.ID, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, DeletedAt: deletedAt(a.DeletedAt),
			Title: a.Title, Description: a.Description, Location: a.Location, Date: a.Date,
		})
	}
	for _, r := range m.Reminders {
		c.Reminders = append(c.Reminders, domain.Reminder{
			ID: r.ID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, DeletedAt: deletedAt(r.DeletedAt),
			Message: r.Message, ByMail: r.ByMail, RemindAt: r.RemindAt, Recurrence: r.Recurrence,
			ReoccurFromCompletion: r.ReoccurFromCompletion, Completed: r.Completed, EmailSent: r.EmailSent,
			LastSent: r.LastSent, ContactID: contactID(r.ContactID),
		})
	}
	return c
}

func deletedAt(d gorm.DeletedAt) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

func contactID(id *uint) *domain.ContactID {
	if id == nil {
		return nil
	}
	v := domain.ContactID(*id)
	return &v
}
