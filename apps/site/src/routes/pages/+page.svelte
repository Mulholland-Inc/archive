<script>
  import Art from '#lib/Art.svelte';
  import Button from '#lib/Button.svelte';
  import Comp from '#lib/Comp.svelte';
  import Cta from '#lib/Cta.svelte';
  import Facts from '#lib/Facts.svelte';
  import Faq from '#lib/Faq.svelte';
  import Feature from '#lib/Feature.svelte';
  import Hero from '#lib/Hero.svelte';
  import Section from '#lib/Section.svelte';
  import TextBlock from '#lib/TextBlock.svelte';
  import { reveal } from '#lib/reveal.js';
  import { APP } from '#lib/site.js';

  const STEPS = [
    ['Type', 'Your change shows up on your screen at once and heads to Archive.'],
    ['Merge', "Archive weaves it in with everyone else's and saves the page."],
    ['Share', 'Everyone with the page open sees it a moment later, wherever they are.']
  ];

  const STYLES = [
    ['Paragraph', 'Body text, the default for a new block'],
    ['Headings, in three sizes', 'Titles and sections'],
    ['Bulleted and numbered', 'Lists'],
    ['To-do', 'Checklists your team and your AI work through'],
    ['Quote', 'Pulled quotes and callouts'],
    ['Code', 'Snippets, kept exactly as typed'],
    ['Your own', 'Any style your team or assistant uses, like `decision`']
  ];

  const FAQ = [
    ['Can my AI assistant edit while I am typing?', 'Yes. Your edits and its edits are woven together, and it never loses its place, even when you add or delete around it.'],
    ['What if two of us rewrite the same block?', "If you both replace the whole block, the last one saved wins and the words are never jumbled. Smaller edits inside a block keep everyone else's changes to the rest of it."],
    ['Does my AI assistant need anything special?', 'No. It edits pages the same way it edits Word, Excel and PowerPoint files, through one connection.'],
    ['Do pages support formatting and tables?', 'Today pages hold headings, lists, to-dos, quotes and code; bold text, nesting and tables are on the way. To keep track of records, use Tables.']
  ];
</script>

<svelte:head>
  <title>Pages — Archive</title>
  <meta name="description" content="Live documents your team and your AI assistants write together. Everyone edits the same page at once, and nobody's work gets overwritten." />
  <meta name="theme-color" content="#ece4fd" />
</svelte:head>

<Hero
  title="Pages your team and your AI write together."
  lead="Your team types in the browser while your AI assistant edits the same page. Every change lands, and nothing gets overwritten."
  art="pages"
  alt="An open document with blocks being placed by two hands and a small robot arm"
/>

<Comp>
  <h2 class="c-big s-8 at-1 row-1 reveal" use:reveal>Live for everyone.</h2>
  <TextBlock class="s-3 at-1 row-2 down-2">
    <p>Your words appear the instant you type them, then show up for everyone else on the page.</p>
    <p>You see who is here and where they are typing, and edits from different people never clash.</p>
  </TextBlock>
  <Art class="s-8 at-5 row-2 bleed-r" reveal name="crdt" alt="Several copies of one document passing small blocks around a ring" />
  <TextBlock class="s-4 at-8 row-3 center">
    <h2>Agents are editors too.</h2>
    <p>Your AI assistant can add, rewrite or remove any part of a page, even while others keep typing around it.</p>
    <p>It shows up in the list of who is here, and its changes appear on everyone's screen just like a teammate's.</p>
  </TextBlock>
  <Art class="s-6 at-1 row-3 down-1" reveal name="agents" alt="Chat windows connected to a document by thin cables" />
  <h2 class="c-big s-10 at-3 right down-1 reveal" use:reveal>Pages and Office files, one way.</h2>
  <Art class="s-6 at-2 tilt-r" reveal name="office" alt="A page, a spreadsheet and a slide deck plugged into one socket" />
  <TextBlock class="s-4 at-9 down-1">
    <p>The same simple steps that create, read and edit a page also work on Word, Excel and PowerPoint files.</p>
    <p>An assistant that can write a page can already fix a contract.</p>
  </TextBlock>
  <TextBlock class="s-3 at-1 end">
    <h2>Every word is searchable.</h2>
    <p>Every page can be found the moment it is saved, right next to your records and files.</p>
    <p>Each page belongs to one space, so only the right people see it.</p>
  </TextBlock>
  <Art class="s-8 at-5 bleed-r" reveal name="search" alt="A magnifier sweeping across stacked pages" />
</Comp>

<Feature steps={STEPS} art="api" alt="Messages flowing from one screen through a hub to many others">
  {#snippet title()}From your screen to <em>every</em> screen.{/snippet}
</Feature>

<Section title="Built from simple blocks." body="A page is a stack of blocks, like headings, lists and to-dos. Use the built-in styles or name your own.">
  <Facts head={['Style', 'For']} rows={STYLES} />
</Section>

<Comp band>
  <TextBlock spec class="s-7 at-6 row-1 rows-2 bleed-r" art="crdt" title="See who is here" items={['Everyone on the page, each with a name and color', 'Live cursors and highlights', 'People leave the list when they close the page']} />
  <TextBlock spec class="s-4 at-1 row-1 down-1" art="transaction" title="All or nothing" items={['Every change lands in full or not at all', 'If something goes wrong, nothing is saved', 'Everyone sees only finished changes']} />
  <TextBlock spec class="s-4 at-2 row-2 down-2" art="spaces" title="Spaces" items={['Every page belongs to one space', 'Viewers read, editors write', 'Your AI assistants follow the same rules as your team']} />
</Comp>

<Section title="Questions.">
  {#snippet body()}More in the <a class="link" href="/docs">pages docs</a>.{/snippet}
  <Faq still items={FAQ} />
</Section>

<Cta title="Write the first page together." lead="Invite your team, connect your AI assistant, and open the same page.">
  {#snippet actions()}
    <Button href={APP}>Start building free</Button>
    <Button href="/agents" ghost>How agents work</Button>
  {/snippet}
</Cta>
