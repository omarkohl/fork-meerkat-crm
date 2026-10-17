// Package persistence implements the ports of the relationships module on top of GORM and the
// existing models. It is the only code of the module that imports gorm or models
// (docs/adr/0012-module-layout.md), and the place where every query is scoped to the actor.
package persistence

import (
	"context"
	"errors"

	"meerkat/models"
	"meerkat/relationships/internal/app"
	"meerkat/relationships/internal/domain"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

var (
	_ app.Relationships = (*Repository)(nil)
	_ app.GraphSource   = (*Repository)(nil)
	_ app.Store         = (*Repository)(nil)
)

func (r *Repository) ContactExists(ctx context.Context, actor domain.UserID, id domain.ContactID) (bool, error) {
	return contactExists(r.db.WithContext(ctx), actor, id)
}

func (r *Repository) From(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Relationship, error) {
	var found []models.Relationship
	err := r.db.WithContext(ctx).Preload("RelatedContact", func(db *gorm.DB) *gorm.DB {
		return db.Select("ID", "Firstname", "Lastname", "Gender", "Birthday").Where("user_id = ?", uint(actor))
	}).Where("user_id = ? AND contact_id = ?", uint(actor), uint(contact)).Order("id").Find(&found).Error
	return relationships(found), err
}

func (r *Repository) To(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Relationship, error) {
	var found []models.Relationship
	err := r.db.WithContext(ctx).Preload("SourceContact", func(db *gorm.DB) *gorm.DB {
		return db.Select("ID", "Firstname", "Lastname").Where("user_id = ?", uint(actor))
	}).Where("user_id = ? AND related_contact_id = ?", uint(actor), uint(contact)).Order("id").Find(&found).Error
	return relationships(found), err
}

func (r *Repository) VisibleContacts(ctx context.Context, actor domain.UserID) ([]domain.GraphContact, error) {
	var found []models.Contact
	err := r.db.WithContext(ctx).Select("id", "firstname", "lastname", "photo_thumbnail_file", "circles").
		Where("user_id = ? AND archived = ?", uint(actor), false).Find(&found).Error
	out := make([]domain.GraphContact, len(found))
	for i, c := range found {
		out[i] = domain.GraphContact{
			ID: domain.ContactID(c.ID), Firstname: c.Firstname, Lastname: c.Lastname,
			PhotoThumbnail: models.ThumbnailURL(c.ID, c.PhotoThumbnailFile), Circles: c.Circles,
		}
	}
	return out, err
}

func (r *Repository) Links(ctx context.Context, actor domain.UserID) ([]domain.GraphLink, error) {
	var found []models.Relationship
	err := r.db.WithContext(ctx).Where("user_id = ? AND related_contact_id IS NOT NULL", uint(actor)).Find(&found).Error
	var out []domain.GraphLink
	for _, l := range found {
		if l.RelatedContactID != nil {
			out = append(out, domain.GraphLink{
				ID: domain.RelationshipID(l.ID), From: domain.ContactID(l.ContactID), To: domain.ContactID(*l.RelatedContactID), Type: l.Type,
			})
		}
	}
	return out, err
}

func (r *Repository) Activities(ctx context.Context, actor domain.UserID) ([]domain.GraphActivity, error) {
	var found []models.Activity
	err := r.db.WithContext(ctx).Preload("Contacts", func(db *gorm.DB) *gorm.DB {
		return db.Select("id").Where("user_id = ?", uint(actor))
	}).Where("user_id = ?", uint(actor)).Find(&found).Error
	out := make([]domain.GraphActivity, len(found))
	for i, a := range found {
		out[i] = domain.GraphActivity{ID: a.ID, Title: a.Title}
		for _, c := range a.Contacts {
			out[i].Participants = append(out[i].Participants, domain.ContactID(c.ID))
		}
	}
	return out, err
}

func (r *Repository) InTx(ctx context.Context, fn func(app.Tx) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&txRepo{tx: tx})
	})
}

