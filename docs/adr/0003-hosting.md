# ADR 0003: Hosting app.archive.dev

- Status: Accepted
- Date: 2026-10-30
- Deciders: Marijn Brussel

## Summary

app.archive.dev is Archive hosted by Mulholland. It runs the same image from the same open repo as every self-hosted Archive. There is no private repo, no hosted build and no mode switch. What makes it the hosted version is its environment variables (the API keys filled in, the sign-in providers, its two domains) and rows in its database (workspaces).

To get there, Archive serves many workspaces by default. A workspace is one customer's Archive: its own Postgres database, holding its tables, spaces, files and workflows. People are not per workspace: each has one identity, which can belong to several workspaces. A self-hosted install is the same code with one workspace.

```mermaid
flowchart LR
  Browser[app.archive.dev] --> Run
  Clients[api.archive.dev: API, MCP, webhooks, OAuth callbacks] --> Run
  Run[One Cloud Run service, any number of instances] --> Main[(Main database: workspaces, users, tokens, due work)]
  Run --> W1[(Workspace database)]
  Run --> W2[(Workspace database)]
  Run --> Blobs[(One bucket, a prefix per workspace)]
```

## Workspaces

- **A database per workspace.** Each workspace's types are its own tables with their own columns, policies and schema changes (ADR 0001), so workspaces cannot share tables with a `workspace_id` column. Every workspace gets its own database on the same Postgres server: the bundled one, or Cloud SQL.
- **Roles are prefixed per workspace.** Postgres roles are server-wide, so each workspace's roles are prefixed by its id.
- **The main database** is the one `DATABASE_URL` (or the bundled Postgres) points to. It is also the first workspace's database. In its `archive` schema it adds:

| Table | Holds |
|---|---|
| `archive.workspace` | id (a UUIDv7), name, data key (encrypted with `ARCHIVE_SECRET_KEY`), its own Anthropic key, created_at |
| `archive.user` | every person once: email, name, passkeys and OIDC links, and the workspaces they belong to (`workspaces uuid[]`, GIN-indexed) |
| `archive.token` | sessions and API tokens, by hash; an API token also names its one workspace |
| `archive.due` | the next time each workspace has background work |

- **Each workspace has its own data key**, wrapped by `ARCHIVE_SECRET_KEY`, for its secret fields and file links.
- **Files** go to the same blob store under the prefix `<id>/`.

The workspace's id is a UUIDv7, used everywhere: its database is `archive_<id>`, its roles are prefixed `"<id>_"`, its blobs live under `<id>/`, and webhook and file-link URLs, the `-workspace` flag and the `Archive-Workspace` header all carry it. There is no separate short id. Every generated identifier is quoted.

### Why a database, not one schema per workspace

One shared database with a schema per workspace was tried, because it needs one connection pool instead of a pool per workspace, and an empty schema costs kilobytes of catalog rows where `CREATE DATABASE` copies a template of about 8 MB. It was dropped because it could not isolate tenants on our managed hosts:

- **No superuser.** On Cloud SQL and AlloyDB, Archive's database role has CREATEROLE, CREATEDB and LOGIN and owns nothing in `pg_catalog`. Hiding one schema's objects from another needs revoking `SELECT` on the catalog tables and `EXECUTE` on `set_config`, plpgsql and advisory-lock functions from `PUBLIC`. Those revokes require ownership of `pg_catalog`; on Cloud SQL the plpgsql revoke errors and the function revokes are silent no-ops.
- **Catalog leaks that cannot be closed.** Even with the revokes, `16384::regclass`, `::regrole`, `::regnamespace` and the `pg_get_*` and `obj_description` functions name objects in any schema by OID. Their output functions cannot be revoked and the casts cannot be dropped (`DROP CAST` is refused as required by the system), so one tenant's SQL can still enumerate another's table, column and index names.
- **A role-switch hole.** A single shared login role, needed because connections cannot cross schemas cheaply otherwise, can `SET ROLE` into any workspace's roles, reaching another tenant's rows.

A database per workspace sidesteps all three: Postgres keeps each database's catalog private, and a per-workspace login that can connect only to its own database closes the role-switch hole, with no superuser-only revoke anywhere.

### Growing past one server

A workspace is a database of about a hundred catalog objects; a few thousand per Postgres server is comfortable before `pg_database` and shared-catalog contention matter. Past that, or for a customer who needs it: give `archive.workspace` a server column and route each workspace's connections by it; move a workspace with `pg_dump` of its database plus its `archive` rows; put big customers on a server of their own. Sharding across servers is not built.

