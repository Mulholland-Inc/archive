//go:build js

package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"syscall/js"

	"github.com/go-crdt/crdt"
	"github.com/go-crdt/crdt/awareness"
	"github.com/go-crdt/crdt/structured"
)

var (
	doc    *structured.Blocks
	outbox []crdt.PartOps
	site   crdt.SiteID
	who    = awareness.New()
)

func position(id string, offset int) int {
	b, err := block(id)
	if err != nil {
		return -1
	}
	pos, err := doc.At(b, offset)
	if err != nil {
		return -1
	}
	return pos
}

func locate(pos int) (string, int) {
	if pos < 0 {
		return "", 0
	}
	for _, b := range doc.List() {
		start, err := doc.At(b.ID, 0)
		if err != nil {
			continue
		}
		if n := len([]rune(b.Text)); pos >= start && pos <= start+n {
			return b.ID.String(), pos - start
		}
	}
	return "", 0
}

func block(id string) (structured.BlockID, error) {
	for _, b := range doc.List() {
		if b.ID.String() == id {
			return b.ID, nil
		}
	}
	return structured.BlockID{}, errors.New("no block " + id)
}

func queue(ops ...crdt.PartOps) { outbox = append(outbox, ops...) }

func setText(id, text string) error {
	b, err := block(id)
	if err != nil {
		return err
	}
	cur, _ := doc.Text(b)
	o, n := []rune(cur), []rune(text)
	p := 0
	for p < len(o) && p < len(n) && o[p] == n[p] {
		p++
	}
	s := 0
	for s < len(o)-p && s < len(n)-p && o[len(o)-1-s] == n[len(n)-1-s] {
		s++
	}
	if del := len(o) - p - s; del > 0 {
		op, err := doc.DeleteText(b, p, del)
		if err != nil {
			return err
		}
		queue(op)
	}
	if ins := string(n[p : len(n)-s]); ins != "" {
		op, err := doc.InsertText(b, p, ins)
		if err != nil {
			return err
		}
		queue(op)
	}
	return nil
}

func main() {
	api := js.Global().Get("Object").New()
	def := func(name string, f func(a []js.Value) (any, error)) {
		api.Set(name, js.FuncOf(func(_ js.Value, a []js.Value) any {
			v, err := f(a)
			if err != nil {
				return js.Global().Get("Error").New(err.Error())
			}
			b, _ := json.Marshal(v)
			return string(b)
		}))
	}
	def("load", func(a []js.Value) (any, error) {
		snap, err := base64.StdEncoding.DecodeString(a[0].String())
		if err == nil {
			site = crdt.SiteID(a[1].Int())
			doc, err = structured.LoadBlocks(site, snap)
		}
		outbox = nil
		return nil, err
	})
	def("blocks", func(a []js.Value) (any, error) {
		out := []map[string]any{}
		for _, b := range doc.List() {
			done, _ := doc.Field(b.ID, "done")
			out = append(out, map[string]any{"id": b.ID.String(), "type": b.Type, "text": b.Text, "done": string(done) == "1"})
		}
		return out, nil
	})
	def("setText", func(a []js.Value) (any, error) { return nil, setText(a[0].String(), a[1].String()) })
	def("insert", func(a []js.Value) (any, error) {
		after := structured.DocStart
		if id := a[0].String(); id != "" {
			b, err := block(id)
			if err != nil {
				return nil, err
			}
			after = b
		}
		id, ops, err := doc.Insert(after, a[1].String())
		queue(ops...)
		return id.String(), err
	})
	def("split", func(a []js.Value) (any, error) {
		b, err := block(a[0].String())
		if err != nil {
			return nil, err
		}
		id, ops, err := doc.Split(b, a[1].Int(), a[2].String())
		queue(ops...)
		return id.String(), err
	})
	def("merge", func(a []js.Value) (any, error) {
		b, err := block(a[0].String())
		if err != nil {
			return nil, err
		}
		ops, err := doc.Merge(b)
		queue(ops...)
		return nil, err
	})
	def("remove", func(a []js.Value) (any, error) {
		b, err := block(a[0].String())
		if err != nil {
			return nil, err
		}
		ops, err := doc.Remove(b)
		queue(ops...)
		return nil, err
	})
	def("setField", func(a []js.Value) (any, error) {
		b, err := block(a[0].String())
		if err != nil {
			return nil, err
		}
		op, err := doc.SetField(b, a[1].String(), []byte(a[2].String()))
		queue(op)
		return nil, err
	})
	def("setType", func(a []js.Value) (any, error) {
		b, err := block(a[0].String())
		if err != nil {
			return nil, err
		}
		op, err := doc.SetType(b, a[1].String())
		queue(op)
		return nil, err
	})
	def("apply", func(a []js.Value) (any, error) {
		b, err := base64.StdEncoding.DecodeString(a[0].String())
		var ops []crdt.PartOps
		if err == nil {
			ops, err = crdt.ParsePartOps(b)
		}
		if err == nil {
			err = doc.Apply(ops...)
		}
		return nil, err
	})
	def("flush", func(a []js.Value) (any, error) {
		if len(outbox) == 0 {
			return "", nil
		}
		b, err := crdt.AppendPartOps(nil, outbox)
		outbox = nil
		return base64.StdEncoding.EncodeToString(b), err
	})
	def("presence", func(a []js.Value) (any, error) {
		var meta map[string]string
		if err := json.Unmarshal([]byte(a[3].String()), &meta); err != nil {
			return nil, err
		}
		c := awareness.Cursor{Anchor: -1, Head: -1}
		if id := a[0].String(); id != "" {
			c = awareness.Cursor{Anchor: position(id, a[1].Int()), Head: position(id, a[2].Int())}
		}
		b, err := who.Publish(site, c, meta).MarshalBinary()
		return base64.StdEncoding.EncodeToString(b), err
	})
	def("applyPresence", func(a []js.Value) (any, error) {
		b, err := base64.StdEncoding.DecodeString(a[0].String())
		var u awareness.Update
		if err == nil {
			err = u.UnmarshalBinary(b)
		}
		if err == nil {
			who.Apply(u)
		}
		return nil, err
	})
	def("peers", func(a []js.Value) (any, error) {
		out := []map[string]any{}
		for _, p := range who.Peers() {
			if p.Site == site {
				continue
			}
			fb, fo := locate(p.Cursor.Anchor)
			hb, ho := locate(p.Cursor.Head)
			out = append(out, map[string]any{"site": uint64(p.Site), "name": p.Meta["name"], "color": p.Meta["color"], "agent": p.Meta["agent"] == "1",
				"anchorBlock": fb, "anchor": fo, "headBlock": hb, "head": ho})
		}
		return out, nil
	})
	js.Global().Set("pages", api)
	js.Global().Call("pagesReady")
	select {}
}
