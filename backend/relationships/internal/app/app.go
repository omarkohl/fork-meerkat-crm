// Package app holds the use cases of the relationships module. It depends on ports (the
// interfaces below) and on the domain, never on GORM, Gin or huma
// (docs/adr/0012-module-layout.md).
package app

import (
	"context"
	"errors"

	"meerkat/relationships/internal/domain"
)

// Relationships reads relationships. Every method is scoped to the actor.
type Relationships interface {
	// From returns the relationships that start at the contact, with the related contact.
	From(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Relationship, error)
	// To returns the relationships that point at the contact, with the source contact.
	To(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Relationship, error)
	ContactExists(ctx context.Context, actor domain.UserID, id domain.ContactID) (bool, error)
}

// GraphSource reads what the network graph is drawn from.
type GraphSource interface {
	VisibleContacts(ctx context.Context, actor domain.UserID) ([]domain.GraphContact, error)
	Links(ctx context.Context, actor domain.UserID) ([]domain.GraphLink, error)
	Activities(ctx context.Context, actor domain.UserID) ([]domain.GraphActivity, error)
}

type Tx interface {
	Get(ctx context.Context, actor domain.UserID, id domain.RelationshipID) (domain.Relationship, error)
	ContactExists(ctx context.Context, actor domain.UserID, id domain.ContactID) (bool, error)
	Create(ctx context.Context, actor domain.UserID, r domain.Relationship) (domain.Relationship, error)
	Update(ctx context.Context, actor domain.UserID, r domain.Relationship) (domain.Relationship, error)
	Delete(ctx context.Context, actor domain.UserID, id domain.RelationshipID) error
}

type Store interface {
	InTx(ctx context.Context, fn func(Tx) error) error
}

type Stage string

const (
	StageRetrieve        Stage = "retrieve"
	StageContact         Stage = "contact"
	StageRelated         Stage = "related"
	StageList            Stage = "list"
	StageIncoming        Stage = "incoming"
	StageCreate          Stage = "create"
	StageUpdate          Stage = "update"
	StageDelete          Stage = "delete"
	StageGraphContacts   Stage = "graph-contacts"
	StageGraphLinks      Stage = "graph-links"
	StageGraphActivities Stage = "graph-activities"
)

// Failure wraps an infrastructure error with the stage it happened in. A missing relationship
// or contact is not a Failure: it is a domain error.
type Failure struct {
	Stage Stage
	Err   error
}

func (f *Failure) Error() string { return string(f.Stage) + ": " + f.Err.Error() }
func (f *Failure) Unwrap() error { return f.Err }

type UseCases struct {
	relationships Relationships
	graph         GraphSource
	store         Store
}

func New(relationships Relationships, graph GraphSource, store Store) *UseCases {
	return &UseCases{relationships: relationships, graph: graph, store: store}
}

func fail(stage Stage, err error) error {
	if err == nil {
		return nil
	}
	var f *Failure
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrContactNotFound) ||
		errors.Is(err, domain.ErrRelatedNotFound) || errors.As(err, &f) {
		return err
	}
	return &Failure{Stage: stage, Err: err}
}

func (u *UseCases) requireContact(ctx context.Context, actor domain.UserID, id domain.ContactID) error {
	ok, err := u.relationships.ContactExists(ctx, actor, id)
	if err != nil {
		return &Failure{Stage: StageContact, Err: err}
	}
	if !ok {
		return domain.ErrContactNotFound
	}
	return nil
}

func (u *UseCases) RelationshipsOf(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Relationship, error) {
	if err := u.requireContact(ctx, actor, contact); err != nil {
		return nil, err
	}
	list, err := u.relationships.From(ctx, actor, contact)
	return list, fail(StageList, err)
}

func (u *UseCases) IncomingRelationshipsOf(ctx context.Context, actor domain.UserID, contact domain.ContactID) ([]domain.Relationship, error) {
	if err := u.requireContact(ctx, actor, contact); err != nil {
		return nil, err
	}
	list, err := u.relationships.To(ctx, actor, contact)
	return list, fail(StageIncoming, err)
}

func requireTxContact(ctx context.Context, tx Tx, actor domain.UserID, id domain.ContactID, stage Stage, missing error) error {
	ok, err := tx.ContactExists(ctx, actor, id)
	if err != nil {
		return &Failure{Stage: stage, Err: err}
	}
	if !ok {
		return missing
	}
	return nil
}

func (u *UseCases) CreateRelationship(ctx context.Context, actor domain.UserID, contact domain.ContactID, draft domain.Relationship) (domain.Relationship, error) {
	var created domain.Relationship
	err := u.store.InTx(ctx, func(tx Tx) error {
		if err := requireTxContact(ctx, tx, actor, contact, StageContact, domain.ErrContactNotFound); err != nil {
			return err
		}
		if draft.RelatedContactID != nil {
			if err := requireTxContact(ctx, tx, actor, *draft.RelatedContactID, StageRelated, domain.ErrRelatedNotFound); err != nil {
				return err
			}
		}
		draft.ContactID = contact
		var err error
		created, err = tx.Create(ctx, actor, draft)
		return err
	})
	if err != nil {
		return domain.Relationship{}, fail(StageCreate, err)
	}
	return created, nil
}

func (u *UseCases) UpdateRelationship(ctx context.Context, actor domain.UserID, id domain.RelationshipID, edit domain.Relationship) (domain.Relationship, error) {
	var updated domain.Relationship
	err := u.store.InTx(ctx, func(tx Tx) error {
		r, err := tx.Get(ctx, actor, id)
		if err != nil {
			return fail(StageRetrieve, err)
		}
		if edit.RelatedContactID != nil {
			if err := requireTxContact(ctx, tx, actor, *edit.RelatedContactID, StageRelated, domain.ErrRelatedNotFound); err != nil {
				return err
			}
		}
		r.Edit(edit)
		updated, err = tx.Update(ctx, actor, r)
		return err
	})
	if err != nil {
		return domain.Relationship{}, fail(StageUpdate, err)
	}
	return updated, nil
}

func (u *UseCases) DeleteRelationship(ctx context.Context, actor domain.UserID, id domain.RelationshipID) error {
	err := u.store.InTx(ctx, func(tx Tx) error {
		if _, err := tx.Get(ctx, actor, id); err != nil {
			return fail(StageRetrieve, err)
		}
		return tx.Delete(ctx, actor, id)
	})
	return fail(StageDelete, err)
}

// NetworkGraph returns the graph of the actor's visible contacts, relationships and activities.
func (u *UseCases) NetworkGraph(ctx context.Context, actor domain.UserID) (domain.Graph, error) {
	contacts, err := u.graph.VisibleContacts(ctx, actor)
	if err != nil {
		return domain.Graph{}, fail(StageGraphContacts, err)
	}
	links, err := u.graph.Links(ctx, actor)
	if err != nil {
		return domain.Graph{}, fail(StageGraphLinks, err)
	}
	activities, err := u.graph.Activities(ctx, actor)
	if err != nil {
		return domain.Graph{}, fail(StageGraphActivities, err)
	}
	return domain.BuildGraph(contacts, links, activities), nil
}
