<script>
  import { onMount } from 'svelte';
  import Code from '#lib/Code.svelte';
  import Cta from '#lib/Cta.svelte';
  import Hero from '#lib/Hero.svelte';
  import Meta from '#lib/Meta.svelte';
  import Note from '#lib/Note.svelte';
  import Section from '#lib/Section.svelte';
  import Specs from '#lib/Specs.svelte';
  import { START } from '#lib/site.js';

  const GUIDES = [
    { href: '#quickstart', title: 'Quick start', art: 'tokens', text: 'Sign up, make a token, make a call.' },
    { href: '#connect', title: 'Connect your AI', art: 'agents', text: 'Claude, Cursor and ChatGPT over MCP.' },
    { href: '#code', title: 'Code mode', art: 'code', text: 'One tool, one script, one transaction.' },
    { href: '#api', title: 'HTTP API', art: 'api', text: 'Plain JSON for every table you make.' }
  ];

  const NAV = [
    [
      'Get started',
      [
        ['intro', 'Introduction'],
        ['quickstart', 'Quick start'],
        ['connect', 'Connect your AI'],
        ['tokens', 'Tokens']
      ]
    ],
    [
      'Data',
      [
        ['types', 'Your own types'],
        ['schema', 'Changing the schema'],
        ['views', 'Views'],
        ['search', 'Search']
      ]
    ],
    [
      'Documents',
      [
        ['files', 'Files'],
        ['office', 'Office documents']
      ]
    ],
    [
      'Build',
      [
        ['code', 'Code mode'],
        ['sdk', 'The SDK'],
        ['api', 'The HTTP API'],
        ['cli', 'The CLI']
      ]
    ],
    [
      'Access',
      [
        ['spaces', 'Spaces'],
        ['people', 'People and roles'],
        ['secrets', 'Secret fields']
      ]
    ],
    [
      'More',
      [
        ['connectors', 'Connectors'],
        ['models', 'Models'],
        ['help', 'Getting help']
      ]
    ]
  ];

  let current = $state('');
  let toc = $state([]);

  onMount(() => {
    const sections = new Map();
    for (const [, links] of NAV) {
      for (const [id] of links) {
        const head = document.getElementById(id);
        if (!head) continue;
        const subs = [];
        for (let n = head.nextElementSibling; n && n.tagName !== 'H2'; n = n.nextElementSibling) {
          if (n.tagName === 'H3' && n.id) subs.push({ id: n.id, text: n.textContent });
        }
        sections.set(head, subs);
      }
    }
    const observer = new IntersectionObserver(
      (entries) =>
        entries.forEach((e) => {
          if (!e.isIntersecting) return;
          current = e.target.id;
          toc = sections.get(e.target);
        }),
      { rootMargin: '-80px 0px -70% 0px' }
    );
    sections.forEach((_, head) => observer.observe(head));
    return () => observer.disconnect();
  });
</script>

<Meta title="Docs — Archive" description="How Archive works: your own Postgres types, live schema changes, code mode over MCP, the SDK, the HTTP API, files, Office documents, spaces and roles." />

<Hero
  title="Everything, documented."
  lead="Types, schema changes, code mode, the SDK and the HTTP API. Copy an example, or hand a prompt to your AI and let it do the work."
  art="docs"
  alt="An open binder of documentation pages with code snippets and diagrams"
  links={[['#quickstart', 'Quick start'], START]}
/>

<Section>
  <Specs items={GUIDES} />
</Section>

