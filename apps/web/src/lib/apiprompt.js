import { API } from './api.js';

// A prompt to paste into another assistant, so it can use this archive's API with the key.
export function apiPrompt({ key, archive }) {
  const base = API || 'http://localhost:8080';
  const scope = archive
    ? `This key belongs to one archive, "${archive.name}" (workspace \`${archive.id}\`), and acts with my access there.`
    : 'This key acts with my access.';

  return `You are helping me work with Archive, a database and file store for my records, through its HTTP API.

## Credentials
- Base URL: ${base}
- API key: ${key}
- Send it on every request: \`Authorization: Bearer ${key}\`
- ${scope}
- Never print the key back to me or put it in code you share; read it from an ARCHIVE_API_KEY environment variable in scripts.

## Endpoints
- \`GET /api/types\`: every table and view, with its columns, types and descriptions. Start here.
- \`GET /api/{table}?limit=&after=&<column>=<value>\`, \`GET /api/{table}/{id}\`: list and read rows.
- \`POST /api/{table}\`, \`PATCH /api/{table}/{id}\`, \`PUT /api/{table}/{id}\`, \`DELETE /api/{table}/{id}\`: write rows (JSON bodies).
- \`GET /api/search?q=&type=&limit=\`: full-text search across tables.
- \`GET /api/files\`, \`POST /api/files\` (multipart, field \`file\`), \`GET /api/files/{id}/content\`, \`GET /api/files/{id}/text\`: files and their text.
- \`POST /api/run\` with \`{ "code": "…" }\`: run TypeScript against the archive SDK; \`GET /api/run/docs\` documents it.
- \`GET /api/workflows\`, \`POST /api/workflows/{name}/run\`: workflows.
- MCP: \`${base}/mcp\` with the same key.

Errors come back as \`{ "error": "…" }\` with a 4xx or 5xx status.`;
}
