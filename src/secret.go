package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const secretRefKey = "$secret"

var secretMasks = map[string]bool{"card": true, "mask": true, "mask_ssn": true, "national": true, "passport": true}

type secretRow struct {
	id    uuid.UUID
	typ   string
	value []byte
}

func secretRef(v any) (string, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", false
	}
	id, ok := m[secretRefKey].(string)
	return id, ok
}

func secretLoadKey(dir string) ([]byte, error) {
	if s := strings.TrimSpace(os.Getenv("ARCHIVE_SECRET_KEY")); s != "" {
		for _, dec := range []func(string) ([]byte, error){hex.DecodeString, base64.StdEncoding.DecodeString, base64.RawURLEncoding.DecodeString} {
			if k, err := dec(s); err == nil && len(k) == 32 {
				return k, nil
			}
		}
		return nil, errors.New("ARCHIVE_SECRET_KEY must be 32 bytes, hex or base64")
	}
	path := filepath.Join(dir, "secret.key")
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
		key := make([]byte, 32)
		rand.Read(key)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, err = f.WriteString(hex.EncodeToString(key) + "\n")
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			return key, err
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		b, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	k, err := hex.DecodeString(strings.TrimSpace(string(b)))
	if err != nil || len(k) != 32 {
		return nil, fmt.Errorf("%s must hold 32 hex-encoded bytes", path)
	}
	return k, nil
}

func secretAEAD(key []byte) (cipher.AEAD, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(c)
}

func secretDerive(key []byte, purpose string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(purpose))
	return m.Sum(nil)
}

func authPassword(key []byte) string { return hex.EncodeToString(secretDerive(key, "authenticator")) }

func (a *Archive) seal(spec *Spec, typ string, data, old map[string]any) ([]secretRow, error) {
	known := map[string]bool{}
	if old != nil {
		spec.eachSecret(typ, old, func(_ string, _ *Type, v any) (any, error) {
			if id, ok := secretRef(v); ok {
				known[id] = true
			}
			return v, nil
		})
	}
	var rows []secretRow
	err := spec.eachSecret(typ, data, func(path string, t *Type, v any) (any, error) {
		if id, ok := secretRef(v); ok {
			if !known[id] {
				return nil, &ValidationError{Type: typ, Errors: []FieldError{{Path: path, Message: "unknown secret reference"}}}
			}
			return v, nil
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		id, root := coreNewID(), spec.secretRoot(t.Name)
		rows = append(rows, secretRow{id, root, a.secretBox(id, root, b)})
		masked := "****"
		if secretMasks[t.Format] {
			masked = spec.Format(t.Name, v, "")
		}
		return map[string]any{secretRefKey: id.String(), "masked": masked}, nil
	})
	return rows, err
}

func (a *Archive) secretBox(id uuid.UUID, typ string, plain []byte) []byte {
	nonce := make([]byte, a.aead.NonceSize())
	rand.Read(nonce)
	return a.aead.Seal(nonce, nonce, plain, []byte(id.String()+typ))
}

func (a *Archive) secretOpen(id uuid.UUID, typ string, ct []byte) ([]byte, error) {
	n := a.aead.NonceSize()
	if len(ct) < n {
		return nil, fmt.Errorf("secret %s is corrupt", id)
	}
	b, err := a.aead.Open(nil, ct[:n], ct[n:], []byte(id.String()+typ))
	if err != nil {
		return nil, fmt.Errorf("secret %s: %w", id, err)
	}
	return b, nil
}

func (a *Archive) secretsPut(ctx context.Context, r *Entity, rows []secretRow) error {
	for _, s := range rows {
		_, err := a.st.Q(ctx).Exec(ctx, `INSERT INTO secret (id, entity_id, type, value, space) VALUES ($1, $2, $3, $4, $5)`,
			s.id, r.ID, s.typ, s.value, r.Space)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Archive) Reveal(ctx context.Context, rs ...*Entity) error {
	if len(rs) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(rs))
	for i, r := range rs {
		ids[i] = r.ID
	}
	return a.as(ctx, func(ctx context.Context) error {
		rows, _ := a.st.Q(ctx).Query(ctx, `SELECT id, entity_id, type, value FROM secret WHERE entity_id = ANY($1)`, ids)
		vals := map[string]any{}
		owner := map[string]uuid.UUID{}
		var id, rec uuid.UUID
		var typ string
		var ct []byte
		_, err := pgx.ForEachRow(rows, []any{&id, &rec, &typ, &ct}, func() error {
			b, err := a.secretOpen(id, typ, ct)
			if err != nil {
				return err
			}
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				return err
			}
			vals[id.String()], owner[id.String()] = v, rec
			return nil
		})
		if err != nil || len(vals) == 0 {
			return err
		}
		for _, r := range rs {
			var data any
			if err := json.Unmarshal(r.Data, &data); err != nil {
				return err
			}
			data = secretReplace(data, func(id string) (any, bool) {
				v, ok := vals[id]
				return v, ok && owner[id] == r.ID
			})
			if r.Data, err = json.Marshal(data); err != nil {
				return err
			}
		}
		return nil
	})
}

func secretReplace(v any, f func(id string) (any, bool)) any {
	switch x := v.(type) {
	case map[string]any:
		if id, ok := secretRef(x); ok {
			if nv, ok := f(id); ok {
				return nv
			}
			return x
		}
		for k, e := range x {
			x[k] = secretReplace(e, f)
		}
	case []any:
		for i, e := range x {
			x[i] = secretReplace(e, f)
		}
	}
	return v
}
