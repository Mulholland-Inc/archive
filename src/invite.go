package archive

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"time"

	"github.com/jackc/pgx/v5"
)

const inviteTTL = 7 * 24 * time.Hour

// Invite is a single-use link into one space at one level, for someone who may have no account.
type Invite struct {
	Email     string    `json:"email"`
	Space     string    `json:"space"`
	Role      string    `json:"role"`
	InvitedBy string    `json:"invited_by"`
	ExpiresAt time.Time `json:"expires_at"`
	Link      string    `json:"link,omitempty"`
}

// Invite lets an admin of a space bring someone into it at a level no higher than their own:
// owners invite owners, admins invite admins and below. Like membership, it needs a signed-in
// person, not an API token. It returns the secret the link carries.
func (a *Archive) Invite(ctx context.Context, space, email, level string) (*Invite, string, error) {
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
	inv := &Invite{Email: email, Space: space, Role: level, InvitedBy: ac.label()}
	return inv, secret, a.sys(ctx, func(ctx context.Context) error {
		base, err := a.spaceRole(ctx, space)
		if err != nil {
			return err
		}
		if !a.has(ctx, ac.UserRole, base+"_"+need) {
			return fmt.Errorf("inviting a %s needs %s in %s: %w", level, need, space, ErrForbidden)
		}
		return a.st.Q(ctx).QueryRow(ctx, `INSERT INTO invites (space, email, level, hash, invited_by, expires_at)
			SELECT id, $2, $3, $4, $5, now() + $6::interval FROM spaces WHERE name = $1 RETURNING expires_at`,
			space, email, level, authHash(secret), inv.InvitedBy, inviteTTL.String()).Scan(&inv.ExpiresAt)
	})
}

// PeekInvite says what an invite is for without using it, for the page its link opens.
func (a *Archive) PeekInvite(ctx context.Context, secret string) (*Invite, error) {
	inv := &Invite{}
	err := a.sys(ctx, func(ctx context.Context) error {
		return a.st.Q(ctx).QueryRow(ctx, `SELECT i.email, s.name, i.level, i.invited_by, i.expires_at FROM invites i JOIN spaces s ON s.id = i.space
			WHERE i.hash = $1 AND i.expires_at > now()`, authHash(secret)).Scan(&inv.Email, &inv.Space, &inv.Role, &inv.InvitedBy, &inv.ExpiresAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("invite link is invalid, used or expired: %w", ErrUnauthorized)
	}
	return inv, err
}

// AcceptInvite uses an invite once: it makes the user if the address has none, grants the level in
// the space (raising, never lowering, what they already have), and returns the user to sign in.
// It is one transaction, so a failure leaves the invite unused.
func (a *Archive) AcceptInvite(ctx context.Context, secret string) (*User, error) {
	var u *User
	err := a.sys(ctx, func(ctx context.Context) error {
		q := a.st.Q(ctx)
		var email, level, base string
		err := q.QueryRow(ctx, `DELETE FROM invites i USING spaces s WHERE s.id = i.space AND i.hash = $1 AND i.expires_at > now()
			RETURNING i.email, i.level, s.role`, authHash(secret)).Scan(&email, &level, &base)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("invite link is invalid, used or expired: %w", ErrUnauthorized)
		}
		if err != nil {
			return err
		}
		if u, err = a.userBy(ctx, `email = $1`, email); errors.Is(err, ErrNotFound) {
			u, err = a.addUser(ctx, email, "", false, false)
		}
		if err != nil {
			return err
		}
		if a.level(ctx, u.Role, base, true) >= authLevel(level) {
			return nil
		}
		return a.setLevel(ctx, u.Role, base, authLevel(level))
	})
	return u, err
}

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
