<script>
  import Cta from '#lib/Cta.svelte';
  import Facts from '#lib/Facts.svelte';
  import Faq from '#lib/Faq.svelte';
  import Hero from '#lib/Hero.svelte';
  import Row from '#lib/Row.svelte';
  import Section from '#lib/Section.svelte';
  import Specs from '#lib/Specs.svelte';

  const FIELDS = [
    ['Email', 'An email address, tidied up', '`ann@Example.COM`'],
    ['Phone', 'A phone number in one standard format', '`(415) 555-0132`'],
    ['Money', 'An amount and its currency', '`$1,234.50`'],
    ['Address', 'Street, city, state, ZIP code and country', 'A mailing address'],
    ['Name', 'The full name and its parts', '`Ann Lee`'],
    ['Date and time', 'The exact moment, with its time zone', '`2026-10-27 09:30`'],
    ['Quantity', 'A measurement and the unit it came in', '`5 km`'],
    ['Location', 'Latitude and longitude', '`37.77, -122.42`'],
    ['Bank and tax numbers', 'Checked so a mistyped digit is caught', 'Spaces and dashes are fine'],
    ['Card, Social Security and passport numbers', 'Locked away and shown masked', 'Revealed only to people you allow'],
    ['Web addresses', 'Links and domains, tidied up', '`Example.com/`'],
    ['Book and product codes', 'ISBNs and barcodes in one standard form', 'Old and new ISBNs, and store barcodes'],
    ['Durations and repeats', 'How long something takes, and how often it repeats', 'Standard calendar formats'],
    ['Country, state and language', 'Standard codes everyone recognizes', '`US`, `US-CA`, `en-US`']
  ];

  const FAQ = [
    ['Do I need a database expert to change things?', 'No. Make your change, and Archive works out the steps and shows you the plan before anything happens.'],
    ['Can I delete a list by accident?', 'No. Deleting a list takes a deliberate extra step. Boards and calendars hold no records of their own, so removing one is always safe.'],
    ['If I rename a field, do I lose what is in it?', 'No. Mark it as a rename and everything in the field comes along, as part of the same change.'],
    ['Can my developers work with the data directly?', 'Yes. The same rules apply, so they see exactly what the app shows you and nothing more.'],
    ['Who can change the structure?', 'Admins of the workspace or of a space. Everyone else adds and edits records in the spaces they belong to.']
  ];

  const KIT = [
    { art: 'search', title: 'Search everywhere', items: ['One search across every list and file you can see', 'Finds words as you type, ignoring case and accents', 'Every new list is searchable the moment you make it'] },
    { art: 'calendar', title: 'Board and calendar', items: ['Group records on a board, say by status', 'Place them on a calendar by date', 'Remove a board anytime; your records stay put'] },
    { art: 'code', title: 'Terraform, too', items: ['Your engineers can manage the setup with Terraform', 'Only real changes show up, never false alarms', 'Access keys and permissions live right alongside it'] }
  ];
</script>

<svelte:head>
  <title>Tables — Archive</title>
  <meta name="description" content="Your own records, shaped the way your business works. Change the structure anytime, see it as a board or a calendar, and let your AI assistants keep it tidy." />
  <meta name="theme-color" content="#0b0d12" />
</svelte:head>

<Hero
  title="Records shaped the way you work."
  lead="Keep track of houses, invoices, contracts or anything else, set up the way your business works. Your team and your AI assistants fill it in, and everything stays neat."
  art="tables"
  alt="An isometric spreadsheet grid rising out of a storage cylinder"
/>

<Section>
  <Row art="transaction" alt="Records sliding neatly into place, all at once">
    <h2>Every kind of thing gets its own list.</h2>
    <p>Houses, invoices and contracts each get their own list, and each entry is the thing itself.</p>
    <p>Your information stays in the open, ready to sort, search and connect, never locked inside one app.</p>
  </Row>
  <Row flip art="schema" alt="Blueprint sheets folding into a finished structure">
    <h2>Change the structure anytime.</h2>
    <p>Add a field, rename one or start a new list, and see exactly what will change before you confirm.</p>
    <p>Nothing gets deleted by accident, and a renamed field keeps everything in it.</p>
  </Row>
  <Row art="views" alt="One list seen as a board, a calendar and a grid">
    <h2>Boards, calendars and lists.</h2>
    <p>See the same records as a board, a calendar or a filtered list, whichever fits the job.</p>
    <p>Everyone sees only what they are allowed to, and every list and board gets an icon of its own.</p>
  </Row>
  <Row flip art="types" alt="Odd-shaped parcels coming out of a press as identical blocks">
    <h2>Types that clean up after people.</h2>
    <p>Type <code class="inline">(415) 555-0132</code> or <code class="inline">$1,234.50</code> however you like, and Archive saves a clean phone number and an amount with its currency.</p>
    <p>Link a record to others, like a house to its owner, and everything stays connected.</p>
  </Row>
</Section>

<Section band title="Everything a list needs." body="The pieces you would otherwise build yourself, ready from day one.">
  <Specs items={KIT} />
</Section>

<Section title="Fields that tidy themselves." body="Archive cleans up whatever comes in, so the same phone number or price looks the same wherever it came from.">
  <Facts head={['Field', 'Keeps', 'You can type, for example']} rows={FIELDS} />
</Section>

<Section band title="Questions.">
  {#snippet body()}More in the <a class="link" href="/docs">tables docs</a>.{/snippet}
  <Faq still items={FAQ} />
</Section>

<Cta title="Set up your records in minutes." lead="Free for your first workspace. No card, no sales call." />
