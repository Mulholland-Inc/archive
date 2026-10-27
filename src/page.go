package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/go-crdt/crdt"
	"github.com/go-crdt/crdt/structured"
	"github.com/google/uuid"
)

const (
	pageExt                  = ".page"
	pageMIME                 = "application/vnd.archive.page"
	pageSite     crdt.SiteID = 1
	pageOpsNames             = "replace_text, insert_paragraph_after, delete_paragraph, set_style"
)

type pageDoc struct {
	mu   sync.Mutex
	hash string
	snap []byte
	doc  *structured.Blocks
}

func pageIs(name string) bool { return strings.EqualFold(path.Ext(name), pageExt) }

func pageLoad(b []byte) (*structured.Blocks, error) {
	if len(b) == 0 {
		return structured.NewBlocks(pageSite), nil
	}
	d, err := structured.LoadBlocks(pageSite, b)
	if err != nil {
		return nil, fmt.Errorf("not a readable page: %v", err)
	}
	return d, nil
}

func pageText(d *structured.Blocks) string {
	return strings.ReplaceAll(d.Plain("\n"), "\x00", "")
}

func pageModel(d *structured.Blocks) *OfficeModel {
	m := &OfficeModel{Type: "page", Paragraphs: []OfficeParagraph{}}
	for _, b := range d.List() {
		m.Paragraphs = append(m.Paragraphs, OfficeParagraph{ID: b.ID.String(), Style: b.Type, Text: b.Text})
	}
	return m
}

func (a *Archive) CreatePage(ctx context.Context, space, name string) (*File, error) {
	if strings.TrimSpace(strings.TrimSuffix(name, pageExt)) == "" {
		return nil, fmt.Errorf("archive.createPage: name must not be empty: %w", httpErrBadRequest)
	}
	if !pageIs(name) {
		name += pageExt
	}
	return a.AddFile(ctx, space, name, bytes.NewReader(structured.NewBlocks(pageSite).Snapshot()))
}

func (a *Archive) pageGet(ctx context.Context, f *File) (*pageDoc, error) {
	v, _ := a.pages.LoadOrStore(f.ID, &pageDoc{})
	p := v.(*pageDoc)
	p.mu.Lock()
	if p.doc != nil && p.hash == f.Hash {
		return p, nil
	}
	rc, err := a.blobs.open(ctx, f.Hash, f.Size)
	var b []byte
	if err == nil {
		b, err = io.ReadAll(rc)
		rc.Close()
	}
	var d *structured.Blocks
	if err == nil {
		if d, err = pageLoad(b); err != nil {
			err = fmt.Errorf("%v: %w", err, httpErrBadRequest)
		}
	}
	if err != nil {
		p.mu.Unlock()
		return nil, err
	}
	p.hash, p.snap, p.doc = f.Hash, b, d
	return p, nil
}

func (a *Archive) ReadPage(ctx context.Context, id uuid.UUID) (*OfficeModel, bool, error) {
	f, err := a.GetFile(ctx, id)
	if err != nil || !pageIs(f.Name) {
		return nil, err != nil, err
	}
	p, err := a.pageGet(ctx, f)
	if err != nil {
		return nil, true, fmt.Errorf("archive.read: %w", err)
	}
	defer p.mu.Unlock()
	return pageModel(p.doc), true, nil
}

func (a *Archive) editPage(ctx context.Context, f *File, ops []OfficeOp) (*File, error) {
	p, err := a.pageGet(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("archive.edit: %w", err)
	}
	defer p.mu.Unlock()
	d, err := pageLoad(p.snap)
	if err != nil {
		return nil, err
	}
	if err := pageApply(d, ops); err != nil {
		return nil, fmt.Errorf("archive.edit: %v: %w", err, httpErrBadRequest)
	}
	out := d.Snapshot()
	sum := sha256.Sum256(out)
	hash := hex.EncodeToString(sum[:])
	if err := a.pagePut(ctx, hash, out); err != nil {
		return nil, err
	}
	nf, err := storeOne[File](ctx, a.st, `UPDATE file SET hash = $2, size = $3, text = $4, status = 'done', error = ''
		WHERE id = $1 RETURNING `+fileCols, f.ID, hash, int64(len(out)), pageText(d))
	if err != nil {
		return nil, err
	}
	p.hash, p.snap, p.doc = hash, out, d
	return nf, nil
}