<div class="wrap docs">
  <nav class="docs-nav" aria-label="Docs">
    {#each NAV as [group, links]}
      <h4>{group}</h4>
      {#each links as [id, label]}<a href="#{id}" class={{ on: current === id }}>{label}</a>{/each}
    {/each}
  </nav>

  <article class="prose">

<h2 id="intro">Introduction</h2>
<p>Archive is a workspace for tables and files with a real Postgres 18 database underneath. Your team works in the web app, your code calls the HTTP API, and every AI app connects over MCP.</p>

<h3 id="intro-ways">Three ways in</h3>
<ul>
<li><strong>The web app</strong> for you and your team: tables, views and files.</li>
<li><strong>The HTTP API</strong> at <code>/api</code> for your code, with a token.</li>
<li><strong>MCP</strong> at <code>/mcp</code> for your AI, with one tool: <code>run</code>.</li>
</ul>
<p>All three pass the same permissions before they touch a row or a file.</p>

<h3 id="intro-postgres">Postgres underneath</h3>
<p>Each type you define is a real Postgres table, and a row is one record. People, spaces and tokens are Postgres roles, and row-level security filters every query, so plain SQL sees exactly what the app sees.</p>

<h3 id="intro-agents">Your AI does the thinking</h3>
<p>Archive calls no AI model on its own. Your assistant reads what is there, decides what to do, and sends Archive a short TypeScript script to run.</p>

<h2 id="quickstart">Quick start</h2>
<p>From sign-up to your first answer in a few minutes. No card, no sales call.</p>
<ol>
<li>Sign up at <a href="https://mulholland.ai">mulholland.ai</a> and create a workspace. It lives at <code>https://your-workspace.archive.mulholland.ai</code>.</li>
<li>Sign in with the link we email you. You get a private space of your own.</li>
<li>Create a token (see <a href="#tokens">Tokens</a>) and keep it as <code>ARCHIVE_TOKEN</code>.</li>
<li>Connect your AI (see <a href="#connect">Connect your AI</a>), or call the API straight away.</li>
</ol>

<h3 id="qs-first-call">Your first call</h3>
<p>Every call carries your token. This one runs a one-line script that lists your tables.</p>
<Code tabs={['curl', 'response']}>
<pre><span class="k">export</span> ARCHIVE_TOKEN=<span class="s">&lt;your token&gt;</span>

curl https://your-workspace.archive.mulholland.ai/api/run \
  -H <span class="s">"Authorization: Bearer $ARCHIVE_TOKEN"</span> \
  -d <span class="s">'&#123;"code": "return await archive.tables()"&#125;'</span></pre>
<pre hidden>&#123;"result": [&#123;"name": "invoice", "kind": "table", "description": "An invoice you sent", "icon": "receipt", "columns": [...]&#125;], "logs": [], "error": ""&#125;</pre>
</Code>

<h3 id="qs-first-type">Your first type</h3>
<p>You rarely write DDL by hand. Ask your AI, and it plans the change before it applies it.</p>
<Code tabs={['prompt']}>
<pre>In my Archive, add an invoice table with a customer, an amount of money, a status
(draft, unpaid or paid) and a due date. Use the run tool: read the files with
archive.schema(), add invoice.sql, and show me the plan from archive.apply(files).
Only apply it with &#123; confirm: true &#125; after I say yes.</pre>
</Code>

<h2 id="connect">Connect your AI</h2>
<p>Archive is a remote MCP server at <code>/mcp</code>, over streamable HTTP, with one tool: <code>run</code>. Add it once with your token and your AI can read and change everything the token can.</p>
<Note>The <code>run</code> tool's description is the full SDK reference plus a type for each of your tables, so your AI writes against your real schema from the first call.</Note>

<h3 id="claude-code">Claude Code</h3>
<Code tabs={['terminal']}>
<pre>claude mcp add --transport http archive https://your-workspace.archive.mulholland.ai/mcp \
  --header <span class="s">"Authorization: Bearer $ARCHIVE_TOKEN"</span></pre>
</Code>

<h3 id="cursor">Cursor</h3>
<p>Add this to <code>.cursor/mcp.json</code> in your project, or to <code>~/.cursor/mcp.json</code> for every project.</p>
<Code tabs={['.cursor/mcp.json']}>
<pre>&#123;
  <span class="s">"mcpServers"</span>: &#123;
    <span class="s">"archive"</span>: &#123;
      <span class="s">"url"</span>: <span class="s">"https://your-workspace.archive.mulholland.ai/mcp"</span>,
      <span class="s">"headers"</span>: &#123; <span class="s">"Authorization"</span>: <span class="s">"Bearer &lt;your token&gt;"</span> &#125;
    &#125;
  &#125;
&#125;</pre>
</Code>

<h3 id="claude-desktop">Claude Desktop</h3>
<p>Claude Desktop's config file starts local programs, so bridge to the remote server with <code>mcp-remote</code>, which sends your token as a header. Add this to <code>claude_desktop_config.json</code> and restart Claude.</p>
<Code tabs={['claude_desktop_config.json']}>
<pre>&#123;
  <span class="s">"mcpServers"</span>: &#123;
    <span class="s">"archive"</span>: &#123;
      <span class="s">"command"</span>: <span class="s">"npx"</span>,
      <span class="s">"args"</span>: [
        <span class="s">"-y"</span>, <span class="s">"mcp-remote"</span>,
        <span class="s">"https://your-workspace.archive.mulholland.ai/mcp"</span>,
        <span class="s">"--header"</span>, <span class="s">"Authorization:$&#123;AUTH_HEADER&#125;"</span>
      ],
      <span class="s">"env"</span>: &#123; <span class="s">"AUTH_HEADER"</span>: <span class="s">"Bearer &lt;your token&gt;"</span> &#125;
    &#125;
  &#125;
&#125;</pre>
</Code>

<h3 id="chatgpt">ChatGPT and claude.ai</h3>
<p>Both reach your workspace over its public HTTPS address. Add <code>https://your-workspace.archive.mulholland.ai/mcp</code> as a custom connector.</p>
<Note mark="!">Archive's MCP server signs requests in with a bearer token and has no OAuth sign-in yet. Use these apps where their connector settings let you send an <code>Authorization</code> header.</Note>

<h3 id="connect-check">Check the connection</h3>
<Code tabs={['prompt']}>
<pre>Call Archive's run tool with: return await archive.tables()
Then tell me which tables and views I have, and what each one is for.</pre>
</Code>

<h2 id="tokens">Tokens</h2>
<p>A token acts as you, and it can only narrow what you can do. Agents, scripts and CI each get their own.</p>
<Code tabs={['header']}>
<pre>Authorization: Bearer $ARCHIVE_TOKEN</pre>
</Code>

<h3 id="token-create">Create a token</h3>
<p>Create your first token in your workspace once you are signed in. After that, make more with the API. The token value comes back once, in the response to create.</p>
<Code tabs={['curl', 'response']}>
<pre>curl https://your-workspace.archive.mulholland.ai/auth/tokens \
  -H <span class="s">"Authorization: Bearer $ARCHIVE_TOKEN"</span> \
  -d <span class="s">'&#123;"name": "invoice-agent", "spaces": &#123;"finance": "editor"&#125;, "expires": "720h"&#125;'</span></pre>
<pre hidden>201  &#123;"id": "0193…", "name": "invoice-agent", "spaces": &#123;"finance": "editor"&#125;, "expires_at": "2026-11-27T09:30:00Z", "created_at": "2026-10-28T09:30:00Z", "token": "…"&#125;</pre>
</Code>

<h3 id="token-scope">Scope</h3>
<table>
<thead><tr><th>Field</th><th>What it does</th></tr></thead>
<tbody>
<tr><td><code>name</code></td><td>A label, so you know which token is which.</td></tr>
<tr><td><code>spaces</code></td><td>Spaces and a role in each, like <code>&#123;"finance": "viewer"&#125;</code>. Never higher than your own role there. Left out, the token gets your current role in every space you are in.</td></tr>
<tr><td><code>expires</code></td><td>A lifetime such as <code>"720h"</code>. Left out, the token does not expire.</td></tr>
<tr><td><code>full</code></td><td>All your rights, including managing spaces, members and tokens, as Terraform needs. A full token takes no <code>spaces</code>.</td></tr>
</tbody>
</table>

<h3 id="token-manage">List and revoke</h3>
<p><code>GET /auth/tokens</code> lists your tokens without their values. <code>DELETE /auth/tokens/&#123;id&#125;</code> revokes one, and it stops working at once. When someone's role in a space goes down, their tokens for that space go down with it.</p>

<h2 id="types">Your own types</h2>
<p>Each kind of record is a real Postgres table in the <code>public</code> schema. The table is the type, a row is a record, and nothing else is stored about it.</p>

<h3 id="types-needs">What every table needs</h3>
<p>The columns <code>id</code>, <code>space</code>, <code>created_at</code>, <code>updated_at</code> and a <code>search</code> tsvector, a search index, and row-level security with policies. A plan without them is rejected. A complete file, <code>invoice.sql</code>:</p>
<Code tabs={['invoice.sql']}>
<pre><span class="k">CREATE TYPE</span> invoice_status <span class="k">AS ENUM</span> (<span class="s">'draft'</span>, <span class="s">'unpaid'</span>, <span class="s">'paid'</span>);

<span class="k">CREATE TABLE</span> invoice (
    id         uuid <span class="k">PRIMARY KEY DEFAULT</span> <span class="f">uuidv7</span>(),
    space      uuid <span class="k">NOT NULL REFERENCES</span> archive.space,
    created_at timestamptz <span class="k">NOT NULL DEFAULT</span> <span class="f">now</span>(),
    updated_at timestamptz <span class="k">NOT NULL DEFAULT</span> <span class="f">now</span>(),
    customer   text <span class="k">NOT NULL</span>,
    amount     archive.money,
    status     invoice_status <span class="k">NOT NULL DEFAULT</span> <span class="s">'unpaid'</span>,
    due        date,
    search     tsvector <span class="k">GENERATED ALWAYS AS</span> (archive.<span class="f">search_doc</span>(customer)) <span class="k">STORED</span>
);
<span class="k">CREATE INDEX</span> invoice_search <span class="k">ON</span> invoice <span class="k">USING</span> gin (search);
<span class="k">ALTER TABLE</span> invoice <span class="k">ENABLE ROW LEVEL SECURITY</span>;
<span class="k">ALTER TABLE</span> invoice <span class="k">FORCE ROW LEVEL SECURITY</span>;
<span class="k">CREATE POLICY</span> viewer <span class="k">ON</span> invoice <span class="k">FOR SELECT USING</span> (space = <span class="k">ANY</span> (archive.<span class="f">my_spaces</span>()));
<span class="k">CREATE POLICY</span> editor <span class="k">ON</span> invoice <span class="k">USING</span> (space = <span class="k">ANY</span> (archive.<span class="f">my_spaces</span>(<span class="s">'editor'</span>)))
    <span class="k">WITH CHECK</span> (space = <span class="k">ANY</span> (archive.<span class="f">my_spaces</span>(<span class="s">'editor'</span>)));
<span class="k">COMMENT ON TABLE</span> invoice <span class="k">IS</span> <span class="s">'An invoice you sent
icon: receipt'</span>;
<span class="k">COMMENT ON COLUMN</span> invoice.customer <span class="k">IS</span> <span class="s">'Who the invoice is for'</span>;</pre>
</Code>
<p>Archive sets <code>id</code>, <code>space</code>, <code>created_at</code> and <code>updated_at</code>. Columns marked <code>NOT NULL</code> without a default are required on create.</p>

<h3 id="types-comments">Settings in comments</h3>
<p>The first lines of a table or view comment are its description. The last lines may be <code>key: value</code> settings. Any other key is an error, and so is an unknown icon, layout or column.</p>
<table>
<thead><tr><th>Key</th><th>On</th><th>Value</th></tr></thead>
<tbody>
<tr><td><code>icon</code></td><td>Tables and views</td><td>A Lucide icon name in kebab-case, like <code>receipt</code> or <code>square-check</code>.</td></tr>
<tr><td><code>layout</code></td><td>Views</td><td><code>table</code>, <code>board</code> or <code>calendar</code>.</td></tr>
<tr><td><code>by</code></td><td>Views</td><td>The column a board groups by, or the date a calendar places rows on.</td></tr>
</tbody>
</table>
<p>Column comments become the field descriptions your AI reads in the <code>run</code> tool.</p>

<h3 id="types-builtin">Built-in types</h3>
<p>Ready-made column types in the <code>archive</code> schema. Writes clean messy input first, then Postgres checks the format, so the same thing is stored the same way whatever it came from.</p>
<table>
<thead><tr><th>Area</th><th>Types</th></tr></thead>
<tbody>
<tr><td>People and places</td><td><code>name</code>, <code>email</code>, <code>phone</code>, <code>address</code>, <code>postcode</code>, <code>country</code>, <code>region</code>, <code>geo</code>, <code>locale</code></td></tr>
<tr><td>Time</td><td><code>timestamp</code>, <code>time</code>, <code>interval</code>, <code>duration</code>, <code>timezone</code>, <code>recurrence</code></td></tr>
<tr><td>Money and business</td><td><code>money</code>, <code>currency</code>, <code>percentage</code>, <code>iban</code>, <code>bic</code>, <code>us_routing</code>, <code>us_ein</code>, <code>vat_id</code>, <code>lei</code>, <code>isin</code></td></tr>
<tr><td>Products and the web</td><td><code>quantity</code>, <code>gtin</code>, <code>isbn</code>, <code>url</code>, <code>domain</code>, <code>ip</code>, <code>mime</code>, <code>richtext</code></td></tr>
<tr><td>Secret</td><td><code>card</code>, <code>us_ssn</code>, <code>nl_bsn</code>, <code>national</code>, <code>passport</code> (see <a href="#secrets">Secret fields</a>)</td></tr>
</tbody>
</table>
<p>So <code>"06 12345678"</code> is stored as <code>"+31612345678"</code>, <code>"€ 1.234,50"</code> as <code>&#123; amount: 1234.5, currency: "EUR" &#125;</code>, and <code>"3-4-1990"</code> as <code>"1990-04-03"</code>.</p>

<h3 id="types-refs">References and lists</h3>
<ul>
<li>A reference is a foreign key: <code>customer uuid REFERENCES company</code>.</li>
<li>A list of references is <code>uuid[]</code> with a column comment starting <code>references company</code>.</li>
<li>Other lists are plain arrays, like <code>text[]</code>. There is no type inheritance.</li>
<li>Files load in any order. For two tables that reference each other, add at least one of those references with <code>ALTER TABLE … ADD FOREIGN KEY</code> after its <code>CREATE TABLE</code>.</li>
</ul>

<h2 id="schema">Changing the schema</h2>
<p>The live database is the only source of truth. Read it as files, change a file, see the plan, then apply it. No migrations to write.</p>

<h3 id="schema-read">Read</h3>
<p><code>archive.schema()</code> generates the DDL from the database: one <code>&lt;name&gt;.sql</code> per table or view, with its indexes, foreign keys, row-level security, policies, triggers and comments. The text is normalized, so your layout and <code>--</code> comments are not kept; <code>COMMENT ON</code> is.</p>

<h3 id="schema-plan">Plan</h3>
<p>Pass the whole set of files to <code>archive.apply</code> without <code>confirm</code> to see the statements it would run, each with its hazards.</p>
<Code tabs={['run.ts', 'plan']}>
<pre><span class="k">const</span> files = <span class="k">await</span> archive.<span class="f">schema</span>()
files[<span class="s">"invoice.sql"</span>] += <span class="s">"\nALTER TABLE invoice ADD COLUMN paid_on date;"</span>

<span class="k">return</span> archive.<span class="f">apply</span>(files)</pre>
<pre hidden>&#123;
  "statements": [
    &#123; "ddl": "ALTER TABLE \"public\".\"invoice\" ADD COLUMN \"paid_on\" date" &#125;
  ],
  "applied": false
&#125;</pre>
</Code>
<Note mark="!">Plan and apply take the whole schema. A table in no file is dropped, so always start from <code>archive.schema()</code>.</Note>

<h3 id="schema-apply">Apply</h3>
<p>Call it again with <code>&#123; confirm: true &#125;</code>. Indexes are built concurrently, and foreign keys are added first and validated after, so tables stay usable while it runs.</p>
<ul>
<li>Apply commits on its own as soon as it succeeds, even if your script throws later.</li>
<li>It waits for locks on what it changes, including locks your own run took by writing those tables. Apply first, or in a run of its own.</li>
<li>New tables and views work in the same run, right after it.</li>
</ul>

<h3 id="schema-renames">Renames</h3>
<p>The diff cannot tell a rename from a drop and an add, which would lose data. Pass renames explicitly, along with the files as they read after the rename. They run first, in the same transaction.</p>
<Code tabs={['run.ts']}>
<pre><span class="k">const</span> files = <span class="k">await</span> archive.<span class="f">schema</span>()
<span class="k">for</span> (<span class="k">const</span> name <span class="k">in</span> files) files[name] = files[name].<span class="f">replace</span>(<span class="s">/\bcustomer\b/g</span>, <span class="s">"client"</span>)

<span class="k">return</span> archive.<span class="f">apply</span>(files, &#123;
  confirm: <span class="k">true</span>,
  renames: [<span class="s">"ALTER TABLE invoice RENAME COLUMN customer TO client"</span>],
&#125;)</pre>
</Code>

<h3 id="schema-drops">Drops</h3>
<p>A statement that deletes data, such as dropping a table or a column, is blocked unless you pass <code>&#123; allowDrop: true &#125;</code>. Dropping a view is never blocked, since a view holds no data.</p>

<h3 id="schema-who">Who can change it</h3>
<p>Workspace admins, and admins of any space. Everyone else can read the schema but not apply it.</p>

<h3 id="schema-git">Keep it in git</h3>
<p><code>apply(await archive.schema())</code> always plans nothing, so you can pull the files, keep them in a repository and apply them back. Over HTTP, use <code>GET /api/schema</code> and <code>POST /api/schema/plan</code> or <code>/apply</code> with <code>&#123;"files": &#123;...&#125;, "renames": [...], "allow_drop": false&#125;</code>.</p>

<h2 id="views">Views</h2>
<p>A view is a saved way to look at a table: a real Postgres view in its own file, with its layout in its comment. Boards, calendars and filtered tables are all views.</p>
<Code tabs={['invoice_board.sql', 'invoice_due.sql']}>
<pre><span class="k">CREATE VIEW</span> invoice_board <span class="k">WITH</span> (security_invoker = <span class="k">true</span>) <span class="k">AS</span>
    <span class="k">SELECT</span> * <span class="k">FROM</span> invoice <span class="k">WHERE</span> status &lt;&gt; <span class="s">'draft'</span>;
<span class="k">COMMENT ON VIEW</span> invoice_board <span class="k">IS</span> <span class="s">'Invoices by status
layout: board
by: status
icon: kanban'</span>;</pre>
<pre hidden><span class="k">CREATE VIEW</span> invoice_due <span class="k">WITH</span> (security_invoker = <span class="k">true</span>) <span class="k">AS</span>
    <span class="k">SELECT</span> * <span class="k">FROM</span> invoice <span class="k">WHERE</span> status = <span class="s">'unpaid'</span>;
<span class="k">COMMENT ON VIEW</span> invoice_due <span class="k">IS</span> <span class="s">'Unpaid invoices by due date
layout: calendar
by: due
icon: calendar'</span>;</pre>
</Code>

<h3 id="views-rules">What every view needs</h3>
<ul>
<li><code>WITH (security_invoker = true)</code>, so row-level security applies to whoever reads it.</li>
<li>An <code>id</code> column, which <code>SELECT *</code> from a table already gives you.</li>
<li>A comment, with <code>layout</code>, <code>by</code> and <code>icon</code> as its last lines.</li>
</ul>

<h3 id="views-read">Reading a view</h3>
<p><code>list</code>, <code>get</code> and <code>search</code> read views like tables. Views are read only: create, update and delete go to their tables, and writes to a view over HTTP return 405. <code>archive.tables()</code> shows each view with <code>kind: "view"</code>, its layout and its <code>by</code> column.</p>

<h3 id="views-change">Add, change or remove one</h3>
<p>Views are schema, so you change them like tables: set <code>files["invoice_board.sql"]</code> and apply, or delete that key and apply to remove it.</p>

<h2 id="search">Search</h2>
<p>One search covers every table and file you can see, best match first.</p>
<Code tabs={['run.ts', 'curl']}>
<pre><span class="k">const</span> hits = <span class="k">await</span> archive.<span class="f">search</span>(<span class="s">"northwind"</span>, &#123; type: <span class="s">"invoice"</span>, from: <span class="s">"2026-01-01"</span> &#125;)</pre>
<pre hidden>curl <span class="s">"https://your-workspace.archive.mulholland.ai/api/search?q=northwind&amp;type=invoice"</span> \
  -H <span class="s">"Authorization: Bearer $ARCHIVE_TOKEN"</span></pre>
</Code>

<h3 id="search-matching">How it matches</h3>
<p>Words match by prefix and ignore case and accents: "nort" finds "Northwind", and "cafe" finds "Café".</p>

<h3 id="search-filters">Filters and results</h3>
<p>Filter by <code>type</code> (a table name, or <code>"file"</code>), <code>space</code>, and <code>from</code> and <code>to</code> dates. You get 20 hits by default and up to 100 with <code>limit</code>.</p>
<p>Each hit has <code>kind</code> (<code>"entity"</code> or <code>"file"</code>), <code>type</code>, <code>id</code>, <code>space</code>, <code>date</code>, <code>data</code> and <code>score</code>. File hits add a <code>snippet</code> with the matches in <strong>bold</strong>.</p>

<h3 id="search-column">What gets searched</h3>
<p>A row is found by the words in its <code>search</code> column, so put the columns people look for there: <code>archive.search_doc(customer, notes)</code>. A file is found by its name and extracted text.</p>

<h2 id="files">Files</h2>
<p>Upload any file. Archive pulls out its metadata and text before the upload returns, so search finds it right away.</p>
<Code tabs={['curl']}>
<pre>curl <span class="s">"https://your-workspace.archive.mulholland.ai/api/files?space=finance"</span> \
  -H <span class="s">"Authorization: Bearer $ARCHIVE_TOKEN"</span> \
  -F file=@contract.pdf</pre>
</Code>

<h3 id="files-processing">What Archive extracts</h3>
<p>Text from PDF, HTML, plain text, JSON and email. Metadata such as page counts, image dimensions and mail headers. Thumbnails for images. A file's <code>status</code> is <code>done</code> when its upload returns.</p>
<Note>Office files are stored and edited in place but not text-extracted yet, so search finds them by name only.</Note>

<h3 id="files-large">Large files</h3>
<p>A plain upload takes up to 64 MB. For bigger files, use resumable tus uploads at <code>/api/files/tus</code>, which pick up where they stopped.</p>

<h3 id="files-links">Download links</h3>
<p><code>POST /api/files/&#123;id&#125;/link</code> returns a signed URL that works without signing in, for 24 hours or for <code>?expires=1h</code>. Signed links always drop a photo's GPS location; on a signed-in download, add <code>?strip=gps</code>.</p>

<h3 id="files-endpoints">Endpoints</h3>
<table>
<thead><tr><th>Endpoint</th><th>Returns</th></tr></thead>
<tbody>
<tr><td><code>GET /api/files</code></td><td>Your files, oldest first; page with <code>limit</code> and <code>after</code>.</td></tr>
<tr><td><code>GET /api/files/&#123;id&#125;</code></td><td>One file's metadata.</td></tr>
<tr><td><code>GET /api/files/&#123;id&#125;/content</code></td><td>Its bytes.</td></tr>
<tr><td><code>GET /api/files/&#123;id&#125;/thumbnail</code></td><td>Its thumbnail, for images.</td></tr>
<tr><td><code>GET /api/files/&#123;id&#125;/text</code></td><td>Its extracted text.</td></tr>
</tbody>
</table>

<h2 id="office">Office documents</h2>
<p>Your AI reads and edits Word, Excel and PowerPoint files in place. An edit changes only what it targets, and every other part of the file stays byte for byte, so formatting, images, comments and charts survive.</p>
<Code tabs={['xlsx', 'docx', 'pptx']}>
<pre><span class="k">const</span> budget = <span class="k">await</span> archive.<span class="f">create</span>(<span class="s">"Budget.xlsx"</span>)

<span class="k">await</span> archive.<span class="f">edit</span>(budget.id, [
  &#123; op: <span class="s">"set_cell"</span>, cell: <span class="s">"A1"</span>, value: <span class="s">"Rent"</span> &#125;,
  &#123; op: <span class="s">"set_cell"</span>, cell: <span class="s">"B1"</span>, value: <span class="n">1200</span> &#125;,
  &#123; op: <span class="s">"set_cell"</span>, cell: <span class="s">"A2"</span>, value: <span class="s">"Internet"</span> &#125;,
  &#123; op: <span class="s">"set_cell"</span>, cell: <span class="s">"B2"</span>, value: <span class="n">60</span> &#125;,
  &#123; op: <span class="s">"set_formula"</span>, cell: <span class="s">"B3"</span>, formula: <span class="s">"SUM(B1:B2)"</span> &#125;,
])</pre>
<pre hidden><span class="k">const</span> [hit] = <span class="k">await</span> archive.<span class="f">search</span>(<span class="s">"Northwind agreement"</span>, &#123; type: <span class="s">"file"</span> &#125;)
<span class="k">const</span> doc = <span class="k">await</span> archive.<span class="f">read</span>(hit.id)
<span class="k">if</span> (doc.type !== <span class="s">"docx"</span>) <span class="k">throw new</span> <span class="f">Error</span>(<span class="s">"not a Word file"</span>)

<span class="k">await</span> archive.<span class="f">edit</span>(hit.id, [
  &#123; op: <span class="s">"replace_text"</span>, find: <span class="s">"30 days"</span>, replace: <span class="s">"45 days"</span> &#125;,
  &#123; op: <span class="s">"insert_paragraph_after"</span>, para: doc.paragraphs[<span class="n">0</span>].id, text: <span class="s">"Amended on 28 October 2026"</span> &#125;,
])</pre>
<pre hidden><span class="k">const</span> deck = <span class="k">await</span> archive.<span class="f">create</span>(<span class="s">"Q4 review.pptx"</span>)

<span class="k">await</span> archive.<span class="f">edit</span>(deck.id, [
  &#123; op: <span class="s">"replace_shape_text"</span>, slide: <span class="n">1</span>, shape: <span class="n">0</span>, text: <span class="s">"Q4 review"</span> &#125;,
  &#123; op: <span class="s">"replace_shape_text"</span>, slide: <span class="n">1</span>, shape: <span class="n">1</span>, text: <span class="s">"Revenue up 12%\nCosts flat"</span> &#125;,
])</pre>
</Code>

<h3 id="office-read">Read the model</h3>
<p><code>archive.read(id)</code> returns the document as a model: paragraphs with ids, styles, lists and tables for docx; sheets, cells, formulas, merged ranges and Excel tables for xlsx; slides and their text shapes for pptx. Read again after a successful edit before planning more.</p>

<h3 id="office-new">New documents</h3>
<p><code>archive.create("Name.docx")</code> makes an empty file: a docx with one empty paragraph, an xlsx with an empty Sheet1, or a pptx with one title slide where shape 0 is the title and shape 1 the subtitle.</p>

<h3 id="office-edits">Edits by format</h3>
<table>
<thead><tr><th>Format</th><th>Edits</th></tr></thead>
<tbody>
<tr><td>docx</td><td><code>replace_text</code>, <code>insert_paragraph_after</code>, <code>delete_paragraph</code>, <code>set_cell_text</code></td></tr>
<tr><td>xlsx</td><td><code>set_cell</code>, <code>set_formula</code>, <code>clear_cell</code>, <code>append_table_row</code></td></tr>
<tr><td>pptx</td><td><code>replace_text</code>, <code>replace_shape_text</code></td></tr>
</tbody>
</table>
<p>Formulas recalculate when the file is opened. <code>append_table_row</code> grows an Excel table, so totals and charts that use it pick up the new row.</p>

<h3 id="office-together">Edits are all or nothing</h3>
<p>Edits apply in order. If one fails, nothing is saved, and the error names the edit and how to fix it:</p>
<Code tabs={['error']}>
<pre>edit 0 (replace_text): text "Boot Camp" occurs in 2 paragraphs (p14, p15); pass "para" or set "all": true</pre>
</Code>
<p>Agents editing the same file take turns, so no edit is lost. Paragraph ids are positions in the version you read, and they keep pointing at those paragraphs for every edit in one call.</p>

<h3 id="office-limits">Not possible yet</h3>
<p>Inserting or deleting rows, columns, slides, shapes or tables; changing formatting such as bold, colors and number formats; comments, charts, headers and footers.</p>

<h2 id="code">Code mode</h2>
<p>Your AI writes one short TypeScript script. Archive runs it inside your workspace, as you, in one transaction, and sends back one result.</p>
<Code tabs={['run.ts', 'result']}>
<pre><span class="k">const</span> unpaid = <span class="k">await</span> archive.<span class="f">list</span>(<span class="s">"invoice"</span>, &#123; where: &#123; status: <span class="s">"unpaid"</span> &#125; &#125;)
<span class="k">const</span> total = unpaid.<span class="f">reduce</span>((sum, i) =&gt; sum + (i.amount?.amount ?? <span class="n">0</span>), <span class="n">0</span>)

console.<span class="f">log</span>(<span class="s">`$&#123;unpaid.length&#125; unpaid`</span>)
<span class="k">return</span> &#123; count: unpaid.length, total &#125;</pre>
<pre hidden>&#123;"result": &#123;"count": 3, "total": 6530&#125;, "logs": ["3 unpaid"], "error": ""&#125;</pre>
</Code>

<h3 id="code-why">Why one script</h3>
<p>One trip instead of dozens. A script can search, loop, transform and write in a single turn, which is faster and cheaper than many small tool calls, and it never leaves a change half done.</p>

<h3 id="code-rules">The rules</h3>
<ul>
<li><strong>All or nothing.</strong> If the script throws, times out or returns too much, everything rolls back.</li>
<li><strong>As you.</strong> Row-level security applies to everything, raw SQL included.</li>
<li><strong>Sandboxed.</strong> Only <code>archive</code>, <code>console</code> and standard JavaScript exist: no fetch, imports, timers, files or process.</li>
<li><strong>Limits.</strong> 30 seconds per run, 256 KB of result and 64 KB of logs. Return what you need, not whole tables.</li>
</ul>

<h3 id="code-runtime">How it runs</h3>
<p>Your code is the body of an async function: use top-level <code>await</code> and <code>return</code> a JSON value. Archive strips the TypeScript types and runs the JavaScript inside the server. Effects that leave the database, such as file processing, happen only after the transaction commits.</p>

<h3 id="code-errors">Errors</h3>
<p>SDK errors are <code>Error</code> objects with a <code>code</code>: <code>invalid</code>, <code>not_found</code>, <code>forbidden</code>, <code>conflict</code>, or a Postgres SQLSTATE. Invalid input adds <code>errors: [&#123; path, message &#125;]</code>, one per bad field. A failed call you catch undoes only that call.</p>
<Code tabs={['run.ts']}>
<pre><span class="k">try</span> &#123;
  <span class="k">await</span> archive.<span class="f">create</span>(<span class="s">"invoice"</span>, &#123; customer: <span class="s">"Hart"</span>, amount: <span class="s">"lots"</span> &#125;)
&#125; <span class="k">catch</span> (e: <span class="f">any</span>) &#123;
  <span class="k">return</span> &#123; code: e.code, errors: e.errors &#125;
&#125;</pre>
</Code>

<h3 id="code-docs">The tool's docs</h3>
<p>The <code>run</code> tool's description is the SDK reference plus an interface for each of your tables, generated from the live schema. Read the same text at <code>GET /api/run/docs</code>.</p>

<h3 id="code-elsewhere">Run it from anywhere</h3>
<p>The same runtime answers <code>POST /api/run</code> with <code>&#123;"code": "..."&#125;</code> and <code>archive run script.ts</code> on the command line. The output is always <code>&#123;"result", "logs", "error"&#125;</code>.</p>

<h2 id="sdk">The SDK</h2>
<p>Scripts call one object, <code>archive</code>, with one word per action. Table names are typed from your schema, so <code>Tables[T]</code> below is the row type of table <code>T</code>.</p>

<h3 id="sdk-rows">Rows</h3>
<table>
<thead><tr><th>Call</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>sql(query, ...params): Row[]</code></td><td>Runs one SQL statement as you, with <code>$1</code>, <code>$2</code> bound to params.</td></tr>
<tr><td><code>list(table, &#123; where?, limit?, after?, space?, reveal? &#125;): Tables[T][]</code></td><td>Rows oldest first. <code>where</code> matches exact, cleaned values. Page with <code>after</code> set to the last id.</td></tr>
<tr><td><code>get(table, id, &#123; reveal? &#125;): Tables[T]</code></td><td>One row. Throws <code>not_found</code> if it does not exist or you cannot see it.</td></tr>
<tr><td><code>create(table, data, &#123; space? &#125;): Tables[T]</code></td><td>Inserts a cleaned, validated row into <code>space</code> or your own space.</td></tr>
<tr><td><code>update(table, id, patch): Tables[T]</code></td><td>Changes only the fields given; <code>null</code> clears one.</td></tr>
<tr><td><code>delete(table, id): void</code></td><td>Deletes a row.</td></tr>
<tr><td><code>search(text, &#123; type?, space?, from?, to?, limit? &#125;): Hit[]</code></td><td>Full-text search over every table and file you can see.</td></tr>
</tbody>
</table>

<h3 id="sdk-schema">Schema</h3>
<table>
<thead><tr><th>Call</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>schema(): Record&lt;string, string&gt;</code></td><td>DDL from the database, one <code>&lt;name&gt;.sql</code> per table or view.</td></tr>
<tr><td><code>apply(files, &#123; confirm?, allowDrop?, renames? &#125;): Plan</code></td><td>Plans the whole set; with <code>confirm</code>, applies it.</td></tr>
<tr><td><code>tables(): Table[]</code></td><td>Every table and view with its columns, types, comments and references.</td></tr>
</tbody>
</table>

<h3 id="sdk-files">Files and documents</h3>
<table>
<thead><tr><th>Call</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>files(&#123; limit?, after?, space? &#125;): StoredFile[]</code></td><td>Files you can see, oldest first, with their text.</td></tr>
<tr><td><code>file(id): StoredFile</code></td><td>One file with its text.</td></tr>
<tr><td><code>create(name, &#123; space? &#125;): StoredFile</code></td><td>An empty <code>.docx</code>, <code>.xlsx</code> or <code>.pptx</code>, by extension.</td></tr>
<tr><td><code>read(fileId): Model</code></td><td>The structure of an Office file.</td></tr>
<tr><td><code>edit(fileId, edits): StoredFile</code></td><td>Applies edits and saves the file, all or nothing.</td></tr>
</tbody>
</table>

<h3 id="sdk-spaces">Spaces</h3>
<table>
<thead><tr><th>Call</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>spaces(): Space[]</code></td><td>The spaces you can see, each with <code>id</code>, <code>name</code> and your <code>level</code>: owner, admin, editor or viewer.</td></tr>
</tbody>
</table>

<h3 id="sdk-values">Values</h3>
<p>Rows are plain JSON. Dates are <code>"2026-10-27"</code>, instants are UTC like <code>"2026-10-27T09:30:00Z"</code>, references are id strings, arrays are lists and enums are one of their strings. Empty fields read back as <code>null</code>. The built-in composite types read back as objects:</p>
<table>
<thead><tr><th>Type</th><th>Shape</th><th>Also takes</th></tr></thead>
<tbody>
<tr><td><code>Money</code></td><td><code>&#123; amount, currency &#125;</code></td><td><code>"€ 1.234,50"</code> or a number</td></tr>
<tr><td><code>Timestamp</code></td><td><code>&#123; utc, zone &#125;</code></td><td><code>"2026-10-27 09:30"</code></td></tr>
<tr><td><code>Time</code></td><td><code>&#123; time, zone? &#125;</code></td><td><code>"9:30"</code></td></tr>
<tr><td><code>Interval</code></td><td><code>&#123; start, end &#125;</code></td><td>A date or instant at each end; <code>null</code> is open</td></tr>
<tr><td><code>Address</code></td><td><code>&#123; lines, locality?, region?, postcode?, country? &#125;</code></td><td>Country from the locale when missing</td></tr>
<tr><td><code>Name</code></td><td><code>&#123; full, given?, family?, prefixes? &#125;</code></td><td><code>"Jan de Vries"</code></td></tr>
<tr><td><code>Postcode</code></td><td><code>&#123; country, code &#125;</code></td><td><code>"1012 LG"</code></td></tr>
<tr><td><code>Geo</code></td><td><code>&#123; lat, lon &#125;</code></td><td><code>"52.37, 4.89"</code></td></tr>
<tr><td><code>Quantity</code></td><td><code>&#123; value, unit, original_unit &#125;</code></td><td><code>"5 km"</code>, stored in the SI base unit</td></tr>
</tbody>
</table>

<h2 id="api">The HTTP API</h2>
<p>Plain JSON over HTTPS at <code>https://your-workspace.archive.mulholland.ai/api</code>. Every table and view you make gets list, get, create, update and delete.</p>
<Code tabs={['list', 'create', 'update', 'delete']}>
<pre>curl <span class="s">"https://your-workspace.archive.mulholland.ai/api/invoice?status=unpaid&amp;limit=50"</span> \
  -H <span class="s">"Authorization: Bearer $ARCHIVE_TOKEN"</span>

<span class="c"># 200 [&#123;"id": "0193…", "customer": "Northwind", "amount": &#123;"amount": 4200, "currency": "EUR"&#125;, "status": "unpaid", …&#125;]</span></pre>
<pre hidden>curl <span class="s">"https://your-workspace.archive.mulholland.ai/api/invoice?space=finance"</span> \
  -H <span class="s">"Authorization: Bearer $ARCHIVE_TOKEN"</span> \
  -d <span class="s">'&#123;"customer": "Hart", "amount": "€ 980"&#125;'</span>

<span class="c"># 201 &#123;"id": "0193…", "customer": "Hart", "amount": &#123;"amount": 980, "currency": "EUR"&#125;, …&#125;</span></pre>
<pre hidden>curl -X PATCH https://your-workspace.archive.mulholland.ai/api/invoice/&lt;id&gt; \
  -H <span class="s">"Authorization: Bearer $ARCHIVE_TOKEN"</span> \
  -d <span class="s">'&#123;"status": "paid"&#125;'</span>

<span class="c"># 200 &#123;"id": "0193…", "status": "paid", …&#125;</span></pre>
<pre hidden>curl -X DELETE https://your-workspace.archive.mulholland.ai/api/invoice/&lt;id&gt; \
  -H <span class="s">"Authorization: Bearer $ARCHIVE_TOKEN"</span>

<span class="c"># 204, no body</span></pre>
</Code>

<h3 id="api-rows">Rows</h3>
<table>
<thead><tr><th>Endpoint</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>GET /api/&#123;table&#125;</code></td><td>Lists rows. Other query parameters filter by exact value.</td></tr>
<tr><td><code>POST /api/&#123;table&#125;</code></td><td>Creates a row; <code>?space=</code> picks its space.</td></tr>
<tr><td><code>GET /api/&#123;table&#125;/&#123;id&#125;</code></td><td>One row.</td></tr>
<tr><td><code>PATCH /api/&#123;table&#125;/&#123;id&#125;</code></td><td>Changes only what you send.</td></tr>
<tr><td><code>PUT /api/&#123;table&#125;/&#123;id&#125;</code></td><td>Replaces the whole row.</td></tr>
<tr><td><code>DELETE /api/&#123;table&#125;/&#123;id&#125;</code></td><td>Deletes the row.</td></tr>
</tbody>
</table>

<h3 id="api-query">Query parameters</h3>
<table>
<thead><tr><th>Parameter</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>limit</code>, <code>after</code></td><td>Page size, and the last id you got for the next page.</td></tr>
<tr><td><code>space</code></td><td>On create, the space to write to. On list, only that space.</td></tr>
<tr><td><code>reveal=true</code></td><td>Shows secret values you are granted.</td></tr>
<tr><td><code>column=value</code></td><td>Any other parameter filters rows by that column.</td></tr>
</tbody>
</table>

<h3 id="api-more">Schema, search and code</h3>
<table>
<thead><tr><th>Endpoint</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>GET /api/types</code></td><td>Your tables and views, with their columns.</td></tr>
<tr><td><code>GET /api/schema</code></td><td>The DDL files.</td></tr>
<tr><td><code>POST /api/schema/plan</code>, <code>/apply</code></td><td>Plan or apply <code>&#123;"files", "renames", "allow_drop"&#125;</code>.</td></tr>
<tr><td><code>GET /api/search?q=</code></td><td>Search, with <code>type</code>, <code>space</code>, <code>from</code>, <code>to</code> and <code>limit</code>.</td></tr>
<tr><td><code>POST /api/run</code></td><td>Runs a script: <code>&#123;"code": "..."&#125;</code>.</td></tr>
<tr><td><code>GET /api/run/docs</code></td><td>The SDK reference with your table types.</td></tr>
<tr><td><code>/api/files…</code></td><td>Files, see <a href="#files-endpoints">Files</a>.</td></tr>
</tbody>
</table>

<h3 id="api-auth">Account endpoints</h3>
<table>
<thead><tr><th>Endpoint</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>GET /auth/me</code></td><td>Who you are, your spaces and your own space.</td></tr>
<tr><td><code>GET</code>, <code>POST /auth/spaces</code></td><td>List spaces, or create one with <code>&#123;"name"&#125;</code>.</td></tr>
<tr><td><code>DELETE /auth/spaces/&#123;space&#125;</code></td><td>Delete a space and everything in it.</td></tr>
<tr><td><code>GET</code>, <code>POST /auth/spaces/&#123;space&#125;/members</code></td><td>List members, or set one with <code>&#123;"email", "role"&#125;</code>.</td></tr>
<tr><td><code>GET</code>, <code>POST /auth/tokens</code></td><td>List or create tokens.</td></tr>
<tr><td><code>DELETE /auth/tokens/&#123;id&#125;</code></td><td>Revoke a token.</td></tr>
</tbody>
</table>

<h3 id="api-errors">Errors</h3>
<p>Errors come back as <code>&#123;"error": "..."&#125;</code>: 400 bad request, 401 no or bad token, 403 not allowed, 404 not found or not yours, 405 writing to a view, 409 conflict. Invalid input returns 422 with every wrong field.</p>
<Code tabs={['422']}>
<pre>&#123;"type": "invoice", "errors": [&#123;"path": "amount", "message": "…"&#125;]&#125;</pre>
</Code>

<h2 id="cli">The CLI</h2>
<p>The <code>archive</code> command line offers the same operations as the API and the SDK, for terminals and shell scripts. Output is JSON.</p>
<Code tabs={['terminal']}>
<pre>archive list invoice -limit <span class="n">5</span> status=unpaid
archive search northwind -type invoice -from <span class="n">2026-01-01</span>
archive schema pull ./schema &amp;&amp; archive schema plan ./schema</pre>
</Code>

<h3 id="cli-rows">Rows and search</h3>
<table>
<thead><tr><th>Command</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>archive list &lt;table&gt; [-limit n] [-after id] [-reveal] [column=value …]</code></td><td>Lists rows.</td></tr>
<tr><td><code>archive get &lt;table&gt; &lt;id&gt; [-reveal]</code></td><td>One row.</td></tr>
<tr><td><code>archive create &lt;table&gt; [-space s] [json]</code></td><td>Creates a row from the argument or stdin.</td></tr>
<tr><td><code>archive set &lt;table&gt; &lt;id&gt; [json]</code></td><td>Sets some columns; <code>null</code> clears one.</td></tr>
<tr><td><code>archive update &lt;table&gt; &lt;id&gt; [json]</code></td><td>Replaces the whole row.</td></tr>
<tr><td><code>archive delete &lt;table&gt; &lt;id&gt;</code></td><td>Deletes a row.</td></tr>
<tr><td><code>archive search &lt;query&gt; [-type t] [-space s] [-from d] [-to d] [-limit n]</code></td><td>Finds rows and files.</td></tr>
</tbody>
</table>
<Note mark="!">On the command line, <code>set</code> changes some columns and <code>update</code> replaces the row. In the SDK, <code>update</code> changes only the fields you pass.</Note>

<h3 id="cli-schema">Schema and code</h3>
<table>
<thead><tr><th>Command</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>archive types</code></td><td>Tables, columns and the built-in types.</td></tr>
<tr><td><code>archive schema pull [dir]</code></td><td>Writes one <code>.sql</code> file per table or view.</td></tr>
<tr><td><code>archive schema plan &lt;dir&gt; [-allow-drop]</code></td><td>Shows the plan for the files in <code>dir</code>.</td></tr>
<tr><td><code>archive schema apply &lt;dir&gt; [-allow-drop]</code></td><td>Runs it.</td></tr>
<tr><td><code>archive run &lt;file.ts|-&gt;</code></td><td>Runs a script and prints <code>&#123;result, logs, error&#125;</code>.</td></tr>
<tr><td><code>archive run -docs</code></td><td>Prints the SDK reference with your table types.</td></tr>
</tbody>
</table>

<h3 id="cli-files">Files</h3>
<table>
<thead><tr><th>Command</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>archive file add &lt;path&gt;… [-space s]</code></td><td>Stores files and extracts their text.</td></tr>
<tr><td><code>archive file get &lt;id&gt; [-o path|-]</code></td><td>Prints metadata, or writes the content with <code>-o</code>.</td></tr>
<tr><td><code>archive file list [-limit n] [-after id]</code></td><td>Lists files.</td></tr>
</tbody>
</table>

<h3 id="cli-access">Spaces, people and tokens</h3>
<table>
<thead><tr><th>Command</th><th>Does</th></tr></thead>
<tbody>
<tr><td><code>archive space add|list|remove|members</code></td><td>Manages spaces.</td></tr>
<tr><td><code>archive space grant &lt;space&gt; &lt;email&gt; &lt;role&gt;</code></td><td>Sets a role: owner, admin, editor, viewer or none.</td></tr>
<tr><td><code>archive user add &lt;email&gt; [-name n] [-admin]</code></td><td>Adds a person to the workspace and prints a one-time sign-in link for them.</td></tr>
<tr><td><code>archive user link &lt;email&gt;</code></td><td>Prints a new one-time sign-in link for someone in the workspace.</td></tr>
<tr><td><code>archive token create [-name n] [-space s:role,…] [-expires 720h] [-full]</code></td><td>Creates a token.</td></tr>
<tr><td><code>archive token list</code>, <code>revoke &lt;id&gt;</code></td><td>Lists or revokes tokens.</td></tr>
<tr><td><code>archive secret grant|revoke &lt;email&gt; &lt;type&gt;</code></td><td>Lets someone see a secret type.</td></tr>
</tbody>
</table>

<h2 id="spaces">Spaces</h2>
<p>A space is a shared box of rows and files. Every row and every file belongs to exactly one space, and people only see the spaces they are in.</p>

<h3 id="spaces-personal">Your own space</h3>
<p>Everyone gets a private space when they join. New rows and files go there unless you pick another, and it cannot be deleted.</p>

<h3 id="spaces-shared">Shared spaces</h3>
<p>Make one for a team, a client or a project. You become its owner.</p>
<Code tabs={['curl', 'run.ts']}>
<pre>curl https://your-workspace.archive.mulholland.ai/auth/spaces \
  -H <span class="s">"Authorization: Bearer $ARCHIVE_TOKEN"</span> \
  -d <span class="s">'&#123;"name": "finance"&#125;'</span></pre>
<pre hidden><span class="k">await</span> archive.<span class="f">create</span>(<span class="s">"invoice"</span>, &#123; customer: <span class="s">"Hart"</span>, amount: <span class="s">"€ 980"</span> &#125;, &#123; space: <span class="s">"finance"</span> &#125;)

<span class="k">return</span> archive.<span class="f">list</span>(<span class="s">"invoice"</span>, &#123; space: <span class="s">"finance"</span> &#125;)</pre>
</Code>

<h3 id="spaces-delete">Deleting a space</h3>
<p>Only an owner can delete a space, and it takes every row and file in it along. Deleting cannot be undone.</p>

<h2 id="people">People and roles</h2>
<p>Give someone a role in a space and they see what is in it. Roles are Postgres roles, so the database itself enforces them.</p>
<table>
<thead><tr><th>Role</th><th>Can</th></tr></thead>
<tbody>
<tr><td>viewer</td><td>Read rows and files.</td></tr>
<tr><td>editor</td><td>Also add, change and delete them.</td></tr>
<tr><td>admin</td><td>Also manage members and change the schema.</td></tr>
<tr><td>owner</td><td>Everything, including making owners and deleting the space.</td></tr>
</tbody>
</table>

<h3 id="people-add">Adding people</h3>
<p>A workspace admin adds the person by email. Then a space admin gives them a role, and they can sign in. Set the role to <code>none</code> to remove someone.</p>
<Code tabs={['curl']}>
<pre>curl https://your-workspace.archive.mulholland.ai/auth/spaces/finance/members \
  -H <span class="s">"Authorization: Bearer $ARCHIVE_TOKEN"</span> \
  -d <span class="s">'&#123;"email": "anna@example.com", "role": "editor"&#125;'</span></pre>
</Code>
<p>Making or removing an owner takes an owner, and a space always keeps at least one.</p>

<h3 id="people-signin">Signing in</h3>
<p>No passwords. Sign in with Google, Microsoft or a passkey. An admin can also make a one-time sign-in link with <code>archive user link &lt;email&gt;</code> and pass it on; it works once, for 15 minutes.</p>

<h3 id="people-security">Row-level security</h3>
<p>Every table has a policy: only rows from your spaces. It applies to the web app, the API, your AI and raw SQL alike, so a bug in one surface cannot show another person's data.</p>

<h2 id="secrets">Secret fields</h2>
<p>Some values should never sit in plain text. Columns of a secret type are encrypted, read back masked, and shown only to people granted that type.</p>

<h3 id="secrets-types">Secret types</h3>
<table>
<thead><tr><th>Type</th><th>Holds</th></tr></thead>
<tbody>
<tr><td><code>archive.card</code></td><td>Payment card numbers</td></tr>
<tr><td><code>archive.us_ssn</code></td><td>US Social Security numbers</td></tr>
<tr><td><code>archive.nl_bsn</code></td><td>Dutch citizen service numbers</td></tr>
<tr><td><code>archive.national</code></td><td>National identity numbers</td></tr>
<tr><td><code>archive.passport</code></td><td>Passport numbers</td></tr>
</tbody>
</table>

<h3 id="secrets-reveal">Writing and reading</h3>
<p>Write the real value as usual. Reads return it masked. Pass <code>&#123; reveal: true &#125;</code> to <code>list</code> or <code>get</code>, or <code>?reveal=true</code> over HTTP, to see it if you are granted.</p>

<h3 id="secrets-grant">Granting</h3>
<p>Workspace admins grant one secret type to one person at a time, with <code>archive secret grant anna@example.com passport</code>, and revoke it the same way.</p>

<h2 id="connectors">Connectors</h2>
<p>Connectors bring a space's data in from the tools your team already uses. They are read-only, and they sync only what changed.</p>

<h3 id="connectors-how">How they sync</h3>
<ul>
<li><strong>Read-only.</strong> Connectors ask for read scopes and never write back.</li>
<li><strong>Per space.</strong> A connection belongs to one space, and what it syncs lands there under row-level security.</li>
<li><strong>By delta cursor.</strong> Each connection keeps the provider's cursor and fetches only changes, woken by change notifications where the provider sends them.</li>
<li><strong>Newest first.</strong> The first sync walks the full history once, newest first, so recent data is searchable soonest.</li>
<li><strong>Deletes follow.</strong> Deleting something at the source deletes it in Archive.</li>
</ul>

<h3 id="connectors-land">Where data lands</h3>
<p>Mail, chat messages, calendar events, issues, CRM records, tickets and finance records become rows in built-in tables. Files and attachments become Archive files, text-extracted like uploads; Google Docs, Sheets and Slides arrive as docx, xlsx and pptx. Your AI then reads what landed and files it into your own types.</p>

<h3 id="connectors-list">Providers</h3>
<table>
<thead><tr><th>Area</th><th>Connectors</th></tr></thead>
<tbody>
<tr><td>Mail and calendar</td><td>Gmail, Google Calendar, Outlook mail, Outlook calendar</td></tr>
<tr><td>Files</td><td>Google Drive, SharePoint and OneDrive, Dropbox, Box</td></tr>
<tr><td>Chat</td><td>Slack, Microsoft Teams, Google Chat</td></tr>
<tr><td>Docs</td><td>Notion, Confluence</td></tr>
<tr><td>Work and code</td><td>Jira, Linear, Asana, GitHub, GitLab</td></tr>
<tr><td>CRM and support</td><td>HubSpot, Salesforce, Pipedrive, Zendesk, Intercom</td></tr>
<tr><td>Finance and meetings</td><td>Stripe, Xero, QuickBooks Online, Exact Online, Zoom</td></tr>
</tbody>
</table>
<Note>Connectors roll out provider by provider, as each provider approves the Archive app. Services that issue API tokens, such as Stripe, Notion, Linear and GitHub, connect with a pasted token.</Note>

<h2 id="models">Models</h2>
<p>Every row you file is a labelled example. Export a dataset per table, fine-tune a small model on it, and call it from your agents.</p>
<ul>
<li><strong>Classifier</strong>: picks the type for an input, so intake knows where a file belongs.</li>
<li><strong>Filer</strong>: turns text into rows of your schema.</li>
</ul>
<p>See <a href="/models">Models</a> for how training and serving work.</p>

<h2 id="help">Getting help</h2>
<p>Most errors say what went wrong and how to fix it: invalid input names every bad field, and failed edits name the edit. If you are still stuck, we are close by.</p>

<h3 id="help-issues">Bugs and requests</h3>
<p>Report a bug or ask for a feature on <a href="https://github.com/Mulholland-Inc/archive/issues">GitHub issues</a>.</p>

<h3 id="help-contact">Talk to us</h3>
<p>Write to <a href="mailto:hello@mulholland.ai">hello@mulholland.ai</a> for questions about your workspace, your plan or your data.</p>
  </article>

  <aside class="docs-toc" aria-label="On this section">
    {#if toc.length}
      <h4>On this section</h4>
      {#each toc as sub}<a href="#{sub.id}">{sub.text}</a>{/each}
    {/if}
  </aside>
</div>

<Cta title="Build on a real database." lead="Free for your first workspace. Connect your AI in minutes." links={[START, ['/developers', 'For developers']]} />
