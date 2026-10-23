package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpInstructions = `Archive holds the user's personal data as typed objects. The ontology (spec) defines the types, built-in (email, phone, money, date, ...) and user-defined alike. A scalar type has a JSON Schema for its canonical form; a composite type has fields whose types are other types or references, and records are objects of composite types. A type can extend another and inherit from it.

Start with describe to learn the types, their fields and a few sample records. Read with query and get; write with upsert and delete. Every write is normalized and validated against the ontology; errors list each bad field.

To change the ontology: propose a list of ops, preview the change to see which records it touches and which would fail validation, then apply it if the user agrees. Every applied change can be reverted. Propose only what the user asked for.

Sources are the user's connected accounts and folders (Gmail, Google Drive, IMAP mail, folders and external connectors). List them with sources and sync one with sync; synced items land in the raw layer and intake maps them into the source's type.

Files are documents, photos, audio and mail or Drive attachments. List them with files, read one with file and its extracted text with file_text, and map one into a type with intake (file).

To find something without knowing its type or id, like "the lease for the Utrecht flat", use search: it matches words in any language and, when semantic search is on, meaning.`

type mcpTypeInput struct {
	Type string `json:"type" jsonschema:"name of a type in the ontology"`
}

type mcpSampleInput struct {
	Type string `json:"type" jsonschema:"name of a type in the ontology"`
	N    int    `json:"n,omitempty" jsonschema:"how many records, 1 to 100; default 5"`
}

type mcpQueryInput struct {
	Type   string         `json:"type" jsonschema:"type to list; objects of its subtypes are included"`
	Where  map[string]any `json:"where,omitempty" jsonschema:"field equality filters on top-level fields; values are normalized like writes, so a messy phone or date still matches"`
	Limit  int            `json:"limit,omitempty" jsonschema:"page size, up to 1000; default 100"`
	After  string         `json:"after,omitempty" jsonschema:"id of the last object on the previous page"`
	Reveal bool           `json:"reveal,omitempty" jsonschema:"show values of secret fields the user is granted instead of masks; only when the user asks"`
}

type mcpIDInput struct {
	Type string `json:"type" jsonschema:"type of the object, or one of its parent types"`
	ID   string `json:"id" jsonschema:"object id (uuid)"`
}

type mcpRestrictInput struct {
	Type       string `json:"type" jsonschema:"type of the object, or one of its parent types"`
	ID         string `json:"id" jsonschema:"object id (uuid)"`
	Restricted bool   `json:"restricted" jsonschema:"true hides it, false opens it"`
}

type mcpGetInput struct {
	Type   string `json:"type" jsonschema:"type of the object, or one of its parent types"`
	ID     string `json:"id" jsonschema:"object id (uuid)"`
	Reveal bool   `json:"reveal,omitempty" jsonschema:"show values of secret fields the user is granted instead of masks; only when the user asks"`
}

type mcpUpsertInput struct {
	Space string         `json:"space,omitempty" jsonschema:"space to create the object in; default the user's own space"`
	Type  string         `json:"type" jsonschema:"type of the object"`
	ID    string         `json:"id,omitempty" jsonschema:"id of an existing object to update; omit to create a new one"`
	Data  map[string]any `json:"data" jsonschema:"field values; on update only the given fields change and null clears a field"`
}

type mcpProposeInput struct {
	Ops []Op `json:"ops" jsonschema:"ontology ops, applied in order"`
}

type mcpChangeInput struct {
	ChangeID string `json:"change_id" jsonschema:"id of the change returned by propose"`
}

type mcpIntakeInput struct {
	Space string `json:"space,omitempty" jsonschema:"space to put the objects in; default the user's own space"`
	Dir   string `json:"dir,omitempty" jsonschema:"folder on the Archive server to read files from"`
	File  string `json:"file,omitempty" jsonschema:"id of a stored file to read instead of a folder; the object lands in the file's space"`
	Type  string `json:"type" jsonschema:"type to map each file into"`
}

type mcpFilesInput struct {
	Limit int    `json:"limit,omitempty" jsonschema:"page size, up to 1000; default 100"`
	After string `json:"after,omitempty" jsonschema:"id of the last file on the previous page"`
}

type mcpFileInput struct {
	ID string `json:"id" jsonschema:"file id (uuid), from files"`
}

type mcpFileTextInput struct {
	ID     string `json:"id" jsonschema:"file id (uuid), from files"`
	Offset int    `json:"offset,omitempty" jsonschema:"character to start at; default 0"`
	Limit  int    `json:"limit,omitempty" jsonschema:"how many characters; default 20000"`
}

