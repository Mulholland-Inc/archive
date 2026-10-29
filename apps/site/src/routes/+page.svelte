<script>
  import Cta from '#lib/Cta.svelte';
  import Hero from '#lib/Hero.svelte';
  import Letters from '#lib/Letters.svelte';
  import Logos from '#lib/Logos.svelte';
  import Scene from '#lib/Scene.svelte';
  import Section from '#lib/Section.svelte';
  import Specs from '#lib/Specs.svelte';
  import Story from '#lib/Story.svelte';
  import { START } from '#lib/site.js';

  const STORY = [
    { art: 'story-1', alt: 'A filing cabinet, a data silo, a mailbox and a spreadsheet board, each standing apart', title: 'Today, it is scattered.', text: 'Files sit in SharePoint and Google Drive, numbers in Snowflake and NetSuite, decisions in Outlook and Slack.' },
    { art: 'story-2', alt: 'The same four tools tied together with loose cables, papers on the ground', title: 'Held together by hand.', text: 'Spreadsheets and forms stand in for structure, and AI assistants glue it all together but remember nothing.' },
    { art: 'story-3', alt: 'The four tools moving onto one shared platform, joined by one straight conduit', title: 'Archive brings it together.', text: 'Your documents, data and conversations move into one place, organized the way your business works.' },
    { art: 'story-4', alt: 'A working factory with a wall of drawers, a data silo on the roof, an intake bay and a lit panel, with conveyors carrying mail in and sorted stacks out', title: 'Archive keeps it running.', text: 'Your team, your assistants and your own apps work from the same source, every day.' },
    { art: 'story-5', alt: 'The same factory with new service modules around it, plugged in by pipes, one of them still being connected', title: 'Archive grows with you.', text: 'Connect new services, or have an AI coding agent like Claude Code build what you need. It is open, so you can change anything.' }
  ];

  const CONNECTORS = ['Salesforce', 'Notion', 'Shopify', 'Microsoft', 'Google Workspace', 'Slack', 'HubSpot', 'Stripe', 'QuickBooks', 'Xero', 'Zendesk', 'Intercom', 'Jira', 'Confluence', 'GitHub', 'Linear', 'Asana', 'monday.com', 'Gmail', 'Microsoft Teams', 'OneDrive', 'Google Drive', 'Dropbox', 'Box', 'Airtable', 'Snowflake', 'Databricks', 'Zoom', 'Mailchimp', 'Google Calendar', 'Google Sheets', 'Trello', 'Figma', 'Zapier'];

  const PRODUCTS = [
    { href: '/ontology', title: 'Ontology', art: 'ontology', text: 'You decide what your business runs on, and keep it.' },
    { href: '/developers', title: 'Developers', art: 'api', text: 'Build your own apps and workflows on your data.' },
    { href: '/agents', title: 'Agents', art: 'agents', text: 'Works with ChatGPT, Claude, Gemini and Grok.' },
    { href: '/models', title: 'Models', art: 'models', text: 'Small AI models that know your business.' }
  ];

  const ORGANIZE = [
    { art: 'delta', title: 'Change it anytime', items: ['Add a field or a new kind of record in seconds', 'See what a change will do before it happens', 'Rename anything and nothing gets lost'] },
    { art: 'views', title: 'Boards and calendars', items: ['See the same records as a board, a calendar or a list', 'Group them the way your team thinks', 'Everyone sees only what they are allowed to'] },
    { art: 'security', title: 'Only the right people', items: ['Everything belongs to a space for a team or a person', 'Set who can see and change what', 'Your AI assistants never see more than you do'] }
  ];

  const STEPS = [
    ['Collect', 'Gather examples straight from your own records, ready to learn from.'],
    ['Train', 'Teach a model to recognize what each new item is, or to fill in your records from plain text.'],
    ['Use', 'Your assistants call on it whenever they need it, right next to your data.']
  ];

  const TOOLS = ['Gmail', 'Outlook', 'Google Drive', 'SharePoint', 'Slack', 'Teams', 'Notion', 'Jira', 'Linear', 'GitHub', 'HubSpot', 'Salesforce', 'Zendesk', 'Stripe', 'QuickBooks', '+ 13 more'];
</script>

<svelte:head>
  <title>Archive — own your data infrastructure</title>
  <meta name="description" content="Database. File storage. Agent access. A foundation for your digital world, built to run on your terms." />
</svelte:head>

<Hero
  kind="home"
  title="Own your data infrastructure."
  lead="A foundation for your digital world, built to run on your terms."
  badges={['SOC 2', 'HIPAA', 'ISO 27001']}
/>

<Section wide>
  <Logos marquee items={CONNECTORS} />
</Section>

<Section center>
  {#snippet title()}One place for all of it. <em>Open and owned by you.</em>{/snippet}
  <Story steps={STORY} />
</Section>

<Section center>
  {#snippet title()}Everything your business runs on. <em>Built as one.</em>{/snippet}
  <Specs items={PRODUCTS} />
</Section>

<Section title="Messy in, tidy out." body="Email, files and chats arrive however they arrive. Your AI assistant reads each one, works out what it is, and files it neatly in the right place.">
  <Scene name="factory" alt="A factory turning envelopes, sheets and folders into neat stacks stored in a silo" />
</Section>

<Section band title="Organized your way, never locked away." body="Every kind of record you keep has its own home you can open, sort and search. Nothing hides in a format only one app understands.">
  <Specs items={ORGANIZE} />
</Section>

<Section title="Your work trains your models." body="Everything your team and assistants file is an example of how your business works. Turn it into a small model that knows your business.">
  <Scene name="lab" alt="A model foundry: storage silos feed a training rig that stamps out model chips" />
  <Letters columns steps={STEPS} />
</Section>

<Section band title="Connect once. Stay in sync." body="Bring in the mail, files, chats and business tools you already use. Archive only reads, picks up just what is new, and keeps it all in one place.">
  <Scene name="harbor" alt="A harbor where barges of containers are unloaded into a warehouse" />
  <Logos items={TOOLS} />
</Section>

<Cta
  scene="city"
  alt="A city block whose buildings are records, pages, files, a vault and an antenna for AI assistants"
  title="Start building in minutes."
  lead="Free for your first workspace. No card, no sales call."
  links={[START, ['/pricing', 'See pricing']]}
/>
