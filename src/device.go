package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/google/uuid"
)

// The Archive app on an iPhone signs in with an OAuth token of Archive's, so the phone is the member's alone and what it sends is theirs.
// Each install of it is one personal connection, provider iphone, account the install's id. With ARCHIVE_IOS_APP set, each install
// also proves with App Attest that it is the real app on a real iPhone, and signs every request after with the key it attested.

const (
	deviceMax      = 32 << 20
	deviceFileMax  = 4 << 30
	deviceAttestAD = "app attest"
)

// deviceFiles are the fields of a stream's records the phone sends as base64 that the server keeps as files, by table: field, extension.
var deviceFiles = map[string][2]string{"photo": {"thumbnail", ".jpg"}, "health_workout": {"route", ".gpx"}}

type device struct {
	a      *Archive
	ac     *Actor
	c      generated.ArchiveConnection
	r      *http.Request
	signed bool
}

type deviceAttestation struct {
	Key     string `json:"key"`
	Public  string `json:"public"`
	Counter uint32 `json:"counter"`
	Env     string `json:"env"`
}

// device authenticates a request of the phone and finds its connection; signed, when the install must sign it with its attested key.
func (h *Hub) device(signed bool, f func(w http.ResponseWriter, d *device) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			httpWrite(w, 0, nil, fmt.Errorf("send Authorization: Bearer <token>, from signing in with the app: %w", ErrUnauthorized))
			return
		}
		ctx := r.Context()
		a, ac, release, err := h.authenticate(ctx, strings.TrimSpace(tok), r.Header.Get("Archive-Workspace"), "mcp", "api")
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		defer release()
		install := strings.TrimSpace(r.Header.Get("Archive-Device"))
		name := strings.Join(strings.Fields(r.Header.Get("Archive-Device-Name")), " ")
		if _, err := uuid.Parse(install); err != nil || len(name) > 200 {
			httpWrite(w, 0, nil, fmt.Errorf("send Archive-Device with the install's id and Archive-Device-Name with the phone's: %w", httpErrBadRequest))
			return
		}
		d := &device{a: a, ac: ac, r: r.WithContext(withArchive(WithActor(ctx, ac), a)), signed: signed && deviceApp() != ""}
		if err := a.sys(ctx, func(ctx context.Context) (err error) {
			d.c, err = a.st.db(ctx).DeviceConnection(ctx, ac.member, install, cmp.Or(name, "iPhone"))
			return err
		}); err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		if d.signed {
			if err := d.verify(); err != nil {
				httpWrite(w, 0, nil, err)
				return
			}
		}
		v, err := f(w, d)
		if v != nil || err != nil {
			httpWrite(w, http.StatusOK, v, err)
		}
	}
}

func deviceApp() string { return strings.TrimSpace(secretEnv("ARCHIVE_IOS_APP")) }