type txRepo struct{ tx *gorm.DB }

func (t *txRepo) Get(ctx context.Context, actor domain.UserID, id domain.RelationshipID) (domain.Relationship, error) {
	var m models.Relationship
	err := t.tx.WithContext(ctx).Where("user_id = ?", uint(actor)).First(&m, uint(id)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Relationship{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Relationship{}, err
	}
	return toDomain(m), nil
}

func (t *txRepo) ContactExists(ctx context.Context, actor domain.UserID, id domain.ContactID) (bool, error) {
	return contactExists(t.tx.WithContext(ctx), actor, id)
}

func (t *txRepo) Create(ctx context.Context, actor domain.UserID, r domain.Relationship) (domain.Relationship, error) {
	m := models.Relationship{UserID: uint(actor), ContactID: uint(r.ContactID)}
	fill(&m, r)
	if err := t.tx.WithContext(ctx).Create(&m).Error; err != nil {
		return domain.Relationship{}, err
	}
	return toDomain(m), nil
}

func (t *txRepo) Update(ctx context.Context, actor domain.UserID, r domain.Relationship) (domain.Relationship, error) {
	m := models.Relationship{Model: gorm.Model{ID: uint(r.ID), CreatedAt: r.CreatedAt}, UserID: uint(actor), ContactID: uint(r.ContactID)}
	fill(&m, r)
	res := t.tx.WithContext(ctx).Model(&m).Where("user_id = ?", uint(actor)).
		Select("name", "type", "gender", "birthday", "related_contact_id").Updates(&m)
	if res.Error != nil {
		return domain.Relationship{}, res.Error
	}
	if res.RowsAffected == 0 {
		return domain.Relationship{}, domain.ErrNotFound
	}
	return toDomain(m), nil
}

func (t *txRepo) Delete(ctx context.Context, actor domain.UserID, id domain.RelationshipID) error {
	return t.tx.WithContext(ctx).Where("user_id = ?", uint(actor)).Delete(&models.Relationship{}, uint(id)).Error
}

func contactExists(db *gorm.DB, actor domain.UserID, id domain.ContactID) (bool, error) {
	var n int64
	err := db.Model(&models.Contact{}).Where("user_id = ? AND id = ?", uint(actor), uint(id)).Count(&n).Error
	return n > 0, err
}

func fill(m *models.Relationship, r domain.Relationship) {
	m.Name, m.Type, m.Gender, m.Birthday = r.Name, r.Type, r.Gender, r.Birthday
	m.RelatedContactID = nil
	if r.RelatedContactID != nil {
		id := uint(*r.RelatedContactID)
		m.RelatedContactID = &id
	}
}

func relationships(found []models.Relationship) []domain.Relationship {
	out := make([]domain.Relationship, len(found))
	for i, m := range found {
		out[i] = toDomain(m)
	}
	return out
}

func toDomain(m models.Relationship) domain.Relationship {
	r := domain.Relationship{
		ID: domain.RelationshipID(m.ID), UserID: domain.UserID(m.UserID), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		ContactID: domain.ContactID(m.ContactID), Name: m.Name, Type: m.Type, Gender: m.Gender, Birthday: m.Birthday,
	}
	if m.DeletedAt.Valid {
		t := m.DeletedAt.Time
		r.DeletedAt = &t
	}
	if m.RelatedContactID != nil {
		id := domain.ContactID(*m.RelatedContactID)
		r.RelatedContactID = &id
	}
	r.RelatedContact = ref(m.RelatedContact)
	r.SourceContact = ref(m.SourceContact)
	return r
}

func ref(c *models.Contact) *domain.ContactRef {
	if c == nil {
		return nil
	}
	return &domain.ContactRef{ID: domain.ContactID(c.ID), Firstname: c.Firstname, Lastname: c.Lastname, Gender: c.Gender, Birthday: c.Birthday}
}
