package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const inviteTTL = 7 * 24 * time.Hour

// Invite is a one-time link into one space at one level, for someone who may have no account yet.
type Invite struct {
	Email     string    `json:"email"`
	Name      string    `json:"name,omitempty"`
	Space     string    `json:"space"`
	Role      string    `json:"role"`
	InvitedBy string    `json:"invited_by"`
	ExpiresAt time.Time `json:"expires_at"`
	Link      string    `json:"link,omitempty"`
}

// Invite lets an admin of a space bring someone in at a level no higher than their own: an owner
// invites owners, an admin admins and below. Like membership, it needs a signed-in person, not an
// API token. The name, if given, becomes the account's name when the invite makes one. It returns
// the secret the link carries. A person's own home space takes no invites: it is theirs alone.
func (a *Archive) Invite(ctx context.Context, space, email, name, level string) (*Invite, string, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return nil, "", err
	}
	if authLevel(level) == 0 {
		return nil, "", &ValidationError{Type: "invite", Errors: []FieldError{{Path: "role", Message: "want owner, admin, editor or viewer"}}}
	}
	if email, err = authEmail(email); err != nil {
		return nil, "", err
	}
	need := "admin"
	if level == "owner" {
		need = "owner"
	}
	secret := authNewSecret()
	inv := &Invite{Email: email, Name: strings.TrimSpace(name), Space: space, Role: level, InvitedBy: ac.label()}
	return inv, secret, a.sys(ctx, func(ctx context.Context) error {
		base, err := a.spaceRole(ctx, space)
		if err != nil {
			return err
		}
		if !a.has(ctx, ac.UserRole, base+"_"+need) {
			return fmt.Errorf("inviting a %s needs %s in %s: %w", level, need, space, ErrForbidden)
		}
		var home bool
		if err := a.st.Q(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT FROM "user" u JOIN space s ON s.id = u.home WHERE s.name = $1)`, space).Scan(&home); err != nil {
			return err
		}
		if home {
			return fmt.Errorf("%s is someone's personal space, which takes no invites: %w", space, ErrConflict)
		}
		return a.st.Q(ctx).QueryRow(ctx, `INSERT INTO invite (space, email, name, level, invited_by, hash, expires_at)
			SELECT id, $2, $3, $4, $5, $6, now() + $7::interval FROM space WHERE name = $1 RETURNING expires_at`,
			space, email, inv.Name, level, inv.InvitedBy, authHash(secret), inviteTTL.String()).Scan(&inv.ExpiresAt)
	})
}

// PeekInvite says what an invite is for without using it, for the page its link opens.
func (a *Archive) PeekInvite(ctx context.Context, secret string) (*Invite, error) {
	inv := &Invite{}
	err := a.sys(ctx, func(ctx context.Context) error {
		return a.st.Q(ctx).QueryRow(ctx, `SELECT i.email, i.name, s.name, i.level, i.invited_by, i.expires_at FROM invite i JOIN space s ON s.id = i.space
			WHERE i.hash = $1 AND i.expires_at > now()`, authHash(secret)).Scan(&inv.Email, &inv.Name, &inv.Space, &inv.Role, &inv.InvitedBy, &inv.ExpiresAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("invite link is invalid, used or expired: %w", ErrUnauthorized)
	}
	return inv, err
}

// ErrSignInFirst is an invite for an address that has an account, used by someone not signed in
// as that account. The invite stays good, for them to use once they are.
var ErrSignInFirst = fmt.Errorf("this invite is for an address that already has an account: sign in as that address, then use the invite again: %w", ErrConflict)

// AcceptInvite uses an invite once and grants its level in the space, raising, never lowering,
// what the person already has. For an address with no account it makes one, and says so (made),
// so the caller signs the link holder in as it. For an address that has an account it only grants,
// and only when ctx's signed-in person is that account (ErrSignInFirst otherwise): holding the link
// is never a way into an account that exists, since whoever invited holds the link too. It is one
// transaction, so a failure leaves the invite unused.
func (a *Archive) AcceptInvite(ctx context.Context, secret string) (u *User, made bool, err error) {
	signedIn := uuid.Nil
	if ac := ActorFrom(ctx); ac != nil && ac.TokenID == uuid.Nil {
		signedIn = ac.UserID
	}
	err = a.sys(ctx, func(ctx context.Context) error {
		q := a.st.Q(ctx)
		var id uuid.UUID
		var email, name, level, base string
		err := q.QueryRow(ctx, `SELECT i.id, i.email, i.name, i.level, s.role FROM invite i JOIN space s ON s.id = i.space
			WHERE i.hash = $1 AND i.expires_at > now() FOR UPDATE OF i`, authHash(secret)).Scan(&id, &email, &name, &level, &base)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("invite link is invalid, used or expired: %w", ErrUnauthorized)
		}
		if err != nil {
			return err
		}
		u, err = a.userBy(ctx, `email = $1`, email)
		switch {
		case errors.Is(err, ErrNotFound):
			u, err = a.addUser(ctx, email, name, nil, false)
			made = true
		case err != nil:
		case u.ID != signedIn:
			return ErrSignInFirst
		case u.Name == "" && name != "":
			u.Name = name
			_, err = q.Exec(ctx, `UPDATE "user" SET name = $2 WHERE id = $1 AND name = ''`, u.ID, name)
		}
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `DELETE FROM invite WHERE id = $1`, id); err != nil {
			return err
		}
		if a.level(ctx, u.Role, base, true) >= authLevel(level) {
			return nil
		}
		return a.setLevel(ctx, u.Role, base, authLevel(level))
	})
	if err != nil {
		return nil, false, err
	}
	return u, made, nil
}

// inviteTmpl is an invite opened in a browser: who asked them in, and one button that uses it, so
// nothing that only follows the link (a mail scanner, a preview) spends it.
var inviteTmpl = template.Must(template.New("invite").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Join {{.Invite.Space}} on Archive</title>
<style>body{font:17px/1.5 system-ui,sans-serif;max-width:28rem;margin:18vh auto;padding:0 1rem;color:#1d1d1f;background:#fff}
@media (prefers-color-scheme:dark){body{color:#f5f5f7;background:#111}}button{font:inherit;padding:.6rem 1.2rem;border-radius:.6rem;border:0;background:#1d1d1f;color:#fff}
@media (prefers-color-scheme:dark){button{background:#f5f5f7;color:#111}}</style></head>
<body><h1>Join {{.Invite.Space}}</h1>
<p>{{.Invite.InvitedBy}} invited {{.Invite.Email}} to {{.Invite.Space}} on Archive, as {{.As}}.</p>
<form method="post" action="/auth/invite"><input type="hidden" name="token" value="{{.Token}}"><button>Join</button></form>
</body></html>
`))