// read reads the body, up to max bytes, into a temporary file, and checks it is what the install signed, where it signs.
func (d *device) read(w http.ResponseWriter, max int64) (*os.File, error) {
	f, err := os.CreateTemp("", "archive-device-")
	if err != nil {
		return nil, err
	}
	os.Remove(f.Name())
	sum := sha256.New()
	if _, err = io.Copy(io.MultiWriter(f, sum), http.MaxBytesReader(w, d.r.Body, max)); err != nil {
		err = fmt.Errorf("%v: %w", err, httpErrBadRequest)
	} else {
		_, err = f.Seek(0, io.SeekStart)
	}
	if err == nil && d.signed && hex.EncodeToString(sum.Sum(nil)) != strings.ToLower(d.r.Header.Get("Archive-Body-SHA256")) {
		err = fmt.Errorf("the body is not what the app signed: %w", ErrUnauthorized)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func (d *device) readAll(w http.ResponseWriter) ([]byte, error) {
	f, err := d.read(w, deviceMax)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// verify checks Archive-Assertion: the install's App Attest assertion over the method, the path and Archive-Body-SHA256, with a
// counter past the last one Archive saw.
func (d *device) verify() error {
	var att deviceAttestation
	json.Unmarshal([]byte(d.c.Meta["attest"]), &att)
	refused := func(why string) error {
		return fmt.Errorf("App Attest: %s; the app attests its key again: %w", why, ErrUnauthorized)
	}
	if att.Public == "" {
		return refused("this install has attested no key")
	}
	raw, err := base64.StdEncoding.DecodeString(d.r.Header.Get("Archive-Assertion"))
	var as struct {
		Signature []byte `cbor:"signature"`
		AuthData  []byte `cbor:"authenticatorData"`
	}
	if err != nil || webauthncbor.Unmarshal(raw, &as) != nil || len(as.AuthData) < 37 {
		return refused("send Archive-Assertion")
	}
	app := sha256.Sum256([]byte(deviceApp()))
	if subtle.ConstantTimeCompare(as.AuthData[:32], app[:]) != 1 {
		return refused("the assertion is for another app")
	}
	n := binary.BigEndian.Uint32(as.AuthData[33:37])
	if n <= att.Counter {
		return refused("the assertion was used before")
	}
	client := sha256.Sum256([]byte(d.r.Method + " " + d.r.URL.Path + "\n" + strings.ToLower(d.r.Header.Get("Archive-Body-SHA256"))))
	nonce := sha256.Sum256(append(append([]byte{}, as.AuthData...), client[:]...))
	digest := sha256.Sum256(nonce[:])
	der, _ := base64.StdEncoding.DecodeString(att.Public)
	pub, err := x509.ParsePKIXPublicKey(der)
	ec, ok := pub.(*ecdsa.PublicKey)
	if err != nil || !ok || !ecdsa.VerifyASN1(ec, digest[:], as.Signature) {
		return refused("the signature does not match")
	}
	att.Counter = n
	return d.saveAttest(att)
}

func (d *device) saveAttest(att deviceAttestation) error {
	b, _ := json.Marshal(att)
	return d.a.sys(d.r.Context(), func(ctx context.Context) error {
		return d.a.st.db(ctx).DeviceAttest(ctx, d.c.ID, string(b))
	})
}

// deviceChallenge is a one-off challenge for the install to attest its key with, good for five minutes.
func (h *Hub) deviceChallenge(_ http.ResponseWriter, d *device) (any, error) {
	b := make([]byte, 40)
	rand.Read(b[:32])
	binary.BigEndian.PutUint64(b[32:], uint64(time.Now().Add(5*time.Minute).Unix()))
	nonce := make([]byte, h.aead.NonceSize())
	rand.Read(nonce)
	return map[string]any{"challenge": base64.RawURLEncoding.EncodeToString(h.aead.Seal(nonce, nonce, b, []byte(deviceAttestAD+d.c.ID.String()))),
		"required": deviceApp() != ""}, nil
}

// deviceAttest checks an App Attest attestation as Apple says to, and keeps the key it attests as the install's.
func (h *Hub) deviceAttest(w http.ResponseWriter, d *device) (any, error) {
	var body struct {
		Key         string `json:"key"`
		Attestation string `json:"attestation"`
		Challenge   string `json:"challenge"`
	}
	b, err := d.readAll(w)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &body); err != nil {
		return nil, fmt.Errorf("want { key, attestation, challenge }: %v: %w", err, httpErrBadRequest)
	}
	refused := func(why string) error { return fmt.Errorf("App Attest: %s: %w", why, ErrUnauthorized) }
	app := deviceApp()
	if app == "" {
		return nil, fmt.Errorf("App Attest is off here; set ARCHIVE_IOS_APP: %w", ErrNotFound)
	}
	sealed, err := base64.RawURLEncoding.DecodeString(body.Challenge)
	ns := h.aead.NonceSize()
	var opened []byte
	if err == nil && len(sealed) > ns {
		opened, err = h.aead.Open(nil, sealed[:ns], sealed[ns:], []byte(deviceAttestAD+d.c.ID.String()))
	}
	if err != nil || len(opened) != 40 || time.Now().Unix() > int64(binary.BigEndian.Uint64(opened[32:])) {
		return nil, refused("the challenge is not Archive's or ran out; get another")
	}
	key, err1 := base64.StdEncoding.DecodeString(body.Key)
	raw, err2 := base64.StdEncoding.DecodeString(body.Attestation)
	var att struct {
		Fmt  string `cbor:"fmt"`
		Stmt struct {
			X5c [][]byte `cbor:"x5c"`
		} `cbor:"attStmt"`
		AuthData []byte `cbor:"authData"`
	}
	if errors.Join(err1, err2) != nil || webauthncbor.Unmarshal(raw, &att) != nil || att.Fmt != "apple-appattest" || len(att.Stmt.X5c) < 2 || len(att.AuthData) < 55 {
		return nil, refused("want an apple-appattest attestation")
	}
	certs := make([]*x509.Certificate, len(att.Stmt.X5c))
	for i, der := range att.Stmt.X5c {
		if certs[i], err = x509.ParseCertificate(der); err != nil {
			return nil, refused("a certificate does not parse")
		}
	}
	mid := x509.NewCertPool()
	for _, c := range certs[1:] {
		mid.AddCert(c)
	}
	if _, err := certs[0].Verify(x509.VerifyOptions{Roots: deviceRoots, Intermediates: mid, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
		return nil, refused("the certificates are not Apple's")
	}
	client := sha256.Sum256([]byte(body.Challenge))
	nonce := sha256.Sum256(append(append([]byte{}, att.AuthData...), client[:]...))
	var got struct {
		Nonce []byte `asn1:"tag:1,explicit"`
	}
	matched := false
	for _, ext := range certs[0].Extensions {
		if ext.Id.Equal(asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 8, 2}) {
			_, err := asn1.Unmarshal(ext.Value, &got)
			matched = err == nil && subtle.ConstantTimeCompare(got.Nonce, nonce[:]) == 1
		}
	}
	pub, ok := certs[0].PublicKey.(*ecdsa.PublicKey)
	if !matched || !ok {
		return nil, refused("the attestation is not for this challenge")
	}
	point, err := pub.ECDH()
	if err != nil {
		return nil, refused("the key is not a P-256 key")
	}
	id := sha256.Sum256(point.Bytes())
	ad := att.AuthData
	rp := sha256.Sum256([]byte(app))
	credLen := int(binary.BigEndian.Uint16(ad[53:55]))
	env := map[string]string{"appattestdevelop": "development", "appattest\x00\x00\x00\x00\x00\x00\x00": "production"}[string(ad[37:53])]
	switch {
	case !bytes.Equal(id[:], key):
		return nil, refused("the key is not the one attested")
	case !bytes.Equal(ad[:32], rp[:]):
		return nil, refused("the attestation is for another app than " + app)
	case binary.BigEndian.Uint32(ad[33:37]) != 0:
		return nil, refused("the key was used before it was attested")
	case env == "":
		return nil, refused("the attestation is from no App Attest environment")
	case len(ad) < 55+credLen || !bytes.Equal(ad[55:55+credLen], key):
		return nil, refused("the attestation is for another key")
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, err
	}
	if err := d.saveAttest(deviceAttestation{Key: body.Key, Public: base64.StdEncoding.EncodeToString(der), Env: env}); err != nil {
		return nil, err
	}
	return map[string]any{"attested": true, "env": env}, nil
}

// deviceRecords lands a batch of records of one stream of connect.Phone as a sync of the install's connection.
func (h *Hub) deviceRecords(w http.ResponseWriter, d *device) (any, error) {
	s := connect.PhoneStream(d.r.PathValue("stream"))
	if s == nil {
		return nil, fmt.Errorf("no stream %q: %w", d.r.PathValue("stream"), ErrNotFound)
	}
	b, err := d.readAll(w)
	if err != nil {
		return nil, err
	}
	var body struct {
		Records []connect.Record `json:"records"`
		Full    bool             `json:"full"`
		More    bool             `json:"more"`
	}
	if err := json.Unmarshal(b, &body); err != nil {
		return nil, fmt.Errorf("want { records: [{ id, data, deleted }], full, more }: %v: %w", err, httpErrBadRequest)
	}
	a, c, ctx := d.a, d.c, d.r.Context()
	if f, ok := deviceFiles[s.Table]; ok {
		store := a.syncStore(ctx, c.ID, c.Access)
		for i, rec := range body.Records {
			data, _ := rec.Data.(map[string]any)
			v, _ := data[f[0]].(string)
			if v == "" {
				continue
			}
			raw, err := base64.StdEncoding.DecodeString(v)
			if err == nil {
				data[f[0]], err = store(rec.ID+f[1], raw)
			}
			if err != nil {
				return nil, fmt.Errorf("record %d, its %s: %v: %w", i, f[0], err, httpErrBadRequest)
			}
		}
	}
	err = a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		now, err := q.TxNow(ctx)
		if err != nil {
			return err
		}
		if c, err = q.ConnectionLocked(ctx, c.ID); err != nil {
			return err
		}
		st := syncStateOf(c.Cursor)
		cur := st.of(s.Name).fresh(s)
		cur.Def, st[s.Name] = s.Version, cur
		sw := &syncDB{q: q, db: a.st.Q(ctx), id: c.ID, acc: c.Access, now: now, sync: a.Catalog().sync}
		if err := syncSave(ctx, sw, s, &connect.Page{Records: body.Records, Full: body.Full, More: body.More}, cur); err != nil {
			return err
		}
		if !body.More {
			if err := sw.count(ctx, syncStreams(c.Provider)); err != nil {
				return err
			}
		}
		cur.Last = &now
		cursor, _ := json.Marshal(st)
		return q.SaveSync(ctx, generated.SaveSyncParams{ID: c.ID, Cursor: cursor, SyncedAt: &now})
	})
	if err != nil {
		return nil, err
	}
	if len(body.Records) > 0 {
		a.wakeTables(context.WithValue(context.WithoutCancel(ctx), storeTxKey{}, nil))
	}
	return map[string]any{"connection": c.ID, "records": len(body.Records)}, nil
}

