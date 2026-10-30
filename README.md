<p align="center">
  <img src="apps/site/static/art/landscape.webp" alt="A mailroom, a scanner and a building made of filing cabinets, joined by one glass tube." width="100%">
</p>

<h1 align="center">Archive</h1>

<h3 align="center">Own your data infrastructure.</h3>

<p align="center">
  Database. File storage. Agent access.<br>
  A foundation for your digital world, built to run on your terms.
</p>

<p align="center">
  <a href="https://archive.us.org">Website</a> ·
  <a href="https://archive.us.org/docs">Docs</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="#connect-your-agent">Connect your agent</a> ·
  <a href="#run-it-anywhere">Run it anywhere</a> ·
  <a href="docs/adr/0001-architecture.md">Architecture</a>
</p>

---

Archive is one open-source Go program on top of Postgres. Your records live in real Postgres tables and your files on disk or in S3-compatible storage. The same rules guard all of it, whether it is read by you in the web app, by your code over the HTTP API, or by your AI agent over MCP.

Archive never calls an AI model on its own. Your agent does the thinking and sends Archive code to run.

## Shape your data with an agent

<img src="apps/site/static/art/schema.webp" alt="A blueprint being stamped into place." width="320">

Tell your agent what you keep track of, like clients, invoices and projects, and how they link. Every kind of record becomes a real Postgres table. Built-in types such as `archive.money`, `archive.phone`, `archive.email` and `archive.address` clean messy input before it is stored, read in the locale you set with `ARCHIVE_LOCALE`. Secret types like card and passport numbers are encrypted and read back masked.

You see the plan before anything changes, and statements that delete data are blocked unless you allow them. The same flow works by hand:

```sh
archive schema pull ./schema   # every table and view as a .sql file
archive schema plan ./schema   # what would change
archive schema apply ./schema  # do it
```

The live database is the only source of truth, so there is no migrations folder. Pulling and applying straight back plans nothing, so the files can live in git.

## Your agent writes code. Archive runs it.

<img src="apps/site/static/art/code.webp" alt="A code editor whose lines snap neatly into place." width="320">

Instead of a dozen small tool calls, your agent writes one short script. Archive runs it inside, as you, in one transaction, and sends back one result:

```ts
const unpaid = await archive.list("invoice", { where: { status: "unpaid" } });
const total = unpaid.reduce((sum, i) => sum + i.amount.amount, 0);
return { count: unpaid.length, total };
```

```json
{"result": {"count": 3, "total": 6530}, "logs": [], "error": ""}
```

All or nothing, only your spaces (even with raw SQL), and a 30 second limit by default (`ARCHIVE_RUN_TIMEOUT`). A small `fetch` calls other services. The `run` tool's description is the full SDK reference plus a type for each of your tables, so your agent writes against your real schema. Scripts can also search every table and file, and edit Word, Excel and PowerPoint files in place.

Your agent can also save a script as a workflow that Archive runs on its own: on a schedule, when rows of a table are added or changed, or when its webhook URL is called. The first line says when:

```ts
// on: cron CRON_TZ=America/New_York 0 9 * * 1-5
return await archive.sql("SELECT number, customer, due FROM invoice WHERE NOT paid AND due < current_date");
```

The agent saves them with `archive.deploy(files)`, which returns compile and trigger errors instead of saving, and tries them on real data with `archive.test`, which undoes everything it wrote. A webhook workflow gets its own `ARCHIVE_API_URL/hooks/run/<workspace>/<secret>` address. Every run's result, logs and error show on the Workflows screen in the app.

## Connect it to everything else

<img src="apps/site/static/art/api.webp" alt="A switchboard connecting a terminal, a browser and an AI assistant to one place." width="320">

The same data and the same permissions, whichever way in:

