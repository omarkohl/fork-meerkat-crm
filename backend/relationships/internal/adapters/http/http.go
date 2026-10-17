// Package httpadapter exposes the relationships use cases as huma operations. It maps HTTP to
// the use cases and their results and errors back to the responses the API always sent
// (docs/adr/0012-module-layout.md).
package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"meerkat/api"
	apperrors "meerkat/errors"
	"meerkat/relationships/internal/app"
	"meerkat/relationships/internal/domain"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
)

// Register adds the relationship and graph operations to a, on group (the authenticated group).
func Register(a huma.API, group gin.IRoutes, uc *app.UseCases) {
	h := &handlers{uc: uc}
	tags := []string{"relationships"}
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "list-relationships", Method: http.MethodGet, Path: "/contacts/{id}/relationships",
		Summary: "The relationships that start at a contact", Tags: tags,
	}, h.from)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "list-incoming-relationships", Method: http.MethodGet, Path: "/contacts/{id}/incoming-relationships",
		Summary: "The relationships that point at a contact", Tags: tags,
	}, h.to)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "create-relationship", Method: http.MethodPost, Path: "/contacts/{id}/relationships", DefaultStatus: http.StatusCreated,
		Summary: "Add a relationship to a contact", Tags: tags,
	}, h.create)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "update-relationship", Method: http.MethodPut, Path: "/contacts/{id}/relationships/{rid}",
		Summary: "Replace the fields of a relationship; related_contact_id left out unlinks it", Tags: tags,
	}, h.update)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "delete-relationship", Method: http.MethodDelete, Path: "/contacts/{id}/relationships/{rid}",
		Summary: "Delete a relationship", Tags: tags,
	}, h.delete)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "network-graph", Method: http.MethodGet, Path: "/graph",
		Summary: "The network of contacts, relationships and shared activities", Tags: []string{"graph"},
	}, h.graph)
}

type handlers struct{ uc *app.UseCases }

type RelationshipBody struct {
	Name             string `json:"name" validate:"required,min=1,max=100"`
	Type             string `json:"type" validate:"required,min=1,max=50"`
	Gender           string `json:"gender" validate:"omitempty,oneof=male female other prefer_not_to_say"`
	Birthday         string `json:"birthday" validate:"omitempty,birthday"`
	RelatedContactID *uint  `json:"related_contact_id,omitempty"`
}

type RelatedContactResponse struct {
	ID        uint   `json:"ID"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Gender    string `json:"gender"`
	Birthday  string `json:"birthday"`
}

type SourceContactResponse struct {
	ID        uint   `json:"ID"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
}

type RelationshipResponse struct {
	ID               uint                    `json:"ID"`
	CreatedAt        time.Time               `json:"CreatedAt"`
	UpdatedAt        time.Time               `json:"UpdatedAt"`
	DeletedAt        *time.Time              `json:"DeletedAt" nullable:"true"`
	Name             string                  `json:"name"`
	Type             string                  `json:"type"`
	Gender           string                  `json:"gender"`
	Birthday         string                  `json:"birthday"`
	ContactID        uint                    `json:"contact_id"`
	RelatedContactID *uint                   `json:"related_contact_id" nullable:"true"`
	RelatedContact   *RelatedContactResponse `json:"related_contact,omitempty"`
	SourceContact    *SourceContactResponse  `json:"source_contact,omitempty"`
}

type GraphNodeResponse struct {
	ID             string   `json:"id" doc:"c-{contact ID} or a-{activity ID}"`
	Type           string   `json:"type" enum:"contact,activity"`
	Label          string   `json:"label"`
	PhotoThumbnail string   `json:"photo_thumbnail,omitempty"`
	Circles        []string `json:"circles,omitempty"`
}

type GraphEdgeResponse struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type" enum:"relationship,activity"`
	Label  string `json:"label" doc:"The relationship type or the activity title"`
}

type contactPath struct {
	ID uint `path:"id" minimum:"1" doc:"The contact's ID"`
}

