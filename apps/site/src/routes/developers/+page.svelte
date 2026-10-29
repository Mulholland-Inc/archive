<script>
  import Art from '#lib/Art.svelte';
  import Button from '#lib/Button.svelte';
  import Comp from '#lib/Comp.svelte';
  import Cta from '#lib/Cta.svelte';
  import Facts from '#lib/Facts.svelte';
  import Faq from '#lib/Faq.svelte';
  import Hero from '#lib/Hero.svelte';
  import Section from '#lib/Section.svelte';
  import TextBlock from '#lib/TextBlock.svelte';
  import { reveal } from '#lib/reveal.js';

  const API = [
    ['`GET /api/types`', 'Every table and view with its columns, comments and references.'],
    ['`GET /api/{type}`', 'List rows, oldest first. Filter with `column=value`, page with `limit` and `after`, add `reveal=true` for granted secrets.'],
    ['`POST /api/{type}`', 'Create a row, cleaned and validated per field, in your space or `?space=`.'],
    ['`GET /api/{type}/{id}`', 'One row.'],
    ['`PATCH /api/{type}/{id}`', 'Change some fields; `null` clears one.'],
    ['`PUT /api/{type}/{id}`', 'Replace the whole row.'],
    ['`DELETE /api/{type}/{id}`', 'Delete a row.'],
    ['`GET /api/search`', 'Full-text search over rows and files with `q`, `type`, `space`, `from`, `to` and `limit`.'],
    ['`GET|POST /api/files`', 'List files, or upload one as multipart form data. Large files use resumable tus uploads at `/api/files/tus`.'],
    ['`GET /api/files/{id}/content`', "The file's bytes. `/text` gives its extracted text, `/thumbnail` an image preview."],
    ['`POST /api/files/{id}/link`', 'A short-lived signed download link.'],
    ['`GET /api/schema`', 'DDL generated from the database, one `<name>.sql` per table or view.'],
    ['`POST /api/schema/plan`', 'Plan the whole set of files against the database, with hazards per statement.'],
    ['`POST /api/schema/apply`', 'Apply that plan. Takes `renames` and `allow_drop`.'],
    ['`POST /api/run`', 'Run TypeScript against the SDK in one transaction; returns result, logs and error.'],
    ['`GET /api/run/docs`', 'The SDK docs, with an interface per table.'],
    ['`GET /api/pages/{id}/sync`', "A server-sent event stream: the page, then everyone's operations and presence."],
    ['`POST /api/pages/{id}/ops`', "Send a tab's go-crdt operations; `/presence` sends a caret, a name and a colour."],
    ['`/auth/spaces`, `/auth/tokens`', 'Spaces, members and scoped tokens.']
  ];

  const VERBS = [
    ['`sql(query, ...params)`', 'Run one SQL statement as you, with `$1`, `$2` bound to params.'],
    ['`list(table, opts)`', 'Rows of a table, oldest first, filtered by `where` and paged with `after`.'],
    ['`get(table, id)`', 'One row by id.'],
    ['`create(table, data)`', 'Insert a cleaned, validated row. `create("Notes.page")` makes an empty page, docx, xlsx or pptx.'],
    ['`update(table, id, patch)`', 'Change the given fields; `null` clears one.'],
    ['`delete(table, id)`', 'Delete a row.'],
    ['`search(text, opts)`', 'Full-text search over every table and file you can see, best first.'],
    ['`schema()`', 'DDL generated from the database, one file per table or view.'],
    ['`apply(files, opts)`', 'Plan the whole set of DDL files; with `confirm`, apply it.'],
    ['`tables()`', 'Every table and view with its columns, types, comments and references.'],
    ['`files(opts)`', 'Files you can see, oldest first, with their extracted text.'],
    ['`file(id)`', 'One file with its extracted text.'],
    ['`read(fileId)`', 'The structure of a docx, xlsx, pptx or page: paragraphs, cells, slides and shapes.'],
    ['`edit(fileId, edits)`', 'Apply a list of edits to a document, all or nothing.'],
    ['`spaces()`', 'The spaces you can see, with your role in each.']
  ];

  const CLI = [
    ['`archive run <file.ts|->`', 'Run TypeScript against the SDK in one transaction. `-docs` prints the SDK docs.'],
    ['`archive types`', 'Print the tables, their columns and the built-in types.'],
    ['`archive schema pull [dir]`', 'Print the generated DDL, or write one `<name>.sql` per table or view into dir.'],
    ['`archive schema plan <dir>`', 'Show the statements that bring the database to the files in dir.'],
    ['`archive schema apply <dir>`', 'Run that plan. `-allow-drop` lets statements that delete data through.'],
    ['`archive list <table>`', 'List rows with `column=value` filters, `-limit`, `-after` and `-reveal`.'],
    ['`archive get <table> <id>`', 'One row.'],
    ['`archive create <table> [json]`', 'Create a row from the argument or stdin, in `-space` or your own.'],
    ['`archive set | update`', 'Set some columns, or replace the whole row.'],
    ['`archive delete <table> <id>`', 'Delete a row.'],
    ['`archive search <query>`', 'Find rows and files by their words, with `-type`, `-space`, `-from` and `-to`.'],
    ['`archive file add|get|list`', 'Store files and extract their text, print their metadata, or write their content.'],
    ['`archive space add|list|members|grant|remove`', 'Make spaces and set who holds which role in them.'],
    ['`archive token create|list|revoke`', 'Scoped tokens with a role per space and an expiry.'],
    ['`archive secret grant|revoke`', 'Let a person read one secret type, like `us_ssn` or `passport`.']
  ];

  const FAQ = [
    ['Which AI assistants can connect?', 'Any assistant that supports remote connectors with an access key, including ChatGPT, Claude, Gemini, Grok, Claude Code and Cursor. Give it your workspace link and a key.'],
    ['What can a script do, and for how long?', 'It can use Archive and standard JavaScript, but it cannot reach the internet, import code, set timers or open files. Each run has a time limit, up to 256 KB of results and 64 KB of logs.'],
    ['How do I give an AI assistant its own access?', 'Make an access key from the command line, the API or Terraform, and choose what it can do in each space and when it expires. It can never do more than you can.'],
    ['Can I keep my structure in git?', 'Yes. Pull it as one file for each kind of record or board, edit, preview and apply. Applying what you just pulled changes nothing, so the files stay stable.'],
    ['How do I page through a lot of records?', 'Lists come back oldest first. Pass the last id you saw as `after` to get the next page.']
  ];
