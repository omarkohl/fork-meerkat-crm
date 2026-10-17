// Package httpadapter exposes the reminders use cases as huma operations. It maps HTTP to the
// use cases and their results and errors back to the responses the API always sent
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
	"meerkat/reminders/internal/app"
	"meerkat/reminders/internal/domain"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
)

const recurrences = "once,weekly,monthly,quarterly,six-months,yearly"

// Register adds the reminder operations to a, on group (the authenticated group).
func Register(a huma.API, group gin.IRoutes, uc *app.UseCases) {
	h := &handlers{uc: uc}
	tags := []string{"reminders"}
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "list-reminders", Method: http.MethodGet, Path: "/reminders",
		Summary: "All reminders, due first", Tags: tags,
	}, h.list)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "upcoming-reminders", Method: http.MethodGet, Path: "/reminders/upcoming",
		Summary: "Reminders due within a week, or the next five when fewer are due", Tags: tags,
	}, h.upcoming)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "list-contact-reminders", Method: http.MethodGet, Path: "/contacts/{id}/reminders",
		Summary: "The reminders about a contact", Tags: tags,
	}, h.ofContact)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "create-reminder", Method: http.MethodPost, Path: "/contacts/{id}/reminders",
		Summary: "Add a reminder about a contact", Tags: tags,
	}, h.create)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "get-reminder", Method: http.MethodGet, Path: "/reminders/{id}",
		Summary: "Get a reminder", Tags: tags,
	}, h.get)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "update-reminder", Method: http.MethodPut, Path: "/reminders/{id}",
		Summary: "Replace the fields of a reminder", Tags: tags,
	}, h.update)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "complete-reminder", Method: http.MethodPost, Path: "/reminders/{id}/complete",
		Summary: "Complete a reminder, or skip it with skip=true; a recurring one is scheduled again", Tags: tags,
	}, h.complete)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "delete-reminder", Method: http.MethodDelete, Path: "/reminders/{id}",
		Summary: "Delete a reminder", Tags: tags,
	}, h.delete)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "list-reminder-completions", Method: http.MethodGet, Path: "/contacts/{id}/reminder-completions",
		Summary: "The completed reminders of a contact, newest first", Tags: tags,
	}, h.completions)
	api.RegisterOn(a, group, huma.Operation{
		OperationID: "delete-reminder-completion", Method: http.MethodDelete, Path: "/reminder-completions/{id}",
		Summary: "Delete a completion from the timeline", Tags: tags,
	}, h.deleteCompletion)
}

type handlers struct{ uc *app.UseCases }

type ReminderBody struct {
	Message               string    `json:"message" validate:"required,min=1,max=500"`
	ByMail                *bool     `json:"by_mail,omitempty" doc:"Send it by e-mail. Leaving it out keeps the current value."`
	RemindAt              time.Time `json:"remind_at" validate:"required" doc:"Only the day counts; the time of day is dropped."`
	Recurrence            string    `json:"recurrence" validate:"required,oneof=once weekly monthly quarterly six-months yearly" enum:"once,weekly,monthly,quarterly,six-months,yearly"`
	ReoccurFromCompletion *bool     `json:"reoccur_from_completion,omitempty" doc:"Count the next occurrence from completion. Leaving it out keeps the current value."`
	ContactID             *uint     `json:"contact_id" validate:"required" doc:"Required; when the reminder is added under /contacts/{id}/reminders the contact of the URL wins."`
}

type ReminderResponse struct {
	ID                    uint       `json:"ID"`
	CreatedAt             time.Time  `json:"CreatedAt"`
	UpdatedAt             time.Time  `json:"UpdatedAt"`
	DeletedAt             *time.Time `json:"DeletedAt" nullable:"true"`
	Message               string     `json:"message"`
	ByMail                *bool      `json:"by_mail" nullable:"true"`
	RemindAt              time.Time  `json:"remind_at"`
	Recurrence            string     `json:"recurrence" enum:"once,weekly,monthly,quarterly,six-months,yearly"`
	ReoccurFromCompletion *bool      `json:"reoccur_from_completion" nullable:"true"`
	Completed             bool       `json:"completed"`
	EmailSent             bool       `json:"email_sent"`
	LastSent              *time.Time `json:"last_sent" nullable:"true"`
	ContactID             *uint      `json:"contact_id" nullable:"true"`
}

type CompletionResponse struct {
	ID          uint       `json:"ID"`
	CreatedAt   time.Time  `json:"CreatedAt"`
	UpdatedAt   time.Time  `json:"UpdatedAt"`
	DeletedAt   *time.Time `json:"DeletedAt" nullable:"true"`
	ReminderID  *uint      `json:"reminder_id,omitempty"`
	ContactID   uint       `json:"contact_id"`
	Message     string     `json:"message"`
	CompletedAt time.Time  `json:"completed_at"`
}

