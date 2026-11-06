package connect

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
)

// Thrift's binary protocol over HTTP, as Evernote speaks it: a call is one message POSTed, its answer one message back.
// Calls are written from TStruct, TList and Go's plain values; answers read into TReply, a struct's fields by id, with
// strings and binary as string, lists and sets as []any, maps as map[any]any and structs as TReply.

type (
	TField struct {
		ID int16
		V  any
	}
	TStruct []TField
	TList   struct {
		Of    byte
		Items []any
	}
	TReply map[int16]any
)

const (
	tStop   byte = 0
	tBool   byte = 2
	tByte   byte = 3
	tDouble byte = 4
	tI16    byte = 6
	tI32    byte = 8
	tI64    byte = 10
	tString byte = 11
	tStruct byte = 12
	tMap    byte = 13
	tSet    byte = 14
	tList   byte = 15

	tCall      = 1
	tReplyMsg  = 2
	tException = 3
	tVersion   = 0x80010000
	tMost      = 64 << 20
	tDeep      = 64
)

// ThriftCall calls method at u with args and gives back its result struct: field 0 is what it returns, the others the
// exceptions it declares.
func ThriftCall(ctx context.Context, hc *http.Client, u, method string, args TStruct) (TReply, error) {
	var w bytes.Buffer
	binary.Write(&w, binary.BigEndian, uint32(tVersion|tCall))
	tWriteString(&w, method)
	binary.Write(&w, binary.BigEndian, int32(1))
	if err := tWrite(&w, args); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, &w)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-thrift")
	req.Header.Set("Accept", "application/x-thrift")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, &Refusal{Status: resp.StatusCode, Message: string(bytes.TrimSpace(b))}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, tMost+1))
	if err != nil {
		return nil, err
	}
	if len(b) > tMost {
		return nil, fmt.Errorf("%s answered more than %d MB", method, tMost>>20)
	}
	r := &tReader{b: b}
	head := r.u32()
	if head&0xffff0000 != tVersion {
		return nil, fmt.Errorf("%s answered something other than Thrift", method)
	}
	name, _ := r.str()
	r.u32()
	if r.err != nil {
		return nil, r.err
	}
	if name != method {
		return nil, fmt.Errorf("%s was answered as %s", method, name)
	}
	body := r.value(tStruct, 0)
	if r.err != nil {
		return nil, fmt.Errorf("%s's answer does not read: %w", method, r.err)
	}
	out, _ := body.(TReply)
	switch byte(head) {
	case tReplyMsg:
		return out, nil
	case tException:
		return nil, fmt.Errorf("%s failed: %s", method, out.Str(1))
	}
	return nil, fmt.Errorf("%s answered with message type %d", method, byte(head))
}

func tWriteString(w *bytes.Buffer, s string) {
	binary.Write(w, binary.BigEndian, int32(len(s)))
	w.WriteString(s)
}

func tTypeOf(v any) (byte, error) {
	switch v.(type) {
	case bool:
		return tBool, nil
	case int8:
		return tByte, nil
	case int16:
		return tI16, nil
	case int32:
		return tI32, nil
	case int64:
		return tI64, nil
	case float64:
		return tDouble, nil
	case string, []byte:
		return tString, nil
	case TStruct:
		return tStruct, nil
	case TList:
		return tList, nil
	}
	return 0, fmt.Errorf("thrift: cannot write a %T", v)
}

