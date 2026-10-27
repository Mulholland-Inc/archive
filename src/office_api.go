package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/google/uuid"
)

type OfficeModel struct {
	Type       string            `json:"type"`
	Paragraphs []OfficeParagraph `json:"paragraphs,omitempty"`
	Tables     []OfficeTable     `json:"tables,omitempty"`
	Sheets     []OfficeSheet     `json:"sheets,omitempty"`
	Slides     []OfficeSlide     `json:"slides,omitempty"`
}
type OfficeParagraph struct {
	ID    string   `json:"id,omitempty"`
	Style string   `json:"style,omitempty"`
	List  string   `json:"list,omitempty"`
	In    string   `json:"in,omitempty"`
	Text  string   `json:"text"`
	Runs  []string `json:"runs,omitempty"`
}
type OfficeTable struct {
	ID   string     `json:"id"`
	Rows [][]string `json:"rows"`
}
type OfficeSheet struct {
	Name      string         `json:"name"`
	Dimension string         `json:"dimension,omitempty"`
	Merged    []string       `json:"merged,omitempty"`
	Tables    []OfficeXTable `json:"tables,omitempty"`
	Cells     []OfficeCell   `json:"cells"`
}
type OfficeXTable struct {
	Name    string   `json:"name"`
	Ref     string   `json:"ref"`
	Columns []string `json:"columns"`
}
type OfficeCell struct {
	Ref     string `json:"ref"`
	Value   string `json:"value,omitempty"`
	Formula string `json:"formula,omitempty"`
}
type OfficeSlide struct {
	Number int           `json:"slide"`
	Hidden bool          `json:"hidden,omitempty"`
	Shapes []OfficeShape `json:"shapes"`
}
type OfficeShape struct {
	Index       int               `json:"shape"`
	Name        string            `json:"name,omitempty"`
	Placeholder string            `json:"placeholder,omitempty"`
	Paragraphs  []OfficeParagraph `json:"paragraphs"`
}
type OfficeOp struct {
	Op        string `json:"op"`
	Para      string `json:"para,omitempty"`
	Find      string `json:"find,omitempty"`
	Replace   string `json:"replace,omitempty"`
	All       bool   `json:"all,omitempty"`
	Text      string `json:"text,omitempty"`
	Style     string `json:"style,omitempty"`
	Table     *int   `json:"table,omitempty"`
	Row       *int   `json:"row,omitempty"`
	Col       *int   `json:"col,omitempty"`
	Sheet     string `json:"sheet,omitempty"`
	Cell      string `json:"cell,omitempty"`
	Value     any    `json:"value,omitempty"`
	TableName string `json:"table_name,omitempty"`
	Values    []any  `json:"values,omitempty"`
	Formula   string `json:"formula,omitempty"`
	Slide     int    `json:"slide,omitempty"`
	Shape     *int   `json:"shape,omitempty"`
}

var officeOpTypes = map[string]string{"replace_text": "docx pptx", "insert_paragraph_after": "docx", "delete_paragraph": "docx", "set_cell_text": "docx", "set_cell": "xlsx", "set_formula": "xlsx", "clear_cell": "xlsx", "append_table_row": "xlsx", "replace_shape_text": "pptx"}

