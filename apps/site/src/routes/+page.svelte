<script>
  import Button from '#lib/Button.svelte';
  import Card from '#lib/Card.svelte';
  import Cta from '#lib/Cta.svelte';
  import Hero from '#lib/Hero.svelte';
  import Scene from '#lib/Scene.svelte';
  import Section from '#lib/Section.svelte';
  import Specs from '#lib/Specs.svelte';
  import { APP } from '#lib/site.js';

  const ORGANIZE = [
    { art: 'schema', title: 'Change it anytime', items: ['Add a field or a new kind of record in seconds', 'See what a change will do before it happens', 'Rename anything and nothing gets lost'] },
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
  <meta name="theme-color" content="#0b0d12" />
</svelte:head>

<Hero
  home
  title="Own your data infrastructure."
  subtitle="Database. File storage. Agent access."
  lead="A foundation for your digital world, built to run on your terms."
  art="landscape"
  alt="People working in an open office, robots sorting and scanning their documents, and everything filed into cabinets in a records room"
/>

<Section>
  <div class="cards">
    <Card href="/tables" title="Tables" art="tables" text="Your own records, shaped the way your business works." />
    <Card href="/pages" title="Pages" art="pages" text="Documents your team and your AI write together." />
    <Card href="/agents" title="Agents" art="agents" text="Works with ChatGPT, Claude, Gemini and Grok." />
    <Card href="/models" title="Models" art="models" text="Small AI models that know your business." />
  </div>
</Section>

<Section title="Messy in, tidy out." body="Email, files and chats arrive however they arrive. Your AI assistant reads each one, works out what it is, and files it neatly in the right place.">
  <Scene name="factory" alt="A factory turning envelopes, sheets and folders into neat stacks stored in a silo" />
</Section>

<Section band title="Organized your way, never locked away." body="Every kind of record you keep has its own home you can open, sort and search. Nothing hides in a format only one app understands.">
  <Specs items={ORGANIZE} />
</Section>

<Section title="Your work trains your models." body="Everything your team and assistants file is an example of how your business works. Turn it into a small model that knows your business.">
  <Scene name="lab" alt="A model foundry: storage silos feed a training rig that stamps out model chips" />
  <ul class="letters three">
    {#each STEPS as [name, text], i}
      <li><span class="l">{String.fromCharCode(65 + i)}</span><div><h3>{name}</h3><p>{text}</p></div></li>
    {/each}
  </ul>
</Section>

<Section band title="Connect once. Stay in sync." body="Bring in the mail, files, chats and business tools you already use. Archive only reads, picks up just what is new, and keeps it all in one place.">
  <Scene name="harbor" alt="A harbor where barges of containers are unloaded into a warehouse" />
  <div class="logos">{#each TOOLS as tool}<span>{tool}</span>{/each}</div>
</Section>

<Cta
  scene="city"
  alt="A city block whose buildings are records, pages, files, a vault and an antenna for AI assistants"
  title="Start building in minutes."
  lead="Free for your first workspace. No card, no sales call."
>
  {#snippet actions()}
    <Button href={APP}>Start building free</Button>
    <Button href="/pricing" ghost>See pricing</Button>
  {/snippet}
</Cta>
