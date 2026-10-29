<script>
  import Art from '#lib/Art.svelte';
  import Button from '#lib/Button.svelte';
  import Card from '#lib/Card.svelte';
  import Comp from '#lib/Comp.svelte';
  import Cta from '#lib/Cta.svelte';
  import Hero from '#lib/Hero.svelte';
  import TextBlock from '#lib/TextBlock.svelte';
  import { reveal } from '#lib/reveal.js';
  import { APP } from '#lib/site.js';

  const STEPS = [
    ['s-4 at-1 row-1', 'Collect', 'Gather examples straight from your own records, ready to learn from.'],
    ['s-3 at-5 row-1 down-2', 'Train', 'Teach a model to recognize what each new item is, or to fill in your records from plain text.'],
    ['s-4 at-9 row-1 down-1', 'Use', 'Your assistants call on it whenever they need it, right next to your data.']
  ];

  const TOOLS = ['Gmail', 'Outlook', 'Google Drive', 'SharePoint', 'Slack', 'Teams', 'Notion', 'Jira', 'Linear', 'GitHub', 'HubSpot', 'Salesforce', 'Zendesk', 'Stripe', 'QuickBooks', '+ 13 more'];
</script>

<svelte:head>
  <title>Archive — own your data infrastructure</title>
  <meta name="description" content="Database. File storage. Agent access. A foundation for your digital world, built to run on your terms." />
  <meta name="theme-color" content="#dce8f7" />
</svelte:head>

<Hero
  home
  title="Own your data infrastructure."
  subtitle="Database. File storage. Agent access."
  lead="A foundation for your digital world, built to run on your terms."
  art="hero"
  alt="An isometric machine joining your records, a page, a file cabinet and a chat window"
/>

<Comp>
  <Card class="s-6 at-1 row-1" href="/tables" n="1" title="Tables" art="tables" text="Your own records, shaped the way your business works." />
  <Card class="s-4 at-9 row-1 down-2" href="/pages" n="2" title="Pages" art="pages" text="Documents your team and your AI write together." />
  <Card class="s-4 at-2 row-2" href="/agents" n="3" title="Agents" art="agents" text="Works with ChatGPT, Claude, Gemini and Grok." />
  <Card class="s-5 at-7 row-2 down-2" href="/models" n="4" title="Models" art="models" text="Small AI models that know your business." />
</Comp>

<section class="section">
  <div class="wrap">
    <div class="head">
      <h2 class="reveal" use:reveal>Messy in, <em>tidy</em> out.</h2>
      <p class="body reveal" use:reveal>Email, files and chats arrive however they arrive. Your AI assistant reads each one, works out what it is, and files it neatly in the right place.</p>
    </div>
  </div>
  <div class="scene-bleed reveal" use:reveal><Art name="factory" alt="A factory turning envelopes, sheets and folders into neat stacks stored in a silo" /></div>
</section>

<Comp band>
  <TextBlock class="s-5 at-1 row-1">
    <h2>Organized your way, never locked away.</h2>
    <p>Every kind of record you keep has its own home you can open, sort and search. Nothing hides in a format only one app understands, so your information stays yours.</p>
  </TextBlock>
  <TextBlock spec class="s-6 at-7 row-1 rows-2" art="schema" title="Change it anytime" items={['Add a field or a new kind of record in seconds', 'See what a change will do before it happens', 'Rename anything and nothing gets lost']} />
  <TextBlock spec class="s-5 at-1 row-2 down-1" art="views" title="Boards and calendars" items={['See the same records as a board, a calendar or a list', 'Group them the way your team thinks', 'Everyone sees only what they are allowed to']} />
  <TextBlock spec class="s-4 at-6 row-3 up-1" art="security" title="Only the right people" items={['Everything belongs to a space for a team or a person', 'Set who can see and change what', 'Your AI assistants never see more than you do']} />
</Comp>

<section class="band">
  <div class="wrap section">
    <div class="head">
      <div class="stack"><h2 class="reveal" use:reveal>Your work trains <em>your</em> models.</h2></div>
      <p class="body reveal" use:reveal>Everything your team and assistants file is an example of how your business works. Turn it into a small model that knows your business.</p>
    </div>
  </div>
  <div class="scene-bleed reveal" use:reveal><Art name="lab" alt="A model foundry: storage silos feed a training rig that stamps out model chips" /></div>
  <div class="wrap">
    <ul class="letters comp">
      {#each STEPS as [cls, name, text], i}
        <li class="{cls} reveal" use:reveal><span class="l">{String.fromCharCode(65 + i)}</span><div><h3>{name}</h3><p>{text}</p></div></li>
      {/each}
    </ul>
  </div>
</section>

<section class="band">
  <div class="wrap section">
    <div class="head">
      <div class="stack"><h2 class="reveal" use:reveal>Connect once. Stay in sync.</h2></div>
      <p class="body reveal" use:reveal>Bring in the mail, files, chats and business tools you already use. Archive only reads, picks up just what is new, and keeps it all in one place.</p>
    </div>
  </div>
  <div class="scene-bleed reveal" use:reveal><Art name="harbor" alt="A harbor where barges of containers are unloaded into a warehouse" /></div>
  <div class="wrap section">
    <div class="logos">{#each TOOLS as tool}<span>{tool}</span>{/each}</div>
  </div>
</section>

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
