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
  <a href="#connect-your-agent">Connect your agent</a> ·
  <a href="#run-it-anywhere">Run it anywhere</a> ·
  <a href="docs/adr/0001-architecture.md">Architecture</a>
</p>

---

Archive is one open-source Go program on top of Postgres. Your records live in real Postgres tables and your files in Google Cloud Storage. The same rules guard all of it, whether it is read by you in the web app, by your code over the HTTP API, or by your AI agent over MCP.

Archive never calls an AI model on its own. Your agent does the thinking and sends Archive code to run.

## Shape your data with an agent

<img src="apps/site/static/art/schema.webp" alt="A blueprint being stamped into place." width="320">

Tell your agent what you keep track of, like clients, invoices and projects, and how they link. Every kind of record becomes a real Postgres table. Built-in types such as `archive.money`, `archive.phone`, `archive.email` and `archive.address` clean messy US input before it is stored. Secret types like card and passport numbers are encrypted and read back masked.

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

All or nothing, only the rows you may see (even with raw SQL), and a 30 second limit. A small `fetch` calls other services. The `run` tool's description is the full SDK reference plus a type for each of your tables, so your agent writes against your real schema. Scripts can also search every table and file, and edit Word, Excel and PowerPoint files in place.

Your agent can also save a script as a workflow that Archive runs on its own: on a schedule, when rows of a table are added or changed, or when its webhook URL is called. The first line says when:

```ts
// on: cron CRON_TZ=America/New_York 0 9 * * 1-5
return await archive.sql("SELECT number, customer, due FROM invoice WHERE NOT paid AND due < current_date");
```

The agent saves them with `archive.deploy(files)`, which returns compile and trigger errors instead of saving, and tries them on real data with `archive.test`, which undoes everything it wrote. A webhook workflow gets its own `https://api.<domain>/hooks/run/<workspace>/<secret>` address. Every run's result, logs and error show on the Workflows screen in the app.

## Connect it to everything else

<img src="apps/site/static/art/api.webp" alt="A switchboard connecting a terminal, a browser and an AI assistant to one place." width="320">

The same data and the same permissions, whichever way in:

| Way in | Good for |
|---|---|
| MCP at `/mcp` | Claude, ChatGPT, Gemini, Cursor and other agents, through one tool: `run` |
| HTTP API at `/api` | Records, files, search and workflows from your own apps and services |
| Webhooks at `/hooks/run/<workspace>/<secret>` | Other services starting a workflow with a POST |
| The `archive` CLI | Scripts, scheduled jobs and quick changes from your terminal |
| [Terraform provider](apps/terraform/provider) | Your schema, members and tokens, kept with the rest of your setup |

Upload any file and Archive pulls out its text, so one search covers every table and file you can see. Read-only connectors for mail, files, chat and business tools are designed in [ADR 0002](docs/adr/0002-connectors.md) and not built yet.

Everyone in a workspace is a `viewer`, an `editor` or an `owner`. Viewers read, editors also write, and owners also change the schema, manage people and see everything. Put people in groups, like `finance`, and set a row's `access` to `finance` so only that group and the owners see it; rows with no access are seen by the whole workspace. People, roles and groups are real Postgres roles, and every table has row-level security, so the database itself checks every row against who is asking. Archive is invite-only: an owner adds people by email on the Users page or with `archive user add`, and their first sign-in with Google, Microsoft, Slack or GitHub under that email, verified by the provider, links them. After that they can add a passkey to each device.

```sh
archive user add anna@example.com -name Anna -role editor   # Anna can now sign in
archive list member email=anna@example.com
archive set member <id> '{"groups":["finance"]}'
archive create invoice '{"number":"1042","access":"finance"}'
```

## Open source. Make it yours.

<img src="apps/site/static/art/factory.webp" alt="A factory turning a jumble of documents into neatly packed crates." width="320">

