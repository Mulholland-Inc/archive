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
  import { APP } from '#lib/site.js';

  const SOON = 'As soon as it changes';
  const MINUTE = 'Within a minute';

  const CONNECTORS = [
    'Mail and calendar',
    ['Gmail', 'Messages and attachments', MINUTE],
    ['Google Calendar', 'Events', SOON],
    ['Outlook mail', 'Messages and attachments, per folder', SOON],
    ['Outlook calendar', 'Events', SOON],
    'Files',
    ['Google Drive', 'Files, with Docs, Sheets and Slides as Word, Excel and PowerPoint', SOON],
    ['SharePoint and OneDrive', 'Files in every drive you can read', SOON],
    ['Dropbox', 'Files', SOON],
    ['Box', 'Files', SOON],
    'Chat',
    ['Slack', 'Channel, private and direct messages, threads, files and people', SOON],
    ['Microsoft Teams', 'Channel messages and chats', SOON],
    ['Google Chat', 'Messages in spaces', SOON],
    'Docs and knowledge',
    ['Notion', 'Pages and databases', MINUTE],
    ['Confluence', 'Pages and their content', SOON],
    'Work tracking and code',
    ['Jira', 'Issues', SOON],
    ['Linear', 'Issues', SOON],
    ['Asana', 'Tasks and projects', SOON],
    ['GitHub', 'Issues, pull requests and repository contents', MINUTE],
    ['GitLab', 'Issues, merge requests and projects', SOON],
    'CRM and support',
    ['HubSpot', 'Contacts, companies, deals and other CRM records', MINUTE],
    ['Salesforce', 'Accounts, contacts, opportunities and other records', SOON],
    ['Pipedrive', 'Deals, people, organizations and activities', SOON],
    ['Zendesk', 'Tickets', SOON],
    ['Intercom', 'Conversations and contacts', MINUTE],
    'Finance and meetings',
    ['Stripe', 'Customers, invoices, payments and subscriptions', SOON],
    ['Xero', 'Transactions and contacts', MINUTE],
    ['QuickBooks Online', 'Invoices, bills, payments and customers', MINUTE],
    ['Exact Online', 'Accounting records', SOON],
    ['Zoom', 'Meetings and recordings', MINUTE]
  ];

  const FAQ = [
    ['Does Archive ever change anything in my tools?', 'No. Connectors only ask to read and never write back. To act on what came in, your assistants work in Archive.'],
    ['How up to date is it?', 'Most tools tell Archive the moment something changes. The rest are checked every minute while an account is busy, and up to once an hour when it goes quiet.'],
    ['What happens when I delete something at the source?', 'It is deleted in Archive too. Everything that came in follows the original.'],
    ['Who can see what comes in?', 'The members of the space the connection belongs to, according to what each may see. The same rules apply to people and assistants alike.'],
    ['Can my assistants use it?', 'Yes. Mail, messages and events become records, and files stay files. Your assistants search and work with them like anything else.']
  ];
</script>

<svelte:head>
  <title>Connectors — Archive</title>
  <meta name="description" content="Bring in mail, calendars, files, chat, docs, projects, customers and finance from the tools you already use. Archive only reads, keeps up as things change, and puts it all in one place." />
  <meta name="theme-color" content="#d6f3e4" />
</svelte:head>

<Hero
  stack
  title="All your tools, in one place."
  lead="Mail, calendars, files, chat, docs, projects, customers and finance flow into Archive and stay up to date. Archive only reads, and the newest things arrive first."
  art="connectors"
  alt="Cables from mail, chat and file services plugged into one Archive socket"
>
  {#snippet actions()}
    <Button href={APP}>Start building free</Button>
    <Button href="#all" ghost>See every connector</Button>
  {/snippet}
</Hero>

<section class="section">
  <div class="wrap">
    <div class="head">
      <h2 class="reveal" use:reveal>Connect once. <em>Stay</em> up to date.</h2>
      <p class="body reveal" use:reveal>Each connection brings in only what is new and puts it in one space, where people and assistants can use it right away.</p>
    </div>
  </div>
  <div class="scene-bleed reveal" use:reveal><Art name="harbor" alt="A harbor where barges of containers are unloaded into a warehouse" /></div>
  <div class="wrap">
    <div class="comp">
      <Art class="s-7 bleed-l" reveal name="delta" alt="Two snapshots of a stack with only the new cards highlighted" />
      <TextBlock class="s-4 at-9 down-1">
        <h2>Only what changed.</h2>
        <p>Archive brings in only what is new or changed since last time, and nothing gets skipped or lost.</p>
        <p>Most tools tell Archive the moment something changes, and the rest are checked every minute, less often once an account goes quiet.</p>
      </TextBlock>
      <h2 class="c-big s-5 at-2 down-1 reveal" use:reveal>Newest first.</h2>
      <Art class="s-5 at-8 rows-2 down-1" reveal name="search" alt="A magnifying glass over a timeline, the newest end lit first" />
      <TextBlock class="s-4 at-2">
        <p>When you first connect, Archive brings in your history newest first, so this week's mail is searchable before last year's.</p>
        <p>Delete something in the original tool and it disappears from Archive too.</p>
      </TextBlock>
      <Art class="s-8 at-1 bleed-l down-2" reveal name="tables" alt="Crates being unpacked onto shelves" />
      <TextBlock class="s-4 at-9">
        <h2>Ready to search and use.</h2>
        <p>Mail, messages, events, issues, customer records, tickets and invoices arrive as records you can search, sort and link like anything else.</p>
        <p>Files arrive just like uploads, and Google Docs, Sheets and Slides come in as Word, Excel and PowerPoint files.</p>
      </TextBlock>
    </div>
  </div>
</section>

<Section id="all" title="Every connector." body="Seven areas a team runs on. Each connector only reads, and keeps up as things change.">
  <Facts head={['Connector', 'What it brings in', 'How changes arrive']} rows={CONNECTORS} />
</Section>

<Comp band>
  <TextBlock class="s-5">
    <h2>Built to be <em>trusted</em> with your inbox.</h2>
    <p>Connectors only read, put things where they belong, and stay out of the way.</p>
  </TextBlock>
  <Art class="s-6 at-7 rows-2 bleed-r" reveal name="security" />
  <TextBlock class="s-4 at-2 down-1" title="Read-only" items={['Permission to read, nothing more, on every tool', 'Archive never changes anything there', 'Sign-in details are stored encrypted']} />
  <TextBlock class="s-4 at-1 down-1" art="spaces" title="Per space" items={['Each connection belongs to one space', 'Everything it brings in lands there', 'Only the right people see it']} />
  <TextBlock class="s-5 at-7 down-3" art="office" title="Office files" items={['Google Docs, Sheets and Slides as Word, Excel and PowerPoint', 'Drive files handled just like uploads', 'Assistants can open and read them']} />
</Comp>

<Section title="Questions">
  <Faq items={FAQ} />
</Section>

<Cta title="Bring your tools in." lead="Connect your first account in a free workspace. No card, no sales call." />