// deviceOriginal keeps a photo or video itself, full size, for a photo the phone synced already: ?photo=<asset id>&name=<file name>,
// in the query since Photos' ids have slashes.
func (h *Hub) deviceOriginal(w http.ResponseWriter, d *device) (any, error) {
	f, err := d.read(w, deviceFileMax)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	id, name := d.r.URL.Query().Get("photo"), path.Base(cmp.Or(d.r.URL.Query().Get("name"), "original"))
	a, c, ctx := d.a, d.c, d.r.Context()
	var file *File
	var n int64
	err = a.sys(ctx, func(ctx context.Context) error {
		var err error
		file, err = a.fileCommit(ctx, c.Access, name, nil, &c.ID, func() (io.ReadCloser, error) {
			_, err := f.Seek(0, io.SeekStart)
			return io.NopCloser(f), err
		})
		if err == nil {
			n, err = a.st.db(ctx).PhotoOriginal(ctx, c.ID, id, &file.ID)
		}
		return err
	})
	if err == nil && n == 0 {
		err = fmt.Errorf("no photo %s synced from this iPhone yet; push it to photos first: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	a.wakeTables(context.WithValue(context.WithoutCancel(ctx), storeTxKey{}, nil))
	return map[string]any{"file": file.ID}, nil
}

// deviceSave keeps what the member shared to Archive from another app, like a PDF or a photo, in their files, as an upload in the web app would.
func (h *Hub) deviceSave(w http.ResponseWriter, d *device) (any, error) {
	f, err := d.read(w, deviceFileMax)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	name := path.Base(cmp.Or(strings.TrimSpace(d.r.URL.Query().Get("name")), "Shared from iPhone"))
	file, err := d.a.fileAdd(d.r.Context(), nil, name, nil, f)
	if err != nil {
		return nil, err
	}
	return map[string]any{"file": file.ID, "name": file.Name}, nil
}

// deviceAsk asks the workspace's first agent, or the one named, as the web app's chat does, streaming its answer back as NDJSON.
func (h *Hub) deviceAsk(w http.ResponseWriter, d *device) (any, error) {
	b, err := d.readAll(w)
	if err != nil {
		return nil, err
	}
	var body struct {
		Agent uuid.UUID `json:"agent"`
		Text  string    `json:"text"`
	}
	if err := json.Unmarshal(b, &body); err != nil {
		return nil, fmt.Errorf("want { text, agent }: %v: %w", err, httpErrBadRequest)
	}
	if body.Agent == uuid.Nil {
		agents, err := d.a.List(d.r.Context(), "agent", nil, 1, uuid.Nil, false)
		if err != nil {
			return nil, err
		}
		if len(agents) == 0 {
			return nil, fmt.Errorf("this workspace has no agent to ask: %w", ErrNotFound)
		}
		body.Agent, _ = uuid.Parse(fmt.Sprint(agents[0]["id"]))
	}
	b, _ = json.Marshal(body)
	r := d.r.Clone(d.r.Context())
	r.Body, r.ContentLength = io.NopCloser(bytes.NewReader(b)), int64(len(b))
	r.Header.Set("Content-Type", "application/json")
	d.a.turn(w, r)
	return nil, nil
}

// deviceRoots is Apple's App Attestation Root CA, from https://www.apple.com/certificateauthority/Apple_App_Attestation_Root_CA.pem.
var deviceRoots = func() *x509.CertPool {
	pool := x509.NewCertPool()
	block, _ := pem.Decode([]byte(deviceRootPEM))
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		panic(err)
	}
	pool.AddCert(c)
	return pool
}()

const deviceRootPEM = `-----BEGIN CERTIFICATE-----
MIICITCCAaegAwIBAgIQC/O+DvHN0uD7jG5yH2IXmDAKBggqhkjOPQQDAzBSMSYw
JAYDVQQDDB1BcHBsZSBBcHAgQXR0ZXN0YXRpb24gUm9vdCBDQTETMBEGA1UECgwK
QXBwbGUgSW5jLjETMBEGA1UECAwKQ2FsaWZvcm5pYTAeFw0yMDAzMTgxODMyNTNa
Fw00NTAzMTUwMDAwMDBaMFIxJjAkBgNVBAMMHUFwcGxlIEFwcCBBdHRlc3RhdGlv
biBSb290IENBMRMwEQYDVQQKDApBcHBsZSBJbmMuMRMwEQYDVQQIDApDYWxpZm9y
bmlhMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAERTHhmLW07ATaFQIEVwTtT4dyctdh
NbJhFs/Ii2FdCgAHGbpphY3+d8qjuDngIN3WVhQUBHAoMeQ/cLiP1sOUtgjqK9au
Yen1mMEvRq9Sk3Jm5X8U62H+xTD3FE9TgS41o0IwQDAPBgNVHRMBAf8EBTADAQH/
MB0GA1UdDgQWBBSskRBTM72+aEH/pwyp5frq5eWKoTAOBgNVHQ8BAf8EBAMCAQYw
CgYIKoZIzj0EAwMDaAAwZQIwQgFGnByvsiVbpTKwSga0kP0e8EeDS4+sQmTvb7vn
53O5+FRXgeLhpJ06ysC5PrOyAjEAp5U4xDgEgllF7En3VcE3iexZZtKeYnpqtijV
oyFraWVIyd/dganmrduC1bmTBGwD
-----END CERTIFICATE-----
`