type CreateInput struct {
	ID   uint `path:"id" minimum:"1" doc:"The contact's ID"`
	Body RelationshipBody
}

func (i *CreateInput) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	return api.ValidateBody(prefix, &i.Body)
}

type UpdateInput struct {
	ID   uint `path:"id" minimum:"1" doc:"The contact's ID; not checked against the relationship"`
	RID  uint `path:"rid" minimum:"1" doc:"The relationship's ID"`
	Body RelationshipBody
}

func (i *UpdateInput) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	return api.ValidateBody(prefix, &i.Body)
}

type DeleteInput struct {
	ID  uint `path:"id" minimum:"1" doc:"The contact's ID; not checked against the relationship"`
	RID uint `path:"rid" minimum:"1" doc:"The relationship's ID"`
}

type RelationshipListOutput struct {
	Body struct {
		Relationships []RelationshipResponse `json:"relationships"`
	}
}

type RelationshipIncomingOutput struct {
	Body struct {
		IncomingRelationships []RelationshipResponse `json:"incoming_relationships"`
	}
}

type RelationshipCreatedOutput struct {
	Body struct {
		Relationship RelationshipResponse `json:"relationship"`
	}
}

type RelationshipOutput struct{ Body RelationshipResponse }

type RelationshipMessageOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

type GraphOutput struct {
	Body struct {
		Nodes []GraphNodeResponse `json:"nodes"`
		Edges []GraphEdgeResponse `json:"edges"`
	}
}

func (h *handlers) from(ctx context.Context, in *contactPath) (*RelationshipListOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	list, err := h.uc.RelationshipsOf(ctx, actor, domain.ContactID(in.ID))
	if err != nil {
		return nil, toAppError(err, 0, in.ID, 0)
	}
	out := &RelationshipListOutput{}
	out.Body.Relationships = responses(list)
	return out, nil
}

func (h *handlers) to(ctx context.Context, in *contactPath) (*RelationshipIncomingOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	list, err := h.uc.IncomingRelationshipsOf(ctx, actor, domain.ContactID(in.ID))
	if err != nil {
		return nil, toAppError(err, 0, in.ID, 0)
	}
	out := &RelationshipIncomingOutput{}
	out.Body.IncomingRelationships = responses(list)
	return out, nil
}

func (h *handlers) create(ctx context.Context, in *CreateInput) (*RelationshipCreatedOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.uc.CreateRelationship(ctx, actor, domain.ContactID(in.ID), in.Body.draft())
	if err != nil {
		return nil, toAppError(err, 0, in.ID, relatedOf(in.Body))
	}
	out := &RelationshipCreatedOutput{}
	out.Body.Relationship = newResponse(r)
	return out, nil
}

func (h *handlers) update(ctx context.Context, in *UpdateInput) (*RelationshipOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.uc.UpdateRelationship(ctx, actor, domain.RelationshipID(in.RID), in.Body.draft())
	if err != nil {
		return nil, toAppError(err, in.RID, 0, relatedOf(in.Body))
	}
	return &RelationshipOutput{Body: newResponse(r)}, nil
}

func (h *handlers) delete(ctx context.Context, in *DeleteInput) (*RelationshipMessageOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.uc.DeleteRelationship(ctx, actor, domain.RelationshipID(in.RID)); err != nil {
		return nil, toAppError(err, in.RID, 0, 0)
	}
	out := &RelationshipMessageOutput{}
	out.Body.Message = "Relationship deleted"
	return out, nil
}

