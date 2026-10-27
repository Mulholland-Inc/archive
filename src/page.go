package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-crdt/crdt"
	"github.com/go-crdt/crdt/awareness"
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
	subs map[chan string]struct{}
	who  *awareness.Registry
}

func (p *pageDoc) send(frame string) {
	for ch := range p.subs {
		select {
		case ch <- frame:
		default:
			delete(p.subs, ch)
			close(ch)
		}
	}
}

func (p *pageDoc) presence(u awareness.Update) {
	if p.who == nil {
		p.who = awareness.New()
	}
	if p.who.Apply(u) {
		p.announce(u)
	}
}

func (p *pageDoc) announce(u awareness.Update) {
	if b, err := u.MarshalBinary(); err == nil {
		p.send("event: presence\ndata: " + base64.StdEncoding.EncodeToString(b) + "\n\n")
	}
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
	nf, err := a.pageChange(ctx, f, func(d *structured.Blocks) error {
		if err := pageApply(d, ops); err != nil {
			return fmt.Errorf("archive.edit: %v: %w", err, httpErrBadRequest)
		}
		return nil
	})
	if err == nil {
		ac := ActorFrom(ctx)
		storeAfter(ctx, func() { a.pageAgent(f.ID, ac) })
	}
	return nf, err
}

func (a *Archive) pageAgent(id uuid.UUID, ac *Actor) {
	v, ok := a.pages.Load(id)
	if !ok {
		return
	}
	p := v.(*pageDoc)
	name := "Agent"
	if ac != nil && ac.Name != "" {
		name = ac.Name + " (agent)"
	}
	p.mu.Lock()
	if p.who == nil {
		p.who = awareness.New()
	}
	p.announce(p.who.Publish(pageSite, awareness.Cursor{Anchor: -1, Head: -1}, map[string]string{"name": name, "color": "#8e8e93", "agent": "1"}))
	p.mu.Unlock()
	time.AfterFunc(4*time.Second, func() {
		p.mu.Lock()
		p.announce(p.who.Leave(pageSite))
		p.mu.Unlock()
	})
}

func (a *Archive) PagePresence(ctx context.Context, id uuid.UUID, b []byte) error {
	var u awareness.Update
	if err := u.UnmarshalBinary(b); err != nil || u.Site <= pageSite {
		return fmt.Errorf("page presence: want an awareness update from a client site: %w", httpErrBadRequest)
	}
	f, err := coreDo(a, ctx, func(ctx context.Context) (*File, error) { return a.pageFile(ctx, id, false) })
	if err != nil {
		return err
	}
	p, err := a.pageGet(ctx, f)
	if err != nil {
		return err
	}
	p.presence(u)
	p.mu.Unlock()
	return nil
}

func (a *Archive) pageChange(ctx context.Context, f *File, change func(*structured.Blocks) error) (*File, error) {
	p, err := a.pageGet(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("archive.edit: %w", err)
	}
	defer p.mu.Unlock()
	d, err := pageLoad(p.snap)
	if err != nil {
		return nil, err
	}
	v := d.Version()
	if err := change(d); err != nil {
		return nil, err
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
	if ops := d.OpsSince(v); len(ops) > 0 {
		if b, err := crdt.AppendPartOps(nil, ops); err == nil {
			frame := "data: " + base64.StdEncoding.EncodeToString(b) + "\n\n"
			storeAfter(ctx, func() {
				p.mu.Lock()
				p.send(frame)
				p.mu.Unlock()
			})
		}
	}
	return nf, nil
}

func (a *Archive) pageFile(ctx context.Context, id uuid.UUID, lock bool) (*File, error) {
	q := `SELECT ` + fileCols + ` FROM file WHERE id = $1`
	if lock {
		q += ` FOR UPDATE`
	}
	f, err := storeOne[File](ctx, a.st, q, id)
	if err == nil && !pageIs(f.Name) {
		err = ErrNotFound
	}
	if errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("page %s: %w", id, ErrNotFound)
	}
	return f, err
}

func (a *Archive) PageOps(ctx context.Context, id uuid.UUID, b []byte) error {
	ops, err := crdt.ParsePartOps(b)
	if err != nil {
		return fmt.Errorf("page ops: %v: %w", err, httpErrBadRequest)
	}
	_, err = coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		f, err := a.pageFile(ctx, id, true)
		if err != nil {
			return nil, err
		}
		return a.pageChange(ctx, f, func(d *structured.Blocks) error {
			if err := d.Apply(ops...); err != nil {
				return fmt.Errorf("page ops: %v: %w", err, httpErrBadRequest)
			}
			return nil
		})
	})
	return err
}

func (a *Archive) pageSync(w http.ResponseWriter, r *http.Request) {
	id, err := httpID(r)
	var f *File
	if err == nil {
		f, err = coreDo(a, r.Context(), func(ctx context.Context) (*File, error) { return a.pageFile(ctx, id, false) })
	}
	var p *pageDoc
	if err == nil {
		p, err = a.pageGet(r.Context(), f)
	}
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	site, _ := strconv.ParseUint(r.URL.Query().Get("site"), 10, 64)
	ch := make(chan string, 256)
	if p.subs == nil {
		p.subs = map[chan string]struct{}{}
	}
	if p.who == nil {
		p.who = awareness.New()
	}
	p.subs[ch] = struct{}{}
	snap := p.snap
	if len(snap) == 0 {
		snap = p.doc.Snapshot()
	}
	var peers []string
	for _, u := range p.who.State() {
		if b, err := u.MarshalBinary(); err == nil {
			peers = append(peers, "event: presence\ndata: "+base64.StdEncoding.EncodeToString(b)+"\n\n")
		}
	}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.subs, ch)
		if site > uint64(pageSite) {
			p.announce(p.who.Leave(crdt.SiteID(site)))
		}
		p.mu.Unlock()
	}()
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	rc := http.NewResponseController(w)
	fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", base64.StdEncoding.EncodeToString(snap))
	io.WriteString(w, strings.Join(peers, ""))
	rc.Flush()
	tick := time.NewTicker(25 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			io.WriteString(w, ": ping\n\n")
		case frame, ok := <-ch:
			if !ok {
				return
			}
			io.WriteString(w, frame)
		}
		if rc.Flush() != nil {
			return
		}
	}
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