## Hosts

- **app.archive.dev** serves the web app and sign-in.
- **api.archive.dev** serves the HTTP API, MCP, workflow webhooks and every OAuth redirect URL: connectors (ADR 0002) and sign-in with Google and Microsoft. These are the URLs registered and approved with each vendor, so they never change and don't depend on the workspace.
- `ARCHIVE_URL` is the app's address. A new `ARCHIVE_API_URL` is the API's, and defaults to `ARCHIVE_URL`, so a self-hosted install keeps using one host for everything.
- Both hosts point at the same Cloud Run service. No subdomain per workspace, no wildcard certificates.

## Finding the workspace

Without subdomains, every request names its workspace, and secrets stay plain `arc_<secret>`:

- **Every secret is checked in the main database**, by one indexed lookup of its hash in `archive.token`. An API or agent token belongs to a user and one workspace, so it names that workspace itself, and a request naming another is refused.
- **The web app** holds one session for the person, in its cookie, and names the current workspace on each request in `Archive-Workspace`, the way it already names the current space; the server checks that the person belongs to it. The space switcher is a workspace and space switcher.
- **Sign-in** looks the person up in `archive.user`: by the provider's account once linked, and on the first sign-in by the verified email they were invited under; by user id for a passkey. One passkey per device and one session reach all their workspaces.
- **Webhook and file-link URLs** carry it: `api.archive.dev/hooks/run/<workspace>/<token>`.
- **OAuth callbacks** carry it in `state`, so one fixed callback URL serves every workspace.

## Serving many workspaces

- **Open on first use.** An instance opens a workspace (its small connection pool, table list and data key) on its first request, and closes it after it has been idle for a while. Each open workspace keeps a few connections; no pooler sits in front of Postgres.
- **Schema upgrades.** After a deploy, an instance upgrades the main database first. Each workspace is upgraded when it is next opened, under the existing advisory lock and revision check. A background pass upgrades idle workspaces, so first requests rarely wait.
- **Background work from one queue.** Workflows (schedules and table triggers) and pending files write their next due time to `archive.due` in the main database. Instances claim due rows with `FOR UPDATE SKIP LOCKED`, open that workspace and do the work. No instance polls every workspace.
- **Limits per workspace**: concurrent runs, so one busy workspace cannot starve the others on a shared instance.

## Invites

- **Invite-only.** There is no signup. Instance admins create and delete workspaces for other people (`archive workspace add|delete`), naming each one's first owner; creating one makes the database, applies the schema, adds the `archive.workspace` row and adds the owner to it. Workspace admins invite the rest by email. A provider sign-in only finds people who were invited, by the verified email they were invited under. Deleting a workspace drops its database and its roles.
- **AI keys**: the shared `ANTHROPIC_API_KEY` serves every workspace. A workspace can bring its own key in a setting.

## Infrastructure

`apps/terraform/gcp` deploys app.archive.dev exactly as it deploys anyone's Archive: one Cloud Run service, Cloud SQL reached directly over its socket, a bucket, and the secrets as environment variables. Mulholland's values (domains, keys) are Terraform variables, not code.

- **Domains go through a Cloudflare Worker.** Cloud Run picks the service by Host header, so DNS alone cannot point a domain at it. Cloud Run's domain mapping is preview and not meant for production, a Google load balancer costs about $18 a month, and Cloudflare Origin Rules rewrite the Host header only on Enterprise. A Worker of a few lines, on Workers' free plan, fetches the same path from the service's run.app address. Its custom domains create the DNS records and certificates, and Cloudflare's protection sits in front. Without domains there is no Worker and the run.app address is the app's.
- **The image comes through Artifact Registry.** Cloud Run cannot pull from ghcr.io, so a remote repository mirrors it. The ghcr.io package has to be public.
- **Every image on main deploys itself.** The image workflow updates the service to that commit's image through workload identity federation. Terraform makes that identity and sets the repository's Actions variables naming it, and ignores the image after creating the service.
- **Everything is in Terraform**, with its state kept locally where it runs. The Cloudflare account and zone are looked up from the zone's name.

## How it was built