| Way in | Good for |
|---|---|
| MCP at `/mcp` | Claude, ChatGPT, Gemini, Cursor and other agents, through one tool: `run` |
| HTTP API at `/api` | Records, files, search and workflows from your own apps and services |
| Webhooks at `/hooks/run/<workspace>/<secret>` | Other services starting a workflow with a POST |
| The `archive` CLI | Scripts, scheduled jobs and quick changes from your terminal |
| [Terraform provider](apps/terraform/provider) | Your schema, spaces, tokens and roles, kept with the rest of your setup |

Upload any file and Archive pulls out its text, so one search covers every table and file you can see. Read-only connectors for mail, files, chat and business tools are designed in [ADR 0002](docs/adr/0002-connectors.md) and not built yet.

Each team, client or project gets its own space, and people and their agents only see the spaces they are in. People, spaces and tokens are real Postgres roles, and every table has row-level security, so the database itself checks every row against who is asking. Roles are `viewer`, `editor`, `admin` and `owner`, and tokens only ever narrow what their owner can do. Archive is invite-only: an admin adds people by email on the Users page or with `archive user add`, and their first sign-in with Google, Microsoft, Slack or GitHub under that email, verified by the provider, links them. After that they can add a passkey to each device.

```sh
docker exec archive archive space add finance
docker exec archive archive user add anna@example.com -name Anna   # Anna can now sign in
docker exec archive archive space grant finance anna@example.com editor
```

## Open source. Make it yours.

<img src="apps/site/static/art/factory.webp" alt="A factory turning a jumble of documents into neatly packed crates." width="320">