</script>

<svelte:head>
  <title>Developers — Archive</title>
  <meta name="description" content="Build on your Archive data from the API, the command line, Terraform or your AI assistant, with the same rules about who can see what everywhere." />
  <meta name="theme-color" content="#15161a" />
</svelte:head>

<Hero
  band
  title="Build on your data from anywhere."
  lead="Use the API, the command line, Terraform or the AI you already use. Every one of them sees the same information and follows the same rules about who can see what."
  art="api"
  alt="A switchboard connecting a terminal, a browser and an AI assistant to one place"
/>

<Comp>
  <h2 class="c-big s-8 at-1 row-1 reveal" use:reveal>Four ways in. <em>One set of rules.</em></h2>
  <TextBlock class="s-4 at-9 row-1 end">
    <p>The same records, the same checks and the same permissions, wherever you build.</p>
    <div class="row"><Button href="/docs" ghost>Read the docs</Button></div>
  </TextBlock>
  <TextBlock class="s-4 at-1 row-2 center">
    <h2>One API for everything.</h2>
    <p>Read and write your records, files and search with plain web requests and one access key.</p>
    <p>Add a new kind of record and it is ready to use right away, with nothing to rebuild or redeploy.</p>
  </TextBlock>
  <Art class="s-7 at-6 row-2 bleed-r" reveal name="endpoint" alt="A service window handing out neatly packed parcels" />
  <TextBlock class="s-4 at-8 row-3 down-2">
    <h2>Fifteen words do it all.</h2>
    <p>Short commands like list, create, search and edit cover the whole product, and the same script runs from the API, the command line or your AI assistant.</p>
    <p>Each script acts with your permissions and saves all of its changes or none of them.</p>
  </TextBlock>
  <Art class="s-6 at-1 row-3 tilt-l" reveal name="code" alt="A code editor whose lines snap neatly into place" />
  <TextBlock class="s-4 at-2 down-2">
    <h2>Works with the AI you already use.</h2>
    <p>ChatGPT, Claude, Gemini, Grok, Claude Code and Cursor connect with a link and an access key.</p>
    <p>They see how your data is actually organized, so they work with your real records instead of guessing.</p>
  </TextBlock>
  <Art class="s-6 at-7" reveal name="agents" alt="Several AI assistants plugged into one socket" />
  <TextBlock class="s-4 at-9 down-1">
    <h2>Change your structure like code.</h2>
    <p>Keep the shape of your data, your access keys and who can do what in files next to your code, or in Terraform with the rest of your stack.</p>
    <p>Preview each change and what it could affect, then apply it. Renaming a field keeps everything in it, and nothing is deleted unless you say so.</p>
  </TextBlock>
  <Art class="s-8 at-1 up-2 bleed-l" reveal name="schema" alt="A blueprint being stamped into place" />
</Comp>

<Section band title="The API, route by route." body="Every request carries a sign-in or an access key. When something is wrong, you get a clear reason and a note on each field to fix.">
  <Facts head={['Endpoint', 'What it does']} rows={API} />
</Section>

<Section title="Every script command." body="Each one is fully typed, so your editor helps as you write. Errors tell you plainly whether something was invalid, missing, not allowed or in conflict.">
  <Facts head={['Verb', 'What it does']} rows={VERBS} />
</Section>

<Section band body="Every command acts as a real person with their permissions. Its output is easy for scripts, scheduled jobs and CI to read.">
  {#snippet title()}The <em>archive</em> command line.{/snippet}
  <Facts head={['Command', 'What it does']} rows={CLI} />
</Section>

<Comp>
  <TextBlock class="s-5 at-1 row-1">
    <h2>Under the hood.</h2>
    <p>Pages that update live, changes that never end up half done, and servers that rest when nobody is working.</p>
  </TextBlock>
  <TextBlock class="s-6 at-1 row-2" art="crdt" title="Live pages" items={["Everyone's edits appear on screen as they happen", 'Edits from people and AI merge on their own, so nothing gets lost', 'See who is on the page and where they are typing']} />
  <TextBlock class="s-4 at-8 row-1 rows-2 down-1" art="transaction" title="All or nothing" items={['If a script fails or runs out of time, nothing is saved', 'An error you handle undoes only that one step', 'Live updates and file processing wait until the work is saved']} />
  <TextBlock class="s-5 at-4 row-3" art="scale" title="Rests when idle" items={['No single server holds anything, so any one can answer', 'Nothing runs between requests', 'An open page keeps things awake only while it is open']} />
</Comp>

<Section band title="Developer questions." body="The rest is in the docs.">
  <Faq items={FAQ} />
</Section>

<Cta title="Build on your own data." lead="Free for your first workspace. Make an access key and start building." />
