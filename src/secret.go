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
)

var secretMasks = map[string]bool{"card": true, "mask": true, "mask_ssn": true, "national": true, "passport": true}

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

func secretMask(s *semType, v any) string {
	if secretMasks[s.Format] {
		return semFormat(s, v, "")
	}
	return "****"
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

func (a *Archive) secretsPut(ctx context.Context, tb *Table, row map[string]any, seal map[string]any) error {
	id, space := coreRowID(row), coreUUID(row, "space")
	cat := a.Catalog()
	for _, col := range specKeys(seal) {
		q := a.st.db(ctx)
		if err := q.ClearSecrets(ctx, tb.Name, id, &col); err != nil {
			return err
		}
		if seal[col] == nil {
			continue
		}
		b, err := json.Marshal(seal[col])
		if err != nil {
			return err
		}
		sid, typ := coreNewID(), tb.col(col).Type
		if cat.secret(tb.col(col)) == nil {
			return fmt.Errorf("%s.%s is not a secret column", tb.Name, col)
		}
		if err := q.CreateSecret(ctx, CreateSecretParams{ID: sid, TableName: tb.Name, RowID: id, ColumnName: col, Type: typ, Value: a.secretBox(sid, typ, b), Space: space}); err != nil {
			return err
		}
	}
	return nil
}

func (a *Archive) Reveal(ctx context.Context, typ string, rows ...map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(rows))
	byID := map[uuid.UUID]map[string]any{}
	for i, r := range rows {
		ids[i] = coreRowID(r)
		byID[ids[i]] = r
	}
	return a.as(ctx, func(ctx context.Context) error {
		rows, err := a.st.db(ctx).Secrets(ctx, typ, ids)
		for _, r := range rows {
			b, err := a.secretOpen(r.ID, r.Type, r.Value)
			if err != nil {
				return err
			}
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				return err
			}
			byID[r.RowID][r.ColumnName] = v
		}
		return err
	})
}
