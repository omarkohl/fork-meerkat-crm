package api

import (
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// relaxRequestBody makes the schema of the operation's request body behave like the old
// ShouldBindJSON plus validate tags did:
//
//   - unknown JSON properties are ignored, not rejected (huma rejects them by default);
//   - a property is required when its `validate` tag says "required" (or it has the huma
//     tag required:"true"), not merely because its JSON tag lacks omitempty. The
//     validate tags are checked by ValidateBody, so the rule exists once and a missing
//     field is reported once.
func relaxRequestBody(a huma.API, op huma.Operation) {
	oa := a.OpenAPI()
	reg := oa.Components.Schemas
	item := oa.Paths[op.Path]
	if item == nil {
		return
	}
	var registered *huma.Operation
	switch op.Method {
	case "POST":
		registered = item.Post
	case "PUT":
		registered = item.Put
	case "PATCH":
		registered = item.Patch
	case "DELETE":
		registered = item.Delete
	}
	if registered == nil || registered.RequestBody == nil {
		return
	}
	for _, media := range registered.RequestBody.Content {
		if media.Schema != nil {
			relax(reg, media.Schema, nil, map[*huma.Schema]bool{})
		}
	}
}

func relax(reg huma.Registry, s *huma.Schema, t reflect.Type, seen map[*huma.Schema]bool) {
	if s == nil {
		return
	}
	if s.Ref != "" {
		t = reg.TypeFromRef(s.Ref)
		s = reg.SchemaFromRef(s.Ref)
		if s == nil {
			return
		}
	}
	if seen[s] {
		return
	}
	seen[s] = true

	for t != nil && (t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array) {
		t = t.Elem()
	}
	if s.Items != nil {
		relax(reg, s.Items, t, seen)
	}
	if s.Type != huma.TypeObject {
		return
	}
	s.AdditionalProperties = nil
	if t == nil || t.Kind() != reflect.Struct {
		return
	}
	var required []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if !f.IsExported() || name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		if hasToken(f.Tag.Get("validate"), "required") || f.Tag.Get("required") == "true" {
			required = append(required, name)
		}
		if prop := s.Properties[name]; prop != nil {
			relax(reg, prop, f.Type, seen)
		}
	}
	s.Required = required
	s.PrecomputeMessages() // huma caches the required set and the messages
}

func hasToken(tag, token string) bool {
	for _, part := range strings.Split(tag, ",") {
		if part == token {
			return true
		}
	}
	return false
}