Built as decided; [ADR 0001](0001-architecture.md#workspaces) describes the result. Where the build differs from the above, or chose where it was silent:

- **One `archive.sql` for every database.** The instance tables are part of Archive's schema, so they exist, empty, in every workspace's database too; only the main database's are used. One schema keeps one revision hash and one upgrade path.
- **Billing was removed.** Plans with monthly limits, usage counting in `archive.usage`, Stripe Checkout and the Stripe webhook were built, then removed: instance admins make the workspaces, so there is nothing to sell or meter.
- **The workspace id is a UUIDv7.** There is no short id. The database is `archive_<id>`, the roles are prefixed `"<id>_"` (quoted, since a UUID is not a bare identifier), the blobs live under `<id>/`, and URLs and the `Archive-Workspace` header carry it. Workspace one lives in the main database (`DATABASE_URL`). Role names fit Postgres's 63-byte limit: a user or token role is `<id>_u<base64url>` / `<id>_t<base64url>`, and a space is `<id>_s<random>_<level>`.
- **Two walls of isolation, neither needing a superuser.** A workspace's roles can CONNECT only to its own database (its authenticator is granted CONNECT there and `PUBLIC` is revoked), and its authenticator's password derives from that workspace's own data key, so no shared login can reach another workspace. Postgres keeps each database's `pg_class`, `pg_attribute` and the rest to itself, so one workspace's raw SQL and psql cannot see another's table names, columns, comments, view definitions or rows. Within a database, row-level security and the space roles filter further. A workspace's authenticator is a member only of that workspace's roles, so `SET ROLE` or `set_config('role', …)` into another workspace's role is refused by Postgres. All of this works where Archive's database role has only CREATEROLE, CREATEDB and LOGIN, as on Cloud SQL and AlloyDB.
- **Still shared across workspaces:** `pg_roles` and `pg_database` are server-wide, so a workspace's SQL can see that other databases and roles exist. Both are only UUIDs (`archive_<id>`, `"<id>_…"`), so they reveal counts, not names or data.
- **Both hosts answer every route.** The server does not look at the host. The URLs it hands out use the right one: the web app and passkeys `ARCHIVE_URL`; webhooks, file links and sign-in redirects `ARCHIVE_API_URL`. The web app on app.archive.dev calls `/api` on its own host, so its cookie stays first-party.
- **One identity per person.** `archive.user` and `archive.token` live in the main database only; a workspace database keeps its data and the Postgres roles row-level security needs, and its rows refer to people by their global user id, with no foreign key across databases. Adding someone to a workspace adds it to their `workspaces` and creates their role (`<id>_u<user>`) and personal space there once; removing them drops that role and their tokens, chats and workflows there, and keeps the personal space for when they come back. A session works in every workspace the person is in, and the server picks the first one when a request names none. The sign-in screen never tells anyone which workspaces an email belongs to. A provider's callback on the API host redirects to `ARCHIVE_URL/auth/oidc/done` with an encrypted hand-off, valid for a minute, that only works in the browser that started (a cookie bound into it). A passkey's user handle is the user id.
- **The archive tables the system writes** (`space`, `file`, `secret`) have row-level security enabled but not forced, so the role that owns them — the one `DATABASE_URL` connects as, never a tenant — bypasses it for background work and admin cleanup, while tenant roles stay filtered by the policies. This replaces relying on a superuser to bypass, which Cloud SQL does not offer. Customer tables keep forced row-level security, so removing a space deletes their rows as the removing owner's role: the system role grants itself that role inside the transaction, sets it for the deletes, and revokes it before committing. No other system work touches customer tables.
- **Instance admins** are the admins of workspace one, which cannot be deleted.
- **Limits per workspace** are concurrent runs (8) and small pools that close when idle. Memory is not limited per workspace.
- **No pooler.** Each instance holds a small pool per open workspace and talks to Postgres directly; `DATABASE_DIRECT_URL` stays available for a deployment that puts a transaction pooler in `DATABASE_URL`, since creating databases and schema changes need a direct session. Archive sets each database's default `search_path` and `timezone`.
- **Workspace one stays open** in every instance; other workspaces close after 10 idle minutes.

## Not doing

- A private repo, a hosted build, or code that only runs on app.archive.dev.
- Feature flags or modes: a single-workspace install is the same code with one row.
- Shared tables with a `workspace_id` column, or one shared database with a schema per workspace (see above).
- A Cloud Run service, database server or subdomain per workspace.

## Open questions

- **License.** The repo has no LICENSE yet. AGPL-3.0 makes anyone offering Archive as a service publish their changes. FSL forbids competing hosted offerings for two years, after which each release becomes Apache 2.0.
- **MCP sign-in for AI apps.** Claude and ChatGPT connect MCP servers through OAuth. api.archive.dev would need to be an OAuth authorization server, and the person would pick a workspace while approving.
- **Deleting and exporting a workspace**: drop its database and blob prefix, and offer a `pg_dump` plus files first.
