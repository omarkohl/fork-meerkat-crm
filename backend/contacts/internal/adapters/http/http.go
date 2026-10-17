// Package httpadapter exposes the contacts use cases as huma operations. It maps HTTP to the
// use cases and their results and errors back to the responses the API always sent
// (docs/adr/0012-module-layout.md).
package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"meerkat/api"
	"meerkat/contacts/internal/app"
	"meerkat/contacts/internal/domain"
	apperrors "meerkat/errors"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
)

// Register adds the contact operations to a, on group (the authenticated group).
func Register(a huma.API, group gin.IRoutes, uc *app.UseCases) {
	h := &handlers{uc: uc}
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "archive-contact", Method: http.MethodPost, Path: "/contacts/{id}/archive",
		Summary: "Archive a contact and delete its reminders", Tags: []string{"contacts"},
	}, h.archive)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "unarchive-contact", Method: http.MethodPost, Path: "/contacts/{id}/unarchive",
		Summary: "Restore an archived contact", Tags: []string{"contacts"},
	}, h.unarchive)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "delete-contact", Method: http.MethodDelete, Path: "/contacts/{id}",
		Summary: "Move a contact and its notes, reminders and relationships to the trash", Tags: []string{"contacts"},
	}, h.delete)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "list-circles", Method: http.MethodGet, Path: "/contacts/circles",
		Summary: "The distinct circle names of all contacts", Tags: []string{"contacts"},
	}, h.circles)
}

type handlers struct{ uc *app.UseCases }

// contactPath is the {id} of /contacts/{id}/....
type contactPath struct {
	ID uint `path:"id" minimum:"1" doc:"The contact's ID"`
}

// ContactOutput is the response of the operations that return the changed contact.
type ContactOutput struct {
	Body ContactResponse
}

// MessageOutput is the response of the operations that only confirm.
type MessageOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

// CirclesOutput is the response of GET /contacts/circles: a JSON array of names.
type CirclesOutput struct {
	Body []string
}

func (h *handlers) archive(ctx context.Context, in *contactPath) (*ContactOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	c, err := h.uc.ArchiveContact(ctx, actor, domain.ContactID(in.ID))
	if err != nil {
		return nil, toAppError(err, in.ID)
	}
	return &ContactOutput{Body: contactResponse(c)}, nil
}

func (h *handlers) unarchive(ctx context.Context, in *contactPath) (*ContactOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	c, err := h.uc.UnarchiveContact(ctx, actor, domain.ContactID(in.ID))
	if err != nil {
		return nil, toAppError(err, in.ID)
	}
	return &ContactOutput{Body: contactResponse(c)}, nil
}

func (h *handlers) delete(ctx context.Context, in *contactPath) (*MessageOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.uc.DeleteContact(ctx, actor, domain.ContactID(in.ID), api.Now(ctx)); err != nil {
		return nil, toAppError(err, in.ID)
	}
	out := &MessageOutput{}
	out.Body.Message = "Contact deleted"
	return out, nil
}

func (h *handlers) circles(ctx context.Context, _ *struct{}) (*CirclesOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	circles, err := h.uc.ListCircles(ctx, actor)
	if err != nil {
		return nil, toAppError(err, 0)
	}
	return &CirclesOutput{Body: circles}, nil
}

func actorOf(ctx context.Context) (domain.UserID, error) {
	id, ok := api.UserID(ctx)
	if !ok {
		return 0, apperrors.ErrUnauthorized("Authentication required")
	}
	return domain.UserID(id), nil
}

// toAppError words a use-case error the way the Gin handlers did.
func toAppError(err error, id uint) error {
	if errors.Is(err, domain.ErrNotFound) {
		return apperrors.ErrNotFound("Contact").WithDetails("id", strconv.FormatUint(uint64(id), 10))
	}
	var f *app.Failure
	if errors.As(err, &f) {
		switch f.Stage {
		case app.StageRetrieve:
			return apperrors.ErrDatabase("Failed to retrieve contact").WithError(f.Err)
		case app.StageArchive:
			return apperrors.ErrDatabase("Failed to archive contact").WithError(f.Err)
		case app.StageUnarchive:
			return apperrors.ErrDatabase("Failed to unarchive contact").WithError(f.Err)
		case app.StageDelete:
			return apperrors.ErrDatabase("Failed to delete contact and associated data").WithError(f.Err)
		case app.StageCircles:
			return apperrors.ErrDatabase("Failed to retrieve circles").WithError(f.Err)
		}
	}
	return apperrors.ErrInternal("Unexpected error").WithError(err)
}