Run Archive in your own Google Cloud account, and keep your data where you want it. Change it or fork it anytime, by hand or by asking Claude Code to add what you need. Or [let us run it for you](https://mulholland.ai).

## Connect your agent

Make a token, then add Archive to Claude Code with its secret:

```sh
archive token create -name "Claude Code"
claude mcp add --transport http archive https://api.example.com/mcp --header "Authorization: Bearer <token>"
```

The token acts as you, with your role and groups. Add `-viewer` to make it read only: `archive token create -name Reports -viewer`.

<details>
<summary><b>Cursor, ChatGPT</b></summary>

**Cursor**: add this to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "archive": {
      "url": "https://api.example.com/mcp",
      "headers": { "Authorization": "Bearer <token>" }
    }
  }
}
```

**ChatGPT and claude.ai** need a connector that can send the bearer header. Archive has no OAuth sign-in yet.

</details>

## Run it anywhere

| Where | How |
|---|---|
| **Google Cloud** | [Terraform](apps/terraform/gcp) (Cloud Run, Cloud SQL, Storage), applied as it is or called as a module from your own setup |
| **As code** | the [`archive` Terraform provider](apps/terraform/provider) for your schema, members and tokens |

The API and the web app are always on addresses of their own: the `ghcr.io/mulholland-inc/archive` image serves the API, sign-in and MCP on Cloud Run, and the web app is static files on a Cloudflare Worker. The web app is on `app.<domain>` and the API on `api.<domain>`: the API is told `ARCHIVE_URL`, the web app's address, and the web app finds the API from its own.

On Google Cloud, everything is in Terraform, in one `apply`. You need a project with billing, Owner on it, Terraform 1.9 or later, and a domain on Cloudflare: the web app gets `app.<domain>` and the API `api.<domain>`.

```sh
gcloud auth application-default login
gcloud storage buckets create gs://<project>-tfstate --project <project> --location us-central1 --uniform-bucket-level-access --public-access-prevention && gcloud storage buckets update gs://<project>-tfstate --versioning
cd apps/terraform/gcp && terraform init -backend-config=bucket=<project>-tfstate
terraform apply -var project=<project> -var domain=example.com
```

Terraform keeps its state only in that versioned bucket, never on disk. The bucket is the one thing made outside Terraform, since the state can't live in a bucket the same state creates.

The Cloudflare account token goes in `CLOUDFLARE_API_TOKEN`. On the zone it needs DNS Write, Config Settings Write and Workers Routes Write; on the account it needs Workers Scripts Write and Account API Tokens Write. Terraform verifies the zone with Google, maps `api.<domain>` to the Cloud Run service behind Cloudflare's proxy, and makes the web app on `app.<domain>` a Worker of static files that calls the API on its own domain. The zone's own SSL settings don't matter: a Configuration Rule makes Cloudflare reach Google over verified TLS, and lets Google's certificate challenges through over HTTP.

The first apply takes about 15 minutes, nearly all of it Cloud SQL, and Google's certificate for `api.<domain>` 15 minutes or more after that. Then add yourself as the first owner with the `archive` command, which Terraform also sets up as a Cloud Run job (`terraform output cli` prints its prefix):

```sh
gcloud run jobs execute archive --region us-central1 --wait --args=user,add,you@example.com
```

The same job runs any other `archive` command. The service scales to zero: Cloud Tasks wakes it at the moment a workflow or file is due, so nothing has to stay running.

To deploy every image built on main, add `-var github_repository=owner/repo` with a `GITHUB_TOKEN` that can write that repository's Actions variables and secrets (`gh auth token` works for an admin). Apply before merging anything else, then run the image workflow once: until it runs, the app domain answers 503.

To add Archive to a Terraform setup you already have, call the same directory as a module. It sets up its own providers from `project` and `region`, with the same `CLOUDFLARE_API_TOKEN` and `GITHUB_TOKEN`, so it can't take `count`, `for_each` or `depends_on`:

```hcl
module "archive" {
  source          = "github.com/Mulholland-Inc/archive//apps/terraform/gcp"
  project = "my-project"
  domain  = "example.com"
}
```

It outputs `url`, `api_url`, `service`, `service_account`, `bucket`, `tasks_queue`, `database_connection_name` and `cli`. It makes its own Cloud SQL instance, bucket and Cloud Tasks queue, all named `archive`, and owns the zone's Configuration Rules.

The API image holds only the binary, built with [ko](https://ko.build) on a minimal base, and it needs a Postgres 18 (`DATABASE_URL`). With nothing else set, it keeps its key and files in `./data`. Give it storage (`ARCHIVE_BLOB_URL`) and one `ARCHIVE_SECRET_KEY`, and it keeps nothing local, so you can run as many copies side by side as you like.

One Archive serves any number of workspaces: one per organization, each its own Postgres database on the same server, with its own members, files and workflows, its own login role and its own data key. People are invited to workspaces (`archive user add`, or the Users page), and one sign-in reaches all of theirs. A new install starts with one. Instance admins (the owners of the first workspace) make and delete more, each with its first owner:

```sh
gcloud run jobs execute archive --region us-central1 --wait --args=workspace,add,Acme,anna@acme.example
gcloud run jobs execute archive --region us-central1 --wait --args=-workspace,<uuid>,list,member
```

A workspace's roles can reach only its own database, so workspaces stay isolated even where Postgres has no superuser, as on Cloud SQL. A token works in the one workspace it was made in, and the web app's one session switches between all of yours. [app.archive.dev](https://app.archive.dev) is this same image, deployed with [`apps/terraform/gcp`](apps/terraform/gcp) with `domain = "archive.dev"`.

Every setting is an environment variable; `archive help` lists them all:

| Variable | What it does |
|---|---|
| `ARCHIVE_URL` | Public base URL of the web app, for passkeys and where sign-in returns (default `http://localhost:5173`, the web app in development). The API, sign-in, MCP, webhooks, file links and OAuth redirects are on the same URL with `app.` made `api.`, or on `http://localhost:8080` while it is `http://localhost`; `/api` and `/auth` allow the web app's origin over CORS |
| `DATABASE_URL` | Postgres 18, required, with CREATEROLE and CREATEDB (no superuser needed); `docker compose up -d` starts one for development. Each workspace gets its own database on the same server |
| `ARCHIVE_SECRET_KEY` | 32-byte key for secret values; required with `DATABASE_URL`, the same on every copy |
| `ARCHIVE_BLOB_URL` | `gs://bucket/prefix` for files in GCS as the Cloud Run service account, with no key |
| `ARCHIVE_GOOGLE` | Sign-in with Google, as `https://<client id>:<client secret>@accounts.google.com` with the id and secret URL-escaped; each provider shows on the sign-in page when it is set, and its redirect URL is `https://api.<domain>/auth/oidc/<google\|microsoft\|slack\|github>/callback` |
| `ARCHIVE_MICROSOFT` | Sign-in with Microsoft, work and personal accounts: `https://<client id>:<client secret>@login.microsoftonline.com` |
| `ARCHIVE_SLACK` | Sign-in with Slack: `https://<client id>:<client secret>@slack.com` |
| `ARCHIVE_GITHUB` | Sign-in with GitHub: `https://<client id>:<client secret>@github.com` |
| `ANTHROPIC_API_KEY` | Key of the in-app agent, the one key for every workspace |