func (a *Archive) pagePut(ctx context.Context, hash string, b []byte) error {
	tmp, err := os.CreateTemp(a.uploads, "page-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(b)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return a.blobs.put(ctx, hash, tmp.Name(), int64(len(b)))
}

func pageApply(d *structured.Blocks, ops []OfficeOp) error {
	last := map[string]structured.BlockID{}
	for i, op := range ops {
		if err := pageOp(d, op, last); err != nil {
			return fmt.Errorf("edit %d (%s): %v", i, op.Op, err)
		}
	}
	return nil
}

func pageBlock(d *structured.Blocks, id string) (structured.Block, error) {
	if id == "" {
		return structured.Block{}, errors.New(`needs "para", a paragraph id from archive.read`)
	}
	for _, b := range d.List() {
		if b.ID.String() == id {
			return b, nil
		}
	}
	return structured.Block{}, fmt.Errorf("paragraph %s does not exist: it was deleted, or the id is not one archive.read returned", id)
}

func pageReplace(d *structured.Blocks, id structured.BlockID, from, n int, s string) error {
	if n > 0 {
		if _, err := d.DeleteText(id, from, n); err != nil {
			return err
		}
	}
	if s != "" {
		if _, err := d.InsertText(id, from, s); err != nil {
			return err
		}
	}
	return nil
}

func pageOp(d *structured.Blocks, op OfficeOp, last map[string]structured.BlockID) error {
	switch op.Op {
	case "replace_text":
		if op.Find == "" {
			if op.Para == "" {
				return errors.New(`replace_text needs "find", or "para" alone to replace the whole paragraph`)
			}
			b, err := pageBlock(d, op.Para)
			if err != nil || b.Text == op.Replace {
				return err
			}
			return pageReplace(d, b.ID, 0, utf8.RuneCountInString(b.Text), op.Replace)
		}
		var hits []structured.Block
		var ids []string
		for _, b := range d.List() {
			if (op.Para == "" || b.ID.String() == op.Para) && strings.Contains(b.Text, op.Find) {
				hits, ids = append(hits, b), append(ids, b.ID.String())
			}
		}
		switch {
		case len(hits) == 0 && op.Para != "":
			b, err := pageBlock(d, op.Para)
			if err != nil {
				return err
			}
			return fmt.Errorf("paragraph %s does not contain %q; its text is %q", op.Para, op.Find, b.Text)
		case len(hits) == 0:
			return fmt.Errorf("text %q does not occur in the page", op.Find)
		case len(hits) > 1 && !op.All:
			return fmt.Errorf("text %q occurs in %d paragraphs (%s); pass \"para\" or set \"all\": true", op.Find, len(hits), strings.Join(ids, ", "))
		}
		n := utf8.RuneCountInString(op.Find)
		for _, b := range hits {
			var at []int
			for i := 0; i <= len(b.Text); {
				j := strings.Index(b.Text[i:], op.Find)
				if j < 0 {
					break
				}
				at = append(at, i+j)
				if !op.All {
					break
				}
				i += j + len(op.Find)
			}
			for k := len(at) - 1; k >= 0; k-- {
				if err := pageReplace(d, b.ID, utf8.RuneCountInString(b.Text[:at[k]]), n, op.Replace); err != nil {
					return err
				}
			}
		}
		return nil
	case "insert_paragraph_after":
		at := structured.DocStart
		if op.Para != "" {
			b, err := pageBlock(d, op.Para)
			if err != nil {
				return err
			}
			at = b.ID
		}
		if prev, ok := last[op.Para]; ok {
			if _, ok := d.Block(prev); ok {
				at = prev
			}
		}
		id, _, err := d.Insert(at, cmp.Or(op.Style, "paragraph"))
		if err != nil {
			return err
		}
		last[op.Para] = id
		if op.Text != "" {
			_, err = d.InsertText(id, 0, op.Text)
		}
		return err
	case "delete_paragraph":
		b, err := pageBlock(d, op.Para)
		if err == nil {
			_, err = d.Remove(b.ID)
		}
		return err
	case "set_style":
		if op.Style == "" {
			return errors.New(`set_style needs "style", for example "heading1"`)
		}
		b, err := pageBlock(d, op.Para)
		if err == nil && b.Type != op.Style {
			_, err = d.SetType(b.ID, op.Style)
		}
		return err
	}
	return fmt.Errorf("unknown op %q for a page; page ops are %s", op.Op, pageOpsNames)
}
