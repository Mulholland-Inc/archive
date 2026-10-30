<script>
  import CopyPrompt from '#lib/CopyPrompt.svelte';
  import Cta from '#lib/Cta.svelte';
  import Facts from '#lib/Facts.svelte';
  import Faq from '#lib/Faq.svelte';
  import Hero from '#lib/Hero.svelte';
  import Meta from '#lib/Meta.svelte';
  import Row from '#lib/Row.svelte';
  import Section from '#lib/Section.svelte';
  import Specs from '#lib/Specs.svelte';
  import { GITHUB, START } from '#lib/site.js';

  const PROMPT = [
    `Help me self-host Archive, the open-source data server at ${GITHUB}.`,
    '1. Clone the repo, then read README.md, Dockerfile, compose.yaml and the Environment section of src/main.go before you change anything.',
    '2. Check what I have installed. Prefer Docker; if I do not have it, build from source with Go 1.27 or newer: cd src && go build -o archive .',
    '3. Ask me which email should be the first owner, and which sign-in provider I want (Google, Microsoft, Slack or GitHub); set its ARCHIVE_<PROVIDER>_CLIENT_ID and ARCHIVE_<PROVIDER>_CLIENT_SECRET.',
    '4. Archive needs Postgres 18 in DATABASE_URL. docker compose up -d starts one next to it; to use my own, set DATABASE_URL to a role with CREATEROLE and CREATEDB.',
    '5. Start it with docker compose up -d from the repo root: Postgres, the API on port 8080 and the web app on port 3000. From source, run docker compose up -d postgres, then ./archive serve with DATABASE_URL set, and npm run dev in apps/web for the web app.',
    '6. Check that it is up: http://localhost:8080/healthz should answer 204 and http://localhost:3000 should show the web app. Then add me as the first owner with archive user add and my email.',
    '7. Make me an access token with archive token create and show me how to connect Claude Code to http://localhost:8080/mcp.',
    '8. If I want it public, give the web app and the API each an HTTPS address, set ARCHIVE_URL and ARCHIVE_API_URL to them, and use apps/terraform/gcp for Google Cloud. Ask me before you create any cloud resources.',
    'Never write secrets into files that get committed, and tell me each step before you run it.'
  ].join('\n');

  const WAYS = [
    ['Your AI agent', 'Claude, ChatGPT, Gemini, Cursor and others connect over MCP and run scripts for you'],
    ['The API', 'Records, files and search from your own apps and services'],
    ['The command line', 'Scripts, scheduled jobs and quick changes from your terminal'],
    ['Terraform', 'Your structure, spaces and access keys kept with the rest of your setup'],
    ['Connectors', 'Mail, calendars, files, chat and business tools, kept up to date']
  ];

  const RUN = [
    { art: 'cli', title: 'On your computer', items: ['One command starts it with Docker', 'Run it again to update; your data stays', 'Postgres runs in its own container'] },
    { art: 'scale', title: 'On your servers', items: ['Two containers: the API and the web app', 'Use your own Postgres and file storage', 'Run as many copies side by side as you like'] },
    { art: 'harbor', title: 'In your cloud', items: ['A ready-made setup for Google Cloud', 'Your account, your database, your storage', 'Or let us run it for you'] }
  ];

  const FAQ = [
    ['Do I need to write code to change my structure?', 'No. Tell your agent what you want. It shows you the plan, and applies it only after you say yes.'],
    ['What can a script do?', 'It works with your records, files and documents, as you, and sees only what you can. It cannot reach the internet, and it has 30 seconds by default. If it fails or runs out of time, nothing is saved.'],
    ['Can I keep my structure in git?', 'Yes. Pull it as one file per kind of record, edit, preview and apply. Applying what you just pulled changes nothing, so the files stay stable.'],
    ['What do I need to run it myself?', 'Docker is enough to start. For production, add your own Postgres and file storage and put it behind HTTPS. The prompt at the top of this page walks Claude Code through it.'],
    ['Does Archive call an AI model?', 'No. Archive never calls a model on its own. Your agent does the thinking and sends Archive code to run, so you keep using the AI you already have.'],
    ['Can I change Archive itself?', 'Yes. The code is on GitHub. Fork it, change it by hand or with Claude Code, and run your own version.']
  ];
</script>

<Meta title="Developers — Archive" description="Shape your data with your AI agent, let it run code inside Archive, connect your other tools, and self-host or fork the open-source server." />

<Hero
  title="Build on your data from anywhere."
  lead="Shape your data by talking to your AI agent, let it run code right next to your data, and connect the tools you already use. Archive is open source, so you can run it yourself and make it your own."
  art="api"
  alt="A switchboard connecting a terminal, a browser and an AI assistant to one place"
  links={[START]}
>
  {#snippet actions()}<CopyPrompt text={PROMPT} />{/snippet}
</Hero>

<Section body="Most of the work happens in a conversation with your agent. The rest is yours to change.">
  {#snippet title()}Talk, run, connect. <em>Own it.</em>{/snippet}
  <Row art="schema" alt="A blueprint being stamped into place">
    <h2>Shape your data with an agent.</h2>
    <p>Tell your agent what you keep track of, like clients, invoices and projects, and how they link. It proposes the kinds of records, their fields and the connections between them.</p>
    <p>You see exactly what will change before anything is applied. Rename or reshape anytime: a renamed field keeps everything in it, and nothing is deleted unless you say so.</p>
  </Row>
  <Row flip art="code" alt="A code editor whose lines snap neatly into place">
    <h2>Your agent writes code. Archive runs it.</h2>
    <p>Your agent writes one short script, and Archive runs it right next to your data, as you, with your permissions.</p>
    <p>Sorting hundreds of files or updating a whole list happens in one go, not a hundred back-and-forths. If anything fails, nothing is saved, so no job is left half done.</p>
  </Row>
  <Row art="connectors" alt="Cables from mail, chat and file services plugged into one Archive socket">
    <h2>Connect it to everything else.</h2>
    <p>Bring in mail, calendars, files, chat and business tools like Gmail, Slack, Google Drive, HubSpot and Stripe. Connectors only read, and keep up as things change.</p>
    <p>Reach Archive from your own apps, scripts and services through the API and the command line, with the same permissions everywhere.</p>
  </Row>
  <Row flip art="factory" alt="A factory turning a jumble of documents into neatly packed crates">
    <h2>Open source. Make it yours.</h2>
    <p>Run Archive on your own computer, your own servers or your own cloud account, and keep your data where you want it.</p>
    <p>Change it or fork it anytime. Ask Claude Code to add what you need and adapt it into your own version.</p>
  </Row>
</Section>

<Section band title="Run it where you like." body="One program on top of Postgres. Pick the setup that fits, and move between them as you grow.">
  <Specs items={RUN} />
</Section>

<Section title="Every way in." body="The same data and the same permissions, whichever you pick.">
  <Facts head={['Way in', 'Good for']} rows={WAYS} />
</Section>

<Section band title="Developer questions.">
  {#snippet body()}The full reference is in the <a class="link" href="/docs">docs</a>.{/snippet}
  <Faq items={FAQ} />
</Section>

<Cta title="Build on your own data." lead="Free for your first workspace, or run your own copy from GitHub." links={[START, [GITHUB, 'View on GitHub']]} />