Run Archive on your own computer, your own servers or your own cloud account, and keep your data where you want it. Change it or fork it anytime, by hand or by asking Claude Code to add what you need. Or [let us run it for you](https://mulholland.ai).

## Quick start

One line starts Archive on your machine with Docker, as three containers: Postgres, the API on port 8080 and the web app on port 3000.

```sh
curl -fsSL https://raw.githubusercontent.com/Mulholland-Inc/archive/main/install.sh | sh
docker exec archive archive user add you@example.com
```

Use your own email: the first person you add is the owner. Set a sign-in provider too, such as `ARCHIVE_GOOGLE_CLIENT_ID` and `ARCHIVE_GOOGLE_CLIENT_SECRET`, and sign in with it under that email; once in, add a passkey. Run the same line again to update. Your data stays in the `archive-postgres` and `archive` volumes.

From a clone of this repo, Docker Compose does the same:

```sh
docker compose up -d
docker compose exec archive archive user add you@example.com
```

Open `http://localhost:3000` for the web app. The API and MCP are on `http://localhost:8080`, where `/healthz` answers `204` when it is up.

## Connect your agent

Make a token, then add Archive to Claude Code with its secret:

```sh
docker exec archive archive token create -name "Claude Code"
claude mcp add --transport http archive http://localhost:8080/mcp --header "Authorization: Bearer <token>"
```

With no `-space`, the token gets your role in every space you are in. Narrow it with `-space finance:viewer,ops:editor`.

<details>
<summary><b>Cursor, Claude Desktop, ChatGPT</b></summary>

**Cursor**: add this to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "archive": {
      "url": "http://localhost:8080/mcp",
      "headers": { "Authorization": "Bearer <token>" }
    }
  }
}
```

**Claude Desktop**, on the machine that runs Archive, with no token, as the person you name:

```json
{
  "mcpServers": {
    "archive": {
      "command": "docker",
      "args": ["exec", "-i", "archive", "archive", "-as", "you@example.com", "mcp"]
    }
  }
}
```

**ChatGPT and claude.ai** run in the cloud, so they need a public HTTPS address and a connector that can send the bearer header. Archive has no OAuth sign-in yet.

</details>

## Run it anywhere

| Where | How |
|---|---|
| **Docker** | [`install.sh`](install.sh), or `docker compose up -d` from the code |
| **Cloud** | Terraform for [Google Cloud](apps/terraform/gcp) (Cloud Run, Cloud SQL, Storage) |
| **As code** | the [`archive` Terraform provider](apps/terraform/provider) for your schema, spaces, tokens and roles |

The API and the web app are separate images, always on addresses of their own: `ghcr.io/mulholland-inc/archive` serves the API, sign-in and MCP, and `ghcr.io/mulholland-inc/archive-web` serves the web app. Point them at each other with `ARCHIVE_URL` (the web app's address, on the API) and `ARCHIVE_API_URL` (the API's address, on both).

On Google Cloud, everything is in Terraform, in one `apply`. You need a project with billing, Owner on it, Terraform 1.9 or later, and a domain on Cloudflare: the API gets one subdomain and the web app another.

```sh
gcloud auth application-default login
cd apps/terraform/gcp && terraform init
terraform apply -var project=<project> -var app_domain=app.example.com -var api_domain=api.example.com -var cloudflare_zone=example.com
```

The Cloudflare account token goes in `CLOUDFLARE_API_TOKEN`. On the zone it needs DNS Write, Config Settings Write and Workers Routes Write; on the account it needs Workers Scripts Write and Account API Tokens Write. Terraform verifies the zone with Google, maps `api_domain` to the Cloud Run service behind Cloudflare's proxy, and makes the web app on `app_domain` a Worker of static files that calls the API on its own domain. The zone's own SSL settings don't matter: a Configuration Rule makes Cloudflare reach Google over verified TLS, and lets Google's certificate challenges through over HTTP.

The first apply takes about 15 minutes, nearly all of it Cloud SQL, and Google's certificate for `api_domain` 15 minutes or more after that. Then add yourself as the first owner with the `archive` command, which Terraform also sets up as a Cloud Run job (`terraform output cli` prints its prefix):

```sh
gcloud run jobs execute archive --region us-central1 --wait --args=user,add,you@example.com
```

The same job runs any other command, like `--args=workspace,add,Acme,anna@acme.example`. The service scales to zero: Cloud Tasks wakes it at the moment a workflow or file is due, so nothing has to stay running.

To deploy every image built on main, add `-var github_repository=owner/repo` with a `GITHUB_TOKEN` that can write that repository's Actions variables and secrets (`gh auth token` works for an admin). Apply before merging anything else, then run the image workflow once: until it runs, the app domain answers 503.

The API image holds only the binary, built with [ko](https://ko.build) on a minimal base, and it needs a Postgres 18 (`DATABASE_URL`). With nothing else set, it keeps its key and files in `ARCHIVE_DATA`. Give it storage (`ARCHIVE_BLOB_URL`) and one `ARCHIVE_SECRET_KEY`, and it keeps nothing local, so you can run as many copies side by side as you like. In production, put both behind HTTPS and set `ARCHIVE_URL` and `ARCHIVE_API_URL`.

One Archive serves any number of workspaces: one per organization, each its own Postgres database on the same server, with its own spaces, files and workflows, its own login role and its own data key. People are invited to workspaces (`archive user add`, or the Users page), and one sign-in reaches all of theirs. A new install starts with one. Instance admins (the admins of the first workspace) make and delete more, each with its first owner:

```sh
docker exec archive archive workspace add "Acme Inc" anna@acme.example
docker exec archive archive -workspace <uuid> space list
```

A workspace's roles can reach only its own database, so workspaces stay isolated even where Postgres has no superuser, as on Cloud SQL. A token works in the one workspace it was made in, and the web app's one session switches between all of yours. [app.archive.dev](https://app.archive.dev) is this same image, deployed with [`apps/terraform/gcp`](apps/terraform/gcp) with `app_domain` and `api_domain` set.

Every setting is an environment variable; `archive help` lists them all:

| Variable | What it does |
|---|---|
| `ARCHIVE_URL` | Public base URL of the web app, for passkeys and where sign-in returns (default `http://localhost:3000`) |
| `ARCHIVE_API_URL` | Public base URL of the API, sign-in, MCP, webhooks, file links and OAuth redirects, which holds the session cookie (default `http://localhost:8080`); `/api` and `/auth` allow `ARCHIVE_URL`'s origin over CORS. The web image reads it too |
| `ARCHIVE_DATA` | Directory for `secret.key` and files, while `ARCHIVE_SECRET_KEY` and `ARCHIVE_BLOB_URL` are unset (default `./data`) |
| `DATABASE_URL` | Postgres 18, required, with CREATEROLE and CREATEDB (no superuser needed); `docker compose up -d postgres` starts one. Each workspace gets its own database on the same server |
| `DATABASE_DIRECT_URL` | The same Postgres without a pooler in between, for schema changes (default `DATABASE_URL`) |
| `ARCHIVE_SECRET_KEY` | 32-byte key for secret values; required with `DATABASE_URL`, the same on every copy |
| `ARCHIVE_BLOB_URL`, `ARCHIVE_BLOB_KEY`, `ARCHIVE_BLOB_SECRET` | S3, R2 or MinIO for files, or `gs://bucket` for GCS as the Cloud Run service account, with no key |
| `ARCHIVE_GOOGLE_CLIENT_ID`, `ARCHIVE_GOOGLE_CLIENT_SECRET` | Sign-in with Google; each provider shows on the sign-in page when both of its variables are set, and its redirect URL is `ARCHIVE_API_URL/auth/oidc/<google\|microsoft\|slack\|github>/callback` |
| `ARCHIVE_MICROSOFT_CLIENT_ID`, `ARCHIVE_MICROSOFT_CLIENT_SECRET` | Sign-in with Microsoft, work and personal accounts |
| `ARCHIVE_SLACK_CLIENT_ID`, `ARCHIVE_SLACK_CLIENT_SECRET` | Sign-in with Slack |
| `ARCHIVE_GITHUB_CLIENT_ID`, `ARCHIVE_GITHUB_CLIENT_SECRET` | Sign-in with GitHub |
| `ANTHROPIC_API_KEY` | Key of the in-app agent; a workspace can set its own |
| `ARCHIVE_LOCALE` | Locale that messy input is read in |
| `ARCHIVE_RUN_TIMEOUT` | Time limit of one run (default `30s`) |