func (h *handlers) graph(ctx context.Context, _ *struct{}) (*GraphOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	g, err := h.uc.NetworkGraph(ctx, actor)
	if err != nil {
		return nil, toAppError(err, 0, 0, 0)
	}
	out := &GraphOutput{}
	out.Body.Nodes = make([]GraphNodeResponse, len(g.Nodes))
	for i, n := range g.Nodes {
		out.Body.Nodes[i] = GraphNodeResponse{ID: n.ID, Type: n.Type, Label: n.Label, PhotoThumbnail: n.PhotoThumbnail, Circles: n.Circles}
	}
	out.Body.Edges = make([]GraphEdgeResponse, len(g.Edges))
	for i, e := range g.Edges {
		out.Body.Edges[i] = GraphEdgeResponse{ID: e.ID, Source: e.Source, Target: e.Target, Type: e.Type, Label: e.Label}
	}
	return out, nil
}

func (b RelationshipBody) draft() domain.Relationship {
	r := domain.Relationship{Name: b.Name, Type: b.Type, Gender: b.Gender, Birthday: b.Birthday}
	if b.RelatedContactID != nil {
		id := domain.ContactID(*b.RelatedContactID)
		r.RelatedContactID = &id
	}
	return r
}

func relatedOf(b RelationshipBody) uint {
	if b.RelatedContactID == nil {
		return 0
	}
	return *b.RelatedContactID
}

func responses(list []domain.Relationship) []RelationshipResponse {
	out := make([]RelationshipResponse, len(list))
	for i, r := range list {
		out[i] = newResponse(r)
	}
	return out
}

func newResponse(r domain.Relationship) RelationshipResponse {
	out := RelationshipResponse{
		ID: uint(r.ID), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, DeletedAt: r.DeletedAt,
		Name: r.Name, Type: r.Type, Gender: r.Gender, Birthday: r.Birthday, ContactID: uint(r.ContactID),
	}
	if r.RelatedContactID != nil {
		id := uint(*r.RelatedContactID)
		out.RelatedContactID = &id
	}
	if c := r.RelatedContact; c != nil {
		out.RelatedContact = &RelatedContactResponse{ID: uint(c.ID), Firstname: c.Firstname, Lastname: c.Lastname, Gender: c.Gender, Birthday: c.Birthday}
	}
	if c := r.SourceContact; c != nil {
		out.SourceContact = &SourceContactResponse{ID: uint(c.ID), Firstname: c.Firstname, Lastname: c.Lastname}
	}
	return out
}

func actorOf(ctx context.Context) (domain.UserID, error) {
	id, ok := api.UserID(ctx)
	if !ok {
		return 0, apperrors.ErrUnauthorized("Authentication required")
	}
	return domain.UserID(id), nil
}

// toAppError words a use-case error the way the Gin handlers did. The IDs, when not 0, are
// reported in the details of a not-found error.
func toAppError(err error, relationshipID, contactID, relatedID uint) error {
	id := func(n uint) string { return strconv.FormatUint(uint64(n), 10) }
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.ErrNotFound("Relationship").WithDetails("id", id(relationshipID))
	case errors.Is(err, domain.ErrContactNotFound):
		return apperrors.ErrNotFound("Contact").WithDetails("id", id(contactID))
	case errors.Is(err, domain.ErrRelatedNotFound):
		return apperrors.ErrNotFound("Related contact").WithDetails("id", id(relatedID))
	}
	var f *app.Failure
	if errors.As(err, &f) {
		msg := map[app.Stage]string{
			app.StageRetrieve:        "Failed to retrieve relationship",
			app.StageContact:         "Failed to retrieve contact",
			app.StageRelated:         "Failed to retrieve related contact",
			app.StageList:            "Failed to retrieve relationships",
			app.StageIncoming:        "Failed to retrieve incoming relationships",
			app.StageCreate:          "Failed to save relationship",
			app.StageUpdate:          "Failed to update relationship",
			app.StageDelete:          "Failed to delete relationship",
			app.StageGraphContacts:   "Failed to fetch contacts",
			app.StageGraphLinks:      "Failed to fetch relationships",
			app.StageGraphActivities: "Failed to fetch activities",
		}[f.Stage]
		if msg != "" {
			return apperrors.ErrDatabase(msg).WithError(f.Err)
		}
	}
	return apperrors.ErrInternal("Unexpected error").WithError(err)
}