On Cloud Run, Archive finds its Cloud Tasks queue, `projects/<project>/locations/<region>/queues/<service>`, and the service account the queue signs its calls to `/tasks` with, `<service>-tasks@<project>.iam.gserviceaccount.com`, from the metadata server. Anywhere else, `archive serve` runs due work itself at its time, and when it starts it picks up what fell due while it was stopped.

## How it is built

```mermaid
flowchart LR
  Apps[ChatGPT, Claude, Gemini, Cursor] <--> MCP[MCP: run code] --> Core[Go core]
  Clients[Web app, CLI, Terraform] <--> API[HTTP API] --> Core
  Core --> PG[(Postgres: archive + public)]
  Core --> Files[(Files: disk or GCS)]
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
docker compose up -d                  # Postgres 18 on localhost:5433
```

```sh
cd src
go build -o archive .
go test ./...                         # end-to-end tests; each starts its own Postgres 18 with initdb and postgres from PATH
export DATABASE_URL="postgres://archive:archive@localhost:5433/archive?sslmode=disable"
ARCHIVE_BIN=./archive ../demo/load.sh you@example.com
./archive serve                       # API, sign-in and MCP on :8080, key and files in ./data; it runs workflows and files when they are due
```

`ARCHIVE_BIN=./archive ../demo/family/load.sh` loads a made-up family, five generations deep, for the family tree. Load it instead of the demo, not alongside it: both have a `person` table.

```sh
cd apps/web && npm install && npm run dev   # web app with hot reload on :5173, calling the API on :8080
```

While `ARCHIVE_URL` is `http://localhost…` (the default), requests from the same machine are signed in as the first owner, so local development needs no sign-in provider. Anything that arrives through a proxy, or for another host name, still signs in.

<p align="center"><sub>Made by <a href="https://mulholland.ai">Mulholland</a>.</sub></p>
