package main

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Sensitivity string

const (
	Public   Sensitivity = "public"
	Personal Sensitivity = "personal"
	Secret   Sensitivity = "secret"
)

type Hints struct {
	Locale   string
	Country  string
	Currency string
	Zone     string
}

type Candidate struct {
	Value      any     `json:"value"`
	Confidence float64 `json:"confidence"`
}

type Spec struct {
	Types map[string]*Type `json:"types"`
	stamp string
}

type Type struct {
	Name        string            `json:"-"`
	Builtin     bool              `json:"-"`
	Edited      bool              `json:"-"`
	Description string            `json:"description,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Extends     string            `json:"extends,omitempty"`
	Category    string            `json:"category,omitempty"`
	Sensitivity Sensitivity       `json:"sensitivity,omitempty"`
	Schema      map[string]any    `json:"schema,omitempty"`
	Normalize   []string          `json:"normalize,omitempty"`
	Parse       string            `json:"parse,omitempty"`
	Check       string            `json:"check,omitempty"`
	Format      string            `json:"format,omitempty"`
	Fields      map[string]*Field `json:"fields,omitempty"`
	Examples    []any             `json:"examples,omitempty"`
}

type Field struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	List        bool     `json:"list,omitempty"`
	Indexed     bool     `json:"indexed,omitempty"`
	To          string   `json:"to,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

type FieldError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type ValidationError struct {
	Type   string       `json:"type"`
	Errors []FieldError `json:"errors"`
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "invalid " + e.Type
	}
	return "invalid " + e.Type + ": " + e.Errors[0].Path + ": " + e.Errors[0].Message
}

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
)

type Item struct {
	Source     string
	ExternalID string
	Payload    json.RawMessage
	Space      uuid.UUID
}

type Entity struct {
	ID        uuid.UUID       `json:"id"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Proposed  bool            `json:"proposed"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Space     uuid.UUID       `json:"space"`
}

type Link struct {
	FromID   uuid.UUID `json:"from_id"`
	ToID     uuid.UUID `json:"to_id"`
	Kind     string    `json:"kind"`
	FromType string    `json:"from_type"`
}

type Filter struct {
	Types    []string
	Where    map[string]any
	Keys     map[string]string
	Space    uuid.UUID
	Proposed bool
	Limit    int
	After    uuid.UUID
}

type IntakeConfig struct {
	Endpoint string
	Model    string
	APIKey   string
	Samples  int
	Accept   float64
	Review   float64
	Hints    Hints
	Vision   string
	Whisper  string
}

type FieldResult struct {
	Path       string  `json:"path"`
	Value      any     `json:"value"`
	Confidence float64 `json:"confidence"`
	Route      string  `json:"route"`
}

type IntakeResult struct {
	Source     string        `json:"source,omitempty"`
	ExternalID string        `json:"external_id,omitempty"`
	Type       string        `json:"type"`
	EntityID   uuid.UUID     `json:"entity_id,omitzero"`
	Proposed   bool          `json:"proposed,omitempty"`
	Confidence float64       `json:"confidence"`
	Fields     []FieldResult `json:"fields"`
}