## How it is built

```mermaid
flowchart LR
  Apps[ChatGPT, Claude, Gemini, Cursor] <--> MCP[MCP: run code] --> Core[Go core]
  Clients[Web app, CLI, Terraform] <--> API[HTTP API] --> Core
  Core --> PG[(Postgres: archive + public)]
  Core --> Files[(Files: disk or S3)]
```

| Path | What lives there |
|---|---|
| [`src`](src) | The Go server and CLI: API, MCP, sign-in, schema, search, files, Office editing. `archive.sql` holds Archive's own schema and, after it, the sqlc queries; `generated` is the code sqlc makes from them |
| [`apps/web`](apps/web) | The Svelte 5 web app |
| [`apps/site`](apps/site) | [archive.us.org](https://archive.us.org) and the docs |
| [`apps/terraform`](apps/terraform) | The Google Cloud setup and the Terraform provider |
| [`demo`](demo) | A made-up personal archive: 18 tables, about 820 rows and 35 files to try it with |
| [`docs/adr`](docs/adr) | Architecture decisions |

## Develop

```sh
docker compose up -d postgres         # Postgres 18 on localhost:5433
cd src
go build -o archive .
go test ./...                         # end-to-end tests; each starts its own Postgres 18 with initdb and postgres from PATH
export DATABASE_URL="postgres://archive:archive@localhost:5433/archive?sslmode=disable" ARCHIVE_DATA=../data
ARCHIVE_BIN=./archive ../demo/load.sh you@example.com
ARCHIVE_URL=http://localhost:5173 ./archive serve  # API, sign-in and MCP on :8080
```

```sh
cd apps/web && npm install && npm run dev   # web app with hot reload on :5173, calling the API on :8080
```

While `ARCHIVE_URL` is `http://localhost…` (the default), requests from the same machine are signed in as the first owner, so local development needs no sign-in provider. Anything that arrives through a proxy, or for another host name, still signs in.

<p align="center"><sub>Made by <a href="https://mulholland.ai">Mulholland</a>.</sub></p>
