package httpadapter

import (
	"time"

	"meerkat/contacts/internal/domain"
)

// ContactValue is a typed email address, phone number, website or messaging handle.
type ContactValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// ContactAddress is a structured postal address.
type ContactAddress struct {
	Type    string `json:"type"`
	Street  string `json:"street"`
	City    string `json:"city"`
	Region  string `json:"region"`
	Postal  string `json:"postal"`
	Country string `json:"country"`
}

// ContactResponse is a contact as the API returns it. The JSON names (ID, CreatedAt, ... next
// to snake_case ones) are those of the GORM model it replaces, because clients depend on them.
type ContactResponse struct {
	ID        uint       `json:"ID"`
	CreatedAt time.Time  `json:"CreatedAt"`
	UpdatedAt time.Time  `json:"UpdatedAt"`
	DeletedAt *time.Time `json:"DeletedAt" nullable:"true"`

	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Nickname  string `json:"nickname"`
	Gender    string `json:"gender"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Birthday  string `json:"birthday"`
	Photo     string `json:"photo"`

	// Relationships is null unless the endpoint loads them.
	Relationships []RelationshipResponse `json:"relationships" nullable:"true"`

	Address            string   `json:"address"`
	HowWeMet           string   `json:"how_we_met"`
	FoodPreference     string   `json:"food_preference"`
	WorkInformation    string   `json:"work_information"`
	ContactInformation string   `json:"contact_information"`
	Circles            []string `json:"circles" nullable:"true"`

	Emails    []ContactValue   `json:"emails" nullable:"true"`
	Phones    []ContactValue   `json:"phones" nullable:"true"`
	Addresses []ContactAddress `json:"addresses" nullable:"true"`
	URLs      []ContactValue   `json:"urls" nullable:"true"`
	IMPPs     []ContactValue   `json:"impps" nullable:"true"`

	Prefix     string `json:"prefix"`
	MiddleName string `json:"middle_name"`
	Suffix     string `json:"suffix"`

	Organization string `json:"organization"`
	Department   string `json:"department"`
	JobTitle     string `json:"job_title"`
	Role         string `json:"role"`

	Anniversary string `json:"anniversary"`
	Note        string `json:"note"`

	CustomFields map[string]string `json:"custom_fields" nullable:"true"`

	Archived bool `json:"archived"`
}

// RelationshipResponse is a relationship between contacts, as embedded in a contact when
// the endpoint loads them.
type RelationshipResponse struct {
	ID               uint       `json:"ID"`
	CreatedAt        time.Time  `json:"CreatedAt"`
	UpdatedAt        time.Time  `json:"UpdatedAt"`
	DeletedAt        *time.Time `json:"DeletedAt" nullable:"true"`
	ContactID        uint       `json:"contact_id"`
	RelatedContactID *uint      `json:"related_contact_id" nullable:"true"`
	Name             string     `json:"name"`
	Type             string     `json:"type"`
	Birthday         string     `json:"birthday"`
	Gender           string     `json:"gender"`
}

func contactResponse(c domain.Contact) ContactResponse {
	r := ContactResponse{
		ID: uint(c.ID), CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, DeletedAt: c.DeletedAt,
		Firstname: c.Firstname, Lastname: c.Lastname, Nickname: c.Nickname, Gender: c.Gender,
		Email: c.Email, Phone: c.Phone, Birthday: c.Birthday, Photo: c.Photo,
		Address: c.Address, HowWeMet: c.HowWeMet, FoodPreference: c.FoodPreference,
		WorkInformation: c.WorkInformation, ContactInformation: c.ContactInformation,
		Circles: c.Circles,
		Prefix:  c.Prefix, MiddleName: c.MiddleName, Suffix: c.Suffix,
		Organization: c.Organization, Department: c.Department, JobTitle: c.JobTitle, Role: c.Role,
		Anniversary: c.Anniversary, Note: c.Note,
		CustomFields: c.CustomFields, Archived: c.Archived,
	}
	r.Emails = values(c.Emails)
	r.Phones = values(c.Phones)
	r.URLs = values(c.URLs)
	r.IMPPs = values(c.IMPPs)
	for _, a := range c.Addresses {
		r.Addresses = append(r.Addresses, ContactAddress{Type: a.Type, Street: a.Street, City: a.City, Region: a.Region, Postal: a.Postal, Country: a.Country})
	}
	return r
}

func values(in []domain.Value) []ContactValue {
	var out []ContactValue
	for _, v := range in {
		out = append(out, ContactValue{Type: v.Type, Value: v.Value})
	}
	return out
}