func tWrite(w *bytes.Buffer, v any) error {
	be := binary.BigEndian
	switch v := v.(type) {
	case bool:
		if v {
			w.WriteByte(1)
		} else {
			w.WriteByte(0)
		}
	case int8:
		w.WriteByte(byte(v))
	case int16, int32, int64:
		binary.Write(w, be, v)
	case float64:
		binary.Write(w, be, math.Float64bits(v))
	case string:
		tWriteString(w, v)
	case []byte:
		binary.Write(w, be, int32(len(v)))
		w.Write(v)
	case TStruct:
		for _, f := range v {
			if f.V == nil {
				continue
			}
			t, err := tTypeOf(f.V)
			if err != nil {
				return err
			}
			w.WriteByte(t)
			binary.Write(w, be, f.ID)
			if err := tWrite(w, f.V); err != nil {
				return err
			}
		}
		w.WriteByte(tStop)
	case TList:
		w.WriteByte(v.Of)
		binary.Write(w, be, int32(len(v.Items)))
		for _, it := range v.Items {
			if err := tWrite(w, it); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("thrift: cannot write a %T", v)
	}
	return nil
}

type tReader struct {
	b   []byte
	at  int
	err error
}

func (r *tReader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || r.at+n > len(r.b) {
		r.err = io.ErrUnexpectedEOF
		return nil
	}
	b := r.b[r.at : r.at+n]
	r.at += n
	return b
}

func (r *tReader) u32() uint32 {
	if b := r.take(4); b != nil {
		return binary.BigEndian.Uint32(b)
	}
	return 0
}

func (r *tReader) str() (string, bool) {
	n := int(int32(r.u32()))
	b := r.take(n)
	return string(b), b != nil
}

// size reads a container's length, which cannot be more than the bytes left, as each item, even
// a bool or a byte, takes one at least.
func (r *tReader) size() int {
	n := int(int32(r.u32()))
	if r.err == nil && (n < 0 || n > len(r.b)-r.at) {
		r.err = errors.New("a container longer than the message")
		return 0
	}
	return n
}

func (r *tReader) value(t byte, depth int) any {
	if depth > tDeep {
		r.err = errors.New("nested too deep")
	}
	if r.err != nil {
		return nil
	}
	switch t {
	case tBool:
		b := r.take(1)
		return b != nil && b[0] != 0
	case tByte:
		if b := r.take(1); b != nil {
			return int8(b[0])
		}
	case tI16:
		if b := r.take(2); b != nil {
			return int16(binary.BigEndian.Uint16(b))
		}
	case tI32:
		return int32(r.u32())
	case tI64:
		if b := r.take(8); b != nil {
			return int64(binary.BigEndian.Uint64(b))
		}
	case tDouble:
		if b := r.take(8); b != nil {
			return math.Float64frombits(binary.BigEndian.Uint64(b))
		}
	case tString:
		s, _ := r.str()
		return s
	case tStruct:
		out := TReply{}
		for r.err == nil {
			ft := r.take(1)
			if ft == nil || ft[0] == tStop {
				break
			}
			id := r.take(2)
			if id == nil {
				break
			}
			out[int16(binary.BigEndian.Uint16(id))] = r.value(ft[0], depth+1)
		}
		return out
	case tList, tSet:
		et := r.take(1)
		if et == nil {
			return nil
		}
		n := r.size()
		out := make([]any, 0, n)
		for i := 0; i < n && r.err == nil; i++ {
			out = append(out, r.value(et[0], depth+1))
		}
		return out
	case tMap:
		kv := r.take(2)
		if kv == nil {
			return nil
		}
		n := r.size()
		out := make(map[any]any, n)
		for i := 0; i < n && r.err == nil; i++ {
			k, v := r.value(kv[0], depth+1), r.value(kv[1], depth+1)
			if _, ok := k.(TReply); !ok {
				out[k] = v
			}
		}
		return out
	default:
		r.err = fmt.Errorf("unknown type %d", t)
	}
	return nil
}

func (t TReply) Str(id int16) string { s, _ := t[id].(string); return s }

func (t TReply) Bool(id int16) bool { b, _ := t[id].(bool); return b }

func (t TReply) Struct(id int16) TReply { s, _ := t[id].(TReply); return s }

func (t TReply) List(id int16) []any { l, _ := t[id].([]any); return l }

func (t TReply) Int(id int16) int64 {
	switch v := t[id].(type) {
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	}
	return 0
}

func (t TReply) Has(id int16) bool { _, ok := t[id]; return ok }

// Strs is a list of strings, like guids.
func (t TReply) Strs(id int16) []string {
	var out []string
	for _, v := range t.List(id) {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Structs is a list of structs.
func (t TReply) Structs(id int16) []TReply {
	var out []TReply
	for _, v := range t.List(id) {
		if s, ok := v.(TReply); ok {
			out = append(out, s)
		}
	}
	return out
}
