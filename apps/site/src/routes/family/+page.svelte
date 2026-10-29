<script>
  import Cta from '#lib/Cta.svelte';
  import Hero from '#lib/Hero.svelte';
  import Meta from '#lib/Meta.svelte';
  import Row from '#lib/Row.svelte';
  import Section from '#lib/Section.svelte';
  import Specs from '#lib/Specs.svelte';
  import { GITHUB } from '#lib/site.js';

  const FAMILY = 'https://archive.family';

  const PARTS = [
    { art: 'types', title: 'Its own kinds of record', items: ['People, couples, parents, events and places', 'Stories, pages and the sources behind each claim', 'Twelve kinds of document, from a bill to a keepsake'] },
    { art: 'security', title: 'Private by default', items: ['A member’s files start in their own space', 'Family records stay closed until an approver opens them', 'Sharing is always on purpose'] },
    { art: 'calendar', title: 'Dates that matter', items: ['Everything due, expiring or renewing in one calendar', 'A reminder by email or text, if you want one', 'A private calendar address for each member'] }
  ];
</script>

<Meta title="Archive Family — Built on Archive" description="Archive Family keeps a family's papers, photos and history in one private place. It is a module on the Archive engine: its own kinds of record, the same files, search and assistants." />

<Hero
  title="Archive Family"
  lead="A private archive for a family's bills, receipts and warranties, and its photos, letters and history. Built as a module on Archive."
  art="family"
  alt="A house on a base, with a mailbox holding a letter, a photo album and a tree out front"
  links={[[FAMILY, 'Visit archive.family'], [GITHUB, 'See the engine']]}
/>

<Section>
  <Row art="factory" alt="A factory turning envelopes, sheets and folders into neat stacks stored in a silo">
    <h2>Drop it in, find it filed.</h2>
    <p>Photos, scans, folders and whole ZIPs go in as they are. Archive reads each one, works out what it is, and files it as a bill, an agreement, a certificate or a keepsake.</p>
    <p>The original is always kept, and what it read waits for a person to check.</p>
  </Row>
  <Row flip art="schema" alt="A board of linked record cards">
    <h2>A family tree that cites its sources.</h2>
    <p>Bring in a family tree file and every person, couple, event and place becomes a record, each pointing back to the line it came from.</p>
    <p>Who a name on a document belongs to is only filled in when the archive is sure.</p>
  </Row>
  <Row art="agents" alt="Chat assistants plugged into one shared cabinet">
    <h2>Ask it anything.</h2>
    <p>The family's own chat, and any assistant they connect, answers from the archive itself, with a link to the original every time.</p>
    <p>It only ever sees what the person asking is allowed to see.</p>
  </Row>
</Section>

<Section band title="One module, the whole engine." body="Archive Family adds its own kinds of record and a few screens. Files, search, spaces, sharing and assistants all come from Archive, in one program.">
  <Specs items={PARTS} />
</Section>

<Cta
  scene="city"
  alt="A city block whose buildings are records, pages, files, a vault and an antenna for AI assistants"
  title="Build your own on Archive."
  lead="Start with your own kinds of record. Archive does the rest."
  links={[[FAMILY, 'Visit archive.family'], ['/developers', 'Build on Archive']]}
/>
