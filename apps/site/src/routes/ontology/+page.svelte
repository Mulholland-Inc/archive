<script>
  import Cta from '#lib/Cta.svelte';
  import Facts from '#lib/Facts.svelte';
  import Faq from '#lib/Faq.svelte';
  import Hero from '#lib/Hero.svelte';
  import Meta from '#lib/Meta.svelte';
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
    ['What is an ontology?', 'A map of your business: the kinds of things you work with, like clients, properties and invoices, what you know about each, and how they connect.'],
    ['Who decides how it is set up?', 'You do. Your AI can suggest and prepare changes, but nothing changes until you confirm it.'],
    ['Can I take my data with me?', 'Yes. Your records sit in a standard, open database and your setup can be pulled as plain files. Copy all of it whenever you like, with no request to us.'],
    ['Can I move to my own cloud?', 'Yes. Archive is open source. Move everything to another provider, or run Archive in your own Google Cloud account, and it works the same.'],
    ['Am I tied to one AI?', 'No. ChatGPT, Claude, Gemini and Grok all work with the same map and the same rules, so you can switch whenever you want.'],
    ['Can I delete something by accident?', 'No. Removing a kind of thing takes a deliberate extra step, and a renamed detail keeps everything in it.']
  ];

  const KIT = [
    { art: 'search', title: 'Search everywhere', items: ['One search across everything you can see', 'Finds words as you type, ignoring case and accents', 'Every new kind of thing is searchable right away'] },
    { art: 'calendar', title: 'Board and calendar', items: ['Group records on a board, say by status', 'Place them on a calendar by date', 'Remove a board anytime; your records stay put'] },
    { art: 'code', title: 'Terraform, too', items: ['Your engineers can manage the ontology with Terraform', 'Only real changes show up, never false alarms', 'Access keys and permissions live right alongside it'] }
  ];
</script>

<Meta title="Ontology — Archive" description="You decide what your business runs on. Shape it together with your AI, keep it in the open, and take it with you whenever you want." />

<Hero
  title="You decide what your business runs on."
  lead="Your ontology is the map of your business: every kind of thing you work with and how it all connects. You shape it together with your AI, and it stays yours to keep, move or change."
  art="ontology"
  alt="A client building, a house, an invoice, a contract folder and an order, linked together on one base"
/>

<Section>
  <Row art="blog-change-the-shape-anytime" alt="A workbench where a structure is being reshaped">
    <h2>You shape it, with your AI.</h2>
    <p>Describe a change in plain words and your assistant prepares it, or make it yourself. You see exactly what will change, and nothing happens until you say yes.</p>
    <p>Add a new kind of thing, a new detail or a new link in minutes, without waiting on a vendor or a consultant.</p>
  </Row>
  <Row flip art="tables" alt="A grid of linked records rising out of a storage cylinder">
    <h2>Defined by you, connected your way.</h2>
    <p>Clients, properties, invoices and contracts each become a kind of thing with the details you care about. A property links to its owner, an invoice to its client, a contract to its files.</p>
    <p>It follows how your business works, not how some app decided it should.</p>
  </Row>
  <Row art="blog-own-your-data" alt="A platform with a database, a server and an antenna">
    <h2>Your data, in the open.</h2>
    <p>Everything sits in a standard, open database, never in a format only one app understands. Your whole setup can be pulled as plain files and kept with your own records.</p>
    <p>Any tool, team or assistant you choose can read it.</p>
  </Row>
  <Row flip art="blog-own-your-data-2" alt="A database being wheeled out through an open door">
    <h2>Take it anywhere, anytime.</h2>
    <p>Copy everything whenever you like, move it to another provider, or run Archive in your own Google Cloud account. It is open source, so you never need our permission.</p>
    <p>You stay because it works for you, not because leaving is hard.</p>
  </Row>
  <Row art="blog-company-memory-for-ai" alt="A central building of records connected to assistants around it">
    <h2>Your AI works for you.</h2>
    <p>ChatGPT, Claude, Gemini and Grok all follow the same map and the same rules you set. They never see more than you allow, and you can switch between them whenever you want.</p>
  </Row>
</Section>

<Section band title="Everything built in." body="The pieces you would otherwise build yourself, ready from day one.">
  <Specs items={KIT} />
</Section>

<Section title="Details that tidy themselves." body="Archive cleans up whatever comes in, so the same phone number or price looks the same wherever it came from.">
  <Facts head={['Detail', 'Keeps', 'You can type, for example']} rows={FIELDS} />
</Section>

<Section band title="Questions.">
  {#snippet body()}More in the <a class="link" href="/docs">docs</a>.{/snippet}
  <Faq still items={FAQ} />
</Section>

<Cta title="Run your business on your terms." lead="Free for your first workspace. No card, no sales call." />
