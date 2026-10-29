<script>
  import Comp from '#lib/Comp.svelte';
  import Cta from '#lib/Cta.svelte';
  import Button from '#lib/Button.svelte';
  import Hero from '#lib/Hero.svelte';
  import Section from '#lib/Section.svelte';
  import TextBlock from '#lib/TextBlock.svelte';
  import Timeline from '#lib/Timeline.svelte';
  import { APP } from '#lib/site.js';

  const ENTRIES = [
    {
      id: '2026-10-28-docs',
      title: 'Docs for every part of Archive',
      items: [
        'A new website with guides to everything you can do: shape your own records, change them as you go, build boards and calendars, search, store files, write pages and edit Office documents.',
        'A full reference for developers, covering every script command and every developer tool.',
        'Step-by-step guides for connecting Claude Code, Cursor, Claude Desktop, ChatGPT and claude.ai.'
      ]
    },
    {
      id: '2026-10-27-views',
      title: 'Boards, calendars and an icon for everything',
      items: [
        'Save boards, calendars and filtered lists, and each person sees only the records they are allowed to see.',
        'Pick an icon and a layout for each one in its settings. A typo is caught before anything changes.',
        'Every kind of record gets an icon of its own, and a new section in the app shows each board in its layout.',
        'Boards are for looking; to change something, you edit the record itself.'
      ]
    },
    {
      id: '2026-10-27-web',
      title: 'A new web app',
      items: [
        'Sign in, browse your files, open pages in the live editor, and get a page for every list and board.',
        'Each list shows its own icon and description.',
        'Simple web addresses, so you can link straight to a list, a board or your files.',
        'One clean, consistent look across the whole app.'
      ]
    },
    {
      id: '2026-10-27-pages',
      title: 'Write together, live',
      items: [
        "People and AI assistants can edit the same page at the same time, and nobody's edits get overwritten.",
        "See who is there, where they are typing and what they have selected. Your AI assistant's edits show up live too.",
        'Edits always land in the right paragraph, even if the page changed in the meantime.',
        'Start a blank page, or a blank Word, Excel or PowerPoint file, with a single command.'
      ]
    },
    {
      id: '2026-10-27-schema',
      title: 'What you see is what is there',
      items: [
        'The shape of your data is always read fresh, so what you see is exactly what is stored.',
        'Records that point to each other can be set up in any order.',
        'Uploads are ready the moment they finish, with their text, details and previews included.',
        'Edits to a live page reach everyone who has it open as soon as they are saved.'
      ]
    },
    {
      id: '2026-10-27-office',
      title: 'Edit Word, Excel and PowerPoint in place',
      items: [
        "Your AI assistant can read a document's paragraphs, cells and slides and change them. Changes are all-or-nothing, so nothing ends up half done.",
        'Only what you change changes. Formatting, images, comments and charts stay exactly as they were.',
        'Replace text, add and remove paragraphs, fill in cells and formulas, add lines to Excel tables, and rewrite slides.',
        'When several assistants edit the same file, they take turns, so no edit is ever lost.'
      ]
    },
    {
      id: '2026-10-27-tables',
      title: 'Records shaped your way, and AI that works on them',
      items: [
        'Create your own kinds of records, shaped the way your business works, and link, sort and search them however you like.',
        'Change the structure anytime: preview what a change will affect, then apply it. Nothing is deleted unless you say so.',
        'Renaming a field keeps everything in it, even when you are only previewing.',
        'Your AI assistant can do several steps of work in one go, with your permissions, and either all of it is saved or none of it is.',
        'Default values fill themselves in when you add a record.'
      ]
    },
    {
      id: '2026-10-23-dashboard',
      title: 'The first dashboard',
      items: [
        'A web dashboard where every record opens in a Notion-style side panel.',
        'Icons for each kind of record, search at the top, and pages that load ahead so moving around feels quick.',
        'Switch between spaces, on any screen size.',
        'Load sample data from the empty home page to look around.'
      ]
    },
    {
      id: '2026-10-23-first',
      title: 'Archive, first build',
      items: [
        'The basics: your own kinds of records, access for your AI assistants, and tools for developers.',
        'People, spaces and access keys, so only the right people see each record.',
        'Encrypted fields for sensitive details, and sign-in without passwords: email links, passkeys and single sign-on.',
        'Store large files with uploads that pick up where they left off, and search your records and files in one place.'
      ]
    }
  ];
</script>

<svelte:head>
  <title>Changelog — Archive</title>
  <meta name="description" content="What you can do in Archive now, newest first: boards and calendars, the web app, live pages, Office editing and AI that works on your records." />
  <meta name="theme-color" content="#f6f4ec" />
</svelte:head>

<Hero
  plain
  title="Changelog."
  lead="Every release of Archive, in plain words: what you and your agents can do now that you could not before."
  art="changelog"
  alt="A printing press stamping out dated release notes onto a long ribbon of paper"
/>

<Section>
  <Timeline entries={ENTRIES} />
</Section>

<Comp band>
  <TextBlock class="s-4 at-1 row-1">
    <h2>Coming <em>next</em>.</h2>
    <p>Designed and on the way, not shipped yet.</p>
  </TextBlock>
  <TextBlock class="s-5 at-7 row-1" art="files" title="Files" items={['A version for every save, to open or restore', 'Share links for people without an account', 'Search inside Office files']} />
  <TextBlock class="s-4 at-2 row-1 down-3" art="pages" title="Documents" items={['Formatting, nesting and tables in pages', 'Grow spreadsheets and decks, including new slides', 'Change formatting in Office files']} />
  <TextBlock class="s-5 at-5 row-2 down-1" art="connectors" title="Connectors" items={['Email, files, chat, docs, sales and finance tools', 'Your information comes in, the originals stay untouched, and everything stays in step', 'Arriving one service at a time']} />
</Comp>

<Cta title="Ship with us." lead="Free for your first workspace. No card, no sales call.">
  {#snippet actions()}
    <Button href={APP}>Start free</Button>
    <Button href="/docs" ghost>Read the docs</Button>
  {/snippet}
</Cta>
