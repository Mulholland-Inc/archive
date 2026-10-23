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
	Version int              `json:"version"`
	Types   map[string]*Type `json:"types"`
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

type Op struct {
	Op    string `json:"op"`
	Type  string `json:"type"`
	Field string `json:"field,omitempty"`
	To    string `json:"to,omitempty"`
	Def   any    `json:"def,omitempty"`
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

type Raw struct {
	ID         uuid.UUID       `json:"id"`
	Source     string          `json:"source"`
	ExternalID string          `json:"external_id"`
	Hash       string          `json:"hash"`
	FetchedAt  time.Time       `json:"fetched_at"`
	Payload    json.RawMessage `json:"payload"`
	Space      uuid.UUID       `json:"space"`
}

type Record struct {
	ID          uuid.UUID       `json:"id"`
	Type        string          `json:"type"`
	SpecVersion int             `json:"spec_version"`
	Data        json.RawMessage `json:"data"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Space       uuid.UUID       `json:"space"`
}

type Provenance struct {
	ID         uuid.UUID   `json:"id"`
	RecordID   uuid.UUID   `json:"record_id"`
	FieldPath  string      `json:"field_path"`
	RawIDs     []uuid.UUID `json:"raw_ids"`
	Method     string      `json:"method"`
	Confidence float64     `json:"confidence"`
	Original   string      `json:"original,omitempty"`
	Space      uuid.UUID   `json:"space"`
	FileID     *uuid.UUID  `json:"file_id,omitempty"`
	Page       int         `json:"page,omitempty"`
}

type Link struct {
	ID       uuid.UUID `json:"id"`
	FromID   uuid.UUID `json:"from_id"`
	ToID     uuid.UUID `json:"to_id"`
	Kind     string    `json:"kind"`
	Space    uuid.UUID `json:"space"`
	FromType string    `json:"from_type"`
}

type Change struct {
	ID         uuid.UUID `json:"id"`
	ProposedBy string    `json:"proposed_by"`
	Status     string    `json:"status"`
	Ops        []Op      `json:"ops"`
	FromVer    int       `json:"from_version"`
	ToVer      int       `json:"to_version"`
	CreatedAt  time.Time `json:"created_at"`
}

type Filter struct {
	Types []string
	Where map[string]any
	Keys  map[string]string
	Space uuid.UUID
	Limit int
	After uuid.UUID
}

type IntakeConfig struct {
	Endpoint string
	Model    string
	APIKey   string
	Samples  int
	Accept   float64
	Review   float64
	Labels   string
	Hints    Hints
	Vision   string
	Whisper  string
	Embed    string
	EmbedDim int
}

type FieldResult struct {
	Path       string  `json:"path"`
	Value      any     `json:"value"`
	Original   string  `json:"original,omitempty"`
	RawScore   float64 `json:"raw_score"`
	Confidence float64 `json:"confidence"`
	Route      string  `json:"route"`
}

type IntakeResult struct {
	RawID    uuid.UUID     `json:"raw_id"`
	Type     string        `json:"type"`
	RecordID uuid.UUID     `json:"record_id,omitempty"`
	Fields   []FieldResult `json:"fields"`
}