type idPath struct {
	ID uint `path:"id" minimum:"1" doc:"The ID in the path"`
}

type CreateInput struct {
	ID   uint `path:"id" minimum:"1" doc:"The contact's ID"`
	Body ReminderBody
}

func (i *CreateInput) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	return api.ValidateBody(prefix, &i.Body)
}

type UpdateInput struct {
	ID   uint `path:"id" minimum:"1" doc:"The reminder's ID"`
	Body ReminderBody
}

func (i *UpdateInput) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	return api.ValidateBody(prefix, &i.Body)
}

type CompleteInput struct {
	ID   uint   `path:"id" minimum:"1" doc:"The reminder's ID"`
	Skip string `query:"skip" doc:"true: reschedule or delete without a timeline entry"`
}

type ReminderOutput struct{ Body ReminderResponse }

type ReminderListOutput struct {
	Body struct {
		Reminders []ReminderResponse `json:"reminders"`
	}
}

type ReminderSavedOutput struct {
	Body struct {
		Message  string           `json:"message"`
		Reminder ReminderResponse `json:"reminder"`
	}
}

type ReminderCompletedOutput struct {
	Body struct {
		Message  string            `json:"message"`
		Reminder *ReminderResponse `json:"reminder,omitempty" doc:"Absent when a once reminder was finished and deleted"`
	}
}

type ReminderMessageOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

type CompletionListOutput struct {
	Body struct {
		Completions []CompletionResponse `json:"completions"`
	}
}

func (h *handlers) list(ctx context.Context, _ *struct{}) (*ReminderListOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	list, err := h.uc.AllReminders(ctx, actor)
	if err != nil {
		return nil, toAppError(err, 0, 0, 0)
	}
	return listOutput(list), nil
}

func (h *handlers) upcoming(ctx context.Context, _ *struct{}) (*ReminderListOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	list, err := h.uc.UpcomingReminders(ctx, actor, api.Now(ctx))
	if err != nil {
		return nil, toAppError(err, 0, 0, 0)
	}
	return listOutput(list), nil
}

func (h *handlers) ofContact(ctx context.Context, in *idPath) (*ReminderListOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	list, err := h.uc.RemindersOfContact(ctx, actor, domain.ContactID(in.ID))
	if err != nil {
		return nil, toAppError(err, 0, in.ID, 0)
	}
	return listOutput(list), nil
}

func (h *handlers) create(ctx context.Context, in *CreateInput) (*ReminderSavedOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.uc.CreateReminder(ctx, actor, domain.ContactID(in.ID), in.Body.draft())
	if err != nil {
		return nil, toAppError(err, 0, in.ID, 0)
	}
	return saved("Reminder created successfully", r), nil
}

func (h *handlers) get(ctx context.Context, in *idPath) (*ReminderOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.uc.GetReminder(ctx, actor, domain.ReminderID(in.ID))
	if err != nil {
		return nil, toAppError(err, in.ID, 0, 0)
	}
	return &ReminderOutput{Body: newReminderResponse(r)}, nil
}

func (h *handlers) update(ctx context.Context, in *UpdateInput) (*ReminderSavedOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.uc.UpdateReminder(ctx, actor, domain.ReminderID(in.ID), in.Body.draft())
	if err != nil {
		return nil, toAppError(err, in.ID, 0, 0)
	}
	return saved("Reminder updated successfully", r), nil
}

func (h *handlers) complete(ctx context.Context, in *CompleteInput) (*ReminderCompletedOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	skip := in.Skip == "true"
	done, err := h.uc.CompleteReminder(ctx, actor, domain.ReminderID(in.ID), skip, api.Now(ctx))
	if err != nil {
		return nil, toAppError(err, in.ID, 0, 0)
	}
	action := "completed"
	if skip {
		action = "skipped"
	}
	log := api.Logger(ctx)
	if done.RecordErr != nil {
		log.Error().Err(done.RecordErr).Msg("Failed to create reminder completion record")
	}
	out := &ReminderCompletedOutput{}
	if done.Finished {
		log.Info().Uint("reminder_id", in.ID).Str("action", action).Msg("Deleted 'once' reminder")
		out.Body.Message = "Reminder " + action + " and deleted"
		return out, nil
	}
	if !done.Reminder.Completed {
		log.Info().Time("next_remind_at", done.Reminder.RemindAt).Uint("reminder_id", uint(done.Reminder.ID)).
			Str("action", action).Msg("Reminder processed, next occurrence scheduled")
	}
	out.Body.Message = "Reminder " + action + " successfully"
	r := newReminderResponse(done.Reminder)
	out.Body.Reminder = &r
	return out, nil
}