func OfficeKind(name string) (string, error) {
	switch k := strings.ToLower(strings.TrimPrefix(path.Ext(name), ".")); k {
	case "docx", "docm", "dotx":
		return "docx", nil
	case "xlsx", "xlsm", "xltx":
		return "xlsx", nil
	case "pptx", "pptm", "potx":
		return "pptx", nil
	default:
		return "", fmt.Errorf("unsupported file type %q; expected docx, xlsx or pptx", k)
	}
}
func OfficeParseOps(b []byte) ([]OfficeOp, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var ops []OfficeOp
	if err := d.Decode(&ops); err != nil {
		return nil, fmt.Errorf("edits must be a JSON array of objects with an \"op\" field: %v", err)
	}
	return ops, nil
}
func OfficeRead(b []byte, name string) (*OfficeModel, error) {
	kind, err := OfficeKind(name)
	if err != nil {
		return nil, err
	}
	p, err := OfficeOpen(b)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "docx":
		w, err := officeOpenWord(p)
		if err != nil {
			return nil, err
		}
		return w.model(), nil
	case "xlsx":
		bk, err := officeOpenBook(p)
		if err != nil {
			return nil, err
		}
		m := &OfficeModel{Type: "xlsx"}
		for _, n := range bk.names {
			s, err := bk.sheet(n)
			if err != nil {
				continue
			}
			m.Sheets = append(m.Sheets, s.model())
		}
		return m, nil
	default:
		d, err := officeOpenDeck(p)
		if err != nil {
			return nil, err
		}
		m := &OfficeModel{Type: "pptx"}
		for i := range d.slides {
			s, err := d.slide(i + 1)
			if err != nil {
				return nil, err
			}
			m.Slides = append(m.Slides, s.model(i+1))
		}
		return m, nil
	}
}
func OfficeEdit(b []byte, name string, ops []OfficeOp) ([]byte, error) {
	p, err := OfficeEditPackage(b, name, ops)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := p.Save(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func OfficeEditPackage(b []byte, name string, ops []OfficeOp) (*OfficePackage, error) {
	kind, err := OfficeKind(name)
	if err != nil {
		return nil, err
	}
	p, err := OfficeOpen(b)
	if err != nil {
		return nil, err
	}
	var st *officeIds
	if kind == "docx" {
		w, err := officeOpenWord(p)
		if err != nil {
			return nil, err
		}
		st = officeNewIDs(w.ps)
	}
	for i, op := range ops {
		n := len(p.Log[officeDocPart])
		if err := officeApply(p, kind, op, st); err != nil {
			return nil, fmt.Errorf("edit %d (%s): %v", i, op.Op, err)
		}
		if st != nil && !st.updated {
			for _, ss := range p.Log[officeDocPart][n:] {
				st.update(ss)
			}
		}
		if st != nil {
			st.updated = false
		}
	}
	return p, nil
}
func officeApply(p *OfficePackage, kind string, op OfficeOp, st *officeIds) error {
	types, ok := officeOpTypes[op.Op]
	if !ok {
		return fmt.Errorf("unknown op %q; ops are replace_text, insert_paragraph_after, delete_paragraph, set_cell_text (docx), set_cell, set_formula, clear_cell, append_table_row (xlsx), replace_text, replace_shape_text (pptx)", op.Op)
	}
	if !strings.Contains(types, kind) {
		return fmt.Errorf("op %s applies to %s files, not %s", op.Op, types, kind)
	}
	switch kind {
	case "docx":
		w, err := officeOpenWord(p)
		if err != nil {
			return err
		}
		w.ids = st
		switch op.Op {
		case "replace_text":
			if op.Find == "" {
				return fmt.Errorf("replace_text needs a non-empty \"find\"")
			}
			return w.replaceText(op)
		case "insert_paragraph_after":
			return w.insertParagraphAfter(op)
		case "delete_paragraph":
			return w.deleteParagraph(op)
		case "set_cell_text":
			return w.setCellText(op)
		}
	case "xlsx":
		bk, err := officeOpenBook(p)
		if err != nil {
			return err
		}
		if op.Op == "append_table_row" {
			return bk.appendTableRow(op)
		}
		if op.Cell == "" {
			return fmt.Errorf("%s needs \"cell\", for example \"B12\"", op.Op)
		}
		return bk.setCell(op)
	case "pptx":
		d, err := officeOpenDeck(p)
		if err != nil {
			return err
		}
		if op.Op == "replace_shape_text" {
			return d.replaceShapeText(op)
		}
		if op.Find == "" {
			return fmt.Errorf("replace_text needs a non-empty \"find\"")
		}
		return d.replaceText(op)
	}
	return nil
}

func (a *Archive) documentBytes(ctx context.Context, id uuid.UUID) (*File, []byte, error) {
	f, rc, err := a.OpenFile(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	return f, b, err
}

func (a *Archive) ReadDocument(ctx context.Context, id uuid.UUID) (*OfficeModel, error) {
	f, b, err := a.documentBytes(ctx, id)
	if err != nil {
		return nil, err
	}
	m, err := OfficeRead(b, f.Name)
	if err != nil {
		return nil, fmt.Errorf("archive.read: %v: %w", err, httpErrBadRequest)
	}
	return m, nil
}

func (a *Archive) EditDocument(ctx context.Context, id uuid.UUID, ops []OfficeOp) (*File, error) {
	f, b, err := a.documentBytes(ctx, id)
	if err != nil {
		return nil, err
	}
	out, err := OfficeEdit(b, f.Name, ops)
	if err != nil {
		return nil, fmt.Errorf("archive.edit: %v: %w", err, httpErrBadRequest)
	}
	tmp, err := os.CreateTemp(a.uploads, "edit-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(out)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(out)
	hash := hex.EncodeToString(sum[:])
	if err := a.blobs.put(ctx, hash, tmp.Name(), int64(len(out))); err != nil {
		return nil, err
	}
	nf, err := coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		return storeOne[File](ctx, a.st, `UPDATE file SET hash = $2, size = $3, status = 'pending', text = '', error = '', metadata = '{}'
			WHERE id = $1 RETURNING `+fileCols, id, hash, int64(len(out)))
	})
	if err == nil {
		a.wakeFiles()
	}
	return nf, err
}