type mcpSearchInput struct {
	Query  string `json:"query" jsonschema:"what to look for, in the user's words, in any language"`
	Type   string `json:"type,omitempty" jsonschema:"only objects of this type and its subtypes; file for files"`
	Space  string `json:"space,omitempty" jsonschema:"only this space (name)"`
	Source string `json:"source,omitempty" jsonschema:"only items synced from this source (id or name, from sources)"`
	From   string `json:"from,omitempty" jsonschema:"only items added on or after this date (2026-10-23 or RFC 3339)"`
	To     string `json:"to,omitempty" jsonschema:"only items added up to this date (2026-10-23, inclusive) or before this RFC 3339 time"`
	Limit  int    `json:"limit,omitempty" jsonschema:"how many results, up to 100; default 20"`
}

type mcpSourceInput struct {
	Source string `json:"source" jsonschema:"id or name of a source, from sources"`
}

func mcpServer(a *Archive) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "archive", Title: "Archive", Version: cliVersion},
		&mcp.ServerOptions{Instructions: mcpInstructions})

	mcpTool(s, "describe", "Describe the ontology: every type, built-in or user-defined, with its schema or fields (type, required, list, reference target, enum), parent type and description; and up to 3 sample records per user type. Call this first.",
		func(ctx context.Context, _ struct{}) (any, error) { return a.Describe(ctx, 3) })
	mcpTool(s, "sample", "Return a few records of exactly one type, to see what real data looks like.",
		func(ctx context.Context, in mcpSampleInput) (any, error) { return a.Sample(ctx, in.Type, in.N) })
	mcpTool(s, "query", "List objects of a type and its subtypes, optionally filtered by field equality. Results are ordered by id (creation time); page with after.",
		func(ctx context.Context, in mcpQueryInput) (any, error) {
			var after uuid.UUID
			if in.After != "" {
				var err error
				if after, err = uuid.Parse(in.After); err != nil {
					return nil, err
				}
			}
			rs, err := a.List(ctx, in.Type, "", in.Where, in.Limit, after)
			if err == nil && in.Reveal {
				err = a.Reveal(ctx, rs...)
			}
			return rs, err
		})
	mcpTool(s, "get", "Read one object by id, with its provenance (where each field came from and how sure Archive is) and its links to other objects.",
		func(ctx context.Context, in mcpGetInput) (any, error) {
			id, err := uuid.Parse(in.ID)
			if err != nil {
				return nil, err
			}
			r, err := a.Get(ctx, in.Type, id)
			if err == nil && in.Reveal {
				err = a.Reveal(ctx, r)
			}
			if err != nil {
				return nil, err
			}
			p, err := a.Provenance(ctx, in.Type, id)
			if err != nil {
				return nil, err
			}
			l, err := a.Links(ctx, in.Type, id)
			return map[string]any{"record": r, "provenance": p, "links": l}, err
		})
	mcpTool(s, "upsert", "Create an object (no id) or update some fields of an existing one (with id). Values are normalized to their canonical form, references must point at existing objects of the right type, and every bad field is reported.",
		func(ctx context.Context, in mcpUpsertInput) (any, error) {
			if in.ID == "" {
				return a.Create(ctx, in.Space, in.Type, in.Data)
			}
			id, err := uuid.Parse(in.ID)
			if err != nil {
				return nil, err
			}
			return a.Patch(ctx, in.Type, id, in.Data)
		})
	mcpTool(s, "delete", "Delete an object with its provenance and links. This cannot be undone; confirm with the user first.",
		func(ctx context.Context, in mcpIDInput) (any, error) {
			id, err := uuid.Parse(in.ID)
			if err != nil {
				return nil, err
			}
			return map[string]string{"deleted": id.String()}, a.Delete(ctx, in.Type, id)
		})
	mcpTool(s, "restrict", "Hide an object from everyone in its space but the space's admins and owners and the person who made it (restricted true), or open it again (false). Only an admin of the space may; use it for what is private, such as people who may be living.",
		func(ctx context.Context, in mcpRestrictInput) (any, error) {
			id, err := uuid.Parse(in.ID)
			if err != nil {
				return nil, err
			}
			return a.Restrict(ctx, in.Type, id, in.Restricted)
		})
	mcpTool(s, "propose", `Propose an ontology change; nothing changes until apply. Each op has "op", "type" and, depending on op: add_type (def: {"description","icon","extends","fields":{name: field}}), change_type (def: the type without fields, replacing description, icon, extends, schema and the other type-level properties), remove_type (not for built-in types), add_field (field, def), remove_field (field), change_field (field, def), rename_field (field, to: new name). A field def is {"type": any type name or "reference", "to": target type for references, "required", "list", "indexed", "enum": [...] for enum, "description"}. Returns the change with its id.`,
		func(ctx context.Context, in mcpProposeInput) (any, error) {
			return a.Propose(ctx, in.Ops)
		})
	mcpTool(s, "preview", "Dry-run a proposed change: how many records it touches, which would fail validation and why, and before/after samples. Show this to the user before applying.",
		func(ctx context.Context, in mcpChangeInput) (any, error) {
			id, err := uuid.Parse(in.ChangeID)
			if err != nil {
				return nil, err
			}
			return a.Preview(ctx, id)
		})
	mcpTool(s, "apply", "Apply a proposed change: the ontology moves to the next version and renamed or removed fields are migrated in existing records. Records that no longer validate keep their old spec version.",
		func(ctx context.Context, in mcpChangeInput) (any, error) {
			id, err := uuid.Parse(in.ChangeID)
			if err != nil {
				return nil, err
			}
			return a.Apply(ctx, id)
		})
	mcpTool(s, "revert", "Roll back the most recently applied change by applying its inverse as a new version. Data dropped by a removed field or type is not restored.",
		func(ctx context.Context, in mcpChangeInput) (any, error) {
			id, err := uuid.Parse(in.ChangeID)
			if err != nil {
				return nil, err
			}
			return a.Revert(ctx, id)
		})
	mcpTool(s, "intake", "Read every new or changed file in a folder, or one stored file, and map each into an object of the given type with a local model. Returns per-field values with calibrated confidence and the route taken (accept, review, drop).",
		func(ctx context.Context, in mcpIntakeInput) (any, error) {
			if in.File != "" {
				id, err := uuid.Parse(in.File)
				if err != nil {
					return nil, err
				}
				return a.IntakeFile(ctx, id, in.Type)
			}
			return a.IntakeDir(ctx, in.Space, in.Dir, in.Type)
		})
	mcpTool(s, "files", "List the user's files (documents, photos, audio, attachments from mail and Drive): name, media type, size, processing status and metadata such as page count, dimensions, camera and date taken. Ordered by id (upload time); page with after.",
		func(ctx context.Context, in mcpFilesInput) (any, error) {
			var after uuid.UUID
			if in.After != "" {
				var err error
				if after, err = uuid.Parse(in.After); err != nil {
					return nil, err
				}
			}
			return a.Files(ctx, "", in.Limit, after)
		})
	mcpTool(s, "file", "Read one file's details by id: name, media type, size, status (pending, processing, done or error) and metadata.",
		func(ctx context.Context, in mcpFileInput) (any, error) {
			id, err := uuid.Parse(in.ID)
			if err != nil {
				return nil, err
			}
			return a.GetFile(ctx, id)
		})
	mcpTool(s, "file_text", "Read the text extracted from a file: the text layer of a PDF (pages separated by form feeds), plain text, HTML, email, OCR of a photo or a transcript of audio. Long text comes in pages of characters.",
		func(ctx context.Context, in mcpFileTextInput) (any, error) {
			id, err := uuid.Parse(in.ID)
			if err != nil {
				return nil, err
			}
			text, err := a.FileText(ctx, id)
			if err != nil {
				return nil, err
			}
			rs := []rune(text)
			start := min(max(in.Offset, 0), len(rs))
			end := min(start+cmp.Or(max(in.Limit, 0), 20000), len(rs))
			return map[string]any{"id": id, "text": string(rs[start:end]), "offset": start, "total": len(rs)}, nil
		})
	mcpTool(s, "search", "Search everything the user can see: objects of every type, files by name and extracted text, and synced items not yet mapped into a type. Full-text matching ignores case and accents and matches word prefixes; with semantic search on, results close in meaning rank high too, and both rankings are fused. Each hit has its kind (record or raw), type, data for records, a snippet for files and raw items, and its text and vector rank.",
		func(ctx context.Context, in mcpSearchInput) (any, error) {
			return a.Search(ctx, in.Query, SearchFilter{Type: in.Type, Space: in.Space, Source: in.Source, From: in.From, To: in.To, Limit: in.Limit})
		})
	mcpTool(s, "sources", "List the sources the user can see: kind, name, space, target type, schedule, sync cursor (state) and the status and error of the last run.",
		func(ctx context.Context, _ struct{}) (any, error) { return a.Sources(ctx) })
	mcpTool(s, "sync", "Sync one source now: fetch new items into the raw layer and map them into the source's type with intake. Returns how many items were fetched, new and mapped, and the error if the run failed.",
		func(ctx context.Context, in mcpSourceInput) (any, error) { return a.RunSource(ctx, in.Source) })
	return s
}

func (a *Archive) mcpVerify(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	ac, err := a.Authenticate(ctx, token)
	if errors.Is(err, ErrUnauthorized) {
		return nil, fmt.Errorf("%w: %v", auth.ErrInvalidToken, err)
	}
	if err != nil {
		return nil, err
	}
	return &auth.TokenInfo{UserID: ac.UserID.String(), Extra: map[string]any{"actor": ac}}, nil
}

func mcpTool[In any](s *mcp.Server, name, desc string, f func(context.Context, In) (any, error)) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
			if req.Extra != nil && req.Extra.TokenInfo != nil {
				ac, _ := req.Extra.TokenInfo.Extra["actor"].(*Actor)
				ctx = WithActor(ctx, ac)
			}
			out, err := f(ctx, in)
			var ve *ValidationError
			if errors.As(err, &ve) {
				b, _ := json.Marshal(ve)
				return nil, nil, errors.New(string(b))
			}
			return nil, out, err
		})
}
