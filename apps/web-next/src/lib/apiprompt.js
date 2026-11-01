export function apiPrompt({ key, vault }) {
  const scope = vault
    ? `This key only works for the vault "${vault.name}" (id \`${vault.id}\`). Use \`${vault.id}\` wherever a path says \`{vault}\`.`
    : 'This key is a personal key: it works across every vault the owner belongs to, with their access. List vaults first and pick the one the task is about.';

  return `You are helping me work with Archive, a database and file store for my records, through its HTTP API.

## Credentials
- Base URL: https://api.archive.dev/v1
- API key: ${key}
- Send it on every request: \`Authorization: Bearer ${key}\`
- ${scope}
- Never print the key back to me or put it in code you share; read it from an ARCHIVE_API_KEY environment variable in scripts.

## Conventions
- JSON in and out, UTF-8. Timestamps are ISO 8601 in UTC.
- Lists are paginated: pass \`limit\` (max 200) and the \`cursor\` from the previous response's \`next\`.
- Errors return \`{ "error": { "code", "message" } }\` with a 4xx/5xx status. 429 means slow down; retry after the \`Retry-After\` seconds.

## Endpoints
### Vaults
- \`GET /vaults\` – vaults this key can reach: \`[{ id, name, role }]\`.
- \`GET /vaults/{vault}\` – name, members count, region, retention.

### Tables (the vault's database)
- \`GET /vaults/{vault}/tables\` – tables with their columns, types and links.
- \`GET /vaults/{vault}/tables/{table}\` – one table's schema.
- \`GET /vaults/{vault}/tables/{table}/rows?where=&order=&limit=\` – read rows. \`where\` uses \`column=value\`, \`column>value\`, joined with \`&\`.
- \`POST /vaults/{vault}/tables/{table}/rows\` – insert \`{ rows: [{ ... }] }\`; returns the new ids.
- \`PATCH /vaults/{vault}/tables/{table}/rows/{id}\` – update fields.
- \`DELETE /vaults/{vault}/tables/{table}/rows/{id}\` – delete a row.
- \`POST /vaults/{vault}/sql\` – run a read-only SQL query: \`{ "query": "select ..." }\`.
- \`POST /vaults/{vault}/migrate\` – change the schema with plain Postgres DDL: \`{ "sql": "alter table ..." }\`. Owners and editors only.

### Files (the library)
- \`GET /vaults/{vault}/files?folder=&q=&kind=\` – list and search files.
- \`POST /vaults/{vault}/files\` – multipart upload (\`file\`, optional \`folder\`).
- \`GET /vaults/{vault}/files/{id}\` – metadata; \`GET .../content\` for the bytes; \`GET .../text\` for extracted text or a transcript.

### Ask
- \`POST /vaults/{vault}/ask\` – ask a question over the vault's tables and files: \`{ "question": "..." }\`. Returns \`{ answer, sources: [{ type, id }] }\`.

### Workflows
- \`GET /vaults/{vault}/workflows\` – workflows with their last run.
- \`POST /vaults/{vault}/workflows/{id}/runs\` – start a run; \`GET .../runs/{run}\` for status and steps.

## How to help me
1. Start by calling \`GET /vaults\` and \`GET /vaults/{vault}/tables\` so you know what exists before writing anything.
2. Prefer reading and querying; ask me before deleting rows or changing the schema.
3. When you write code, use plain \`fetch\` (or Python \`requests\`), handle pagination and errors, and keep it short.

My first request:`;
}