func (h *handlers) delete(ctx context.Context, in *idPath) (*ReminderMessageOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.uc.DeleteReminder(ctx, actor, domain.ReminderID(in.ID)); err != nil {
		return nil, toAppError(err, in.ID, 0, 0)
	}
	out := &ReminderMessageOutput{}
	out.Body.Message = "Reminder deleted"
	return out, nil
}

func (h *handlers) completions(ctx context.Context, in *idPath) (*CompletionListOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	list, err := h.uc.CompletionsOfContact(ctx, actor, domain.ContactID(in.ID))
	if err != nil {
		return nil, toAppError(err, 0, in.ID, 0)
	}
	out := &CompletionListOutput{}
	out.Body.Completions = make([]CompletionResponse, len(list))
	for i, c := range list {
		out.Body.Completions[i] = newCompletionResponse(c)
	}
	return out, nil
}

func (h *handlers) deleteCompletion(ctx context.Context, in *idPath) (*ReminderMessageOutput, error) {
	actor, err := actorOf(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.uc.DeleteCompletion(ctx, actor, domain.CompletionID(in.ID)); err != nil {
		return nil, toAppError(err, 0, 0, in.ID)
	}
	out := &ReminderMessageOutput{}
	out.Body.Message = "Reminder completion deleted"
	return out, nil
}

func (b ReminderBody) draft() domain.Reminder {
	r := domain.Reminder{
		Message: b.Message, ByMail: b.ByMail, RemindAt: b.RemindAt, Recurrence: b.Recurrence,
		ReoccurFromCompletion: b.ReoccurFromCompletion,
	}
	if b.ContactID != nil {
		id := domain.ContactID(*b.ContactID)
		r.ContactID = &id
	}
	return r
}

func listOutput(list []domain.Reminder) *ReminderListOutput {
	out := &ReminderListOutput{}
	out.Body.Reminders = make([]ReminderResponse, len(list))
	for i, r := range list {
		out.Body.Reminders[i] = newReminderResponse(r)
	}
	return out
}

func saved(message string, r domain.Reminder) *ReminderSavedOutput {
	out := &ReminderSavedOutput{}
	out.Body.Message = message
	out.Body.Reminder = newReminderResponse(r)
	return out
}

func newReminderResponse(r domain.Reminder) ReminderResponse {
	out := ReminderResponse{
		ID: uint(r.ID), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, DeletedAt: r.DeletedAt,
		Message: r.Message, ByMail: r.ByMail, RemindAt: r.RemindAt, Recurrence: r.Recurrence,
		ReoccurFromCompletion: r.ReoccurFromCompletion, Completed: r.Completed, EmailSent: r.EmailSent, LastSent: r.LastSent,
	}
	if r.ContactID != nil {
		id := uint(*r.ContactID)
		out.ContactID = &id
	}
	return out
}

func newCompletionResponse(c domain.Completion) CompletionResponse {
	out := CompletionResponse{
		ID: uint(c.ID), CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, DeletedAt: c.DeletedAt,
		ContactID: uint(c.ContactID), Message: c.Message, CompletedAt: c.CompletedAt,
	}
	if c.ReminderID != nil {
		id := uint(*c.ReminderID)
		out.ReminderID = &id
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
func toAppError(err error, reminderID, contactID, completionID uint) error {
	id := func(n uint) string { return strconv.FormatUint(uint64(n), 10) }
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.ErrNotFound("Reminder").WithDetails("id", id(reminderID))
	case errors.Is(err, domain.ErrContactNotFound):
		e := apperrors.ErrNotFound("Contact")
		if contactID != 0 {
			e = e.WithDetails("id", id(contactID))
		}
		return e
	case errors.Is(err, domain.ErrCompletionNotFound):
		return apperrors.ErrNotFound("Reminder completion").WithDetails("id", id(completionID))
	}
	var f *app.Failure
	if errors.As(err, &f) {
		msg := map[app.Stage]string{
			app.StageRetrieve:         "Failed to retrieve reminder",
			app.StageContact:          "Failed to retrieve contact",
			app.StageList:             "Failed to retrieve reminders",
			app.StageCreate:           "Failed to save reminder",
			app.StageUpdate:           "Failed to update reminder",
			app.StageFinish:           "Failed to delete 'once' reminder",
			app.StageDelete:           "Failed to delete reminder",
			app.StageCompletions:      "Failed to retrieve reminder completions",
			app.StageGetCompletion:    "Failed to retrieve reminder completion",
			app.StageDeleteCompletion: "Failed to delete reminder completion",
		}[f.Stage]
		if msg != "" {
			return apperrors.ErrDatabase(msg).WithError(f.Err)
		}
	}
	return apperrors.ErrInternal("Unexpected error").WithError(err)
}
