<script>
  import Cta from '#lib/Cta.svelte';
  import Facts from '#lib/Facts.svelte';
  import Faq from '#lib/Faq.svelte';
  import Hero from '#lib/Hero.svelte';
  import Logos from '#lib/Logos.svelte';
  import Meta from '#lib/Meta.svelte';
  import Row from '#lib/Row.svelte';
  import Section from '#lib/Section.svelte';
  import Specs from '#lib/Specs.svelte';

  const LEVELS = [
    ['viewer', 'Browse, open and search records and files, read documents, and open boards, calendars and lists.'],
    ['editor', 'Everything a viewer can, plus add, change and remove records and files, and edit Office documents.'],
    ['admin', "Everything an editor can, plus add, change and remove members below owner, and change how the space's information is organized."],
    ['owner', 'Everything an admin can, plus add or remove owners and delete the space with everything in it. A space always keeps at least one owner.']
  ];

  const FAQ = [
    ['Can a technical user get around the rules?', 'No. The rules live at the core of Archive, so even someone working directly with the data only sees the spaces they belong to.'],
    ['What can an assistant see?', "Exactly what its access key allows. It acts as the person who connected it, narrowed to the key's spaces, access level and end date."],
    ['Can an assistant leave a half-finished change behind?', 'No. Every job is all or nothing, so if anything goes wrong, every change is undone.'],
    ['Can an assistant reach our servers or network?', 'No. The scripts it runs can call public web services, but never private or internal addresses, so they cannot reach the servers Archive runs on.'],
    ['Who can see a card number or a Social Security number?', 'Only people an admin has allowed, and only in spaces they can already see. Everyone else sees just the last few digits.'],
    ['How do I share a file outside Archive?', 'Make a share link. It lasts 24 hours unless you choose another length, and photos shared through it have their location data removed.']
  ];

  const SIDE = [
    { art: 'transaction', title: 'Assistants act as you', items: ['They see and change only what you could', 'If a job fails partway, nothing is saved', 'Only public web addresses, and a time limit on every job'] },
    { art: 'api', title: 'Access keys only narrow', items: ['A key never does more than the person who made it', 'Limit it to some spaces and a lower level', 'Give it an end date, or revoke it anytime'] },
    { art: 'files', title: 'Files leave carefully', items: ['Share links last 24 hours unless you choose otherwise', 'Photos shared by link lose their location data', 'Files follow the same rules as their space'] }
  ];
</script>

<Meta title="Security — Archive" description="Only the right people see your information. Private and shared spaces, sealed secret fields, and AI assistants that follow the same rules as everyone else." />

<Hero
  title="Only the right people see it."
  lead="Everything you store lives in a space, and only the people you let in can see it. The same rules hold for the app, your own tools and every AI assistant."
  art="security"
  alt="A vault door with records filing past a guard"
/>

<Section>
  <Row art="spaces" alt="Separate rooms, each holding its own records and files">
    <h2>Everything lives in a space.</h2>
    <p>Everyone gets a private space, and you can make shared ones for a team, a client or a household.</p>
    <p>Everything you add belongs to exactly one space, your own unless you pick another.</p>
  </Row>
  <Row flip art="tokens" alt="A ring of keys, each cut for a different door">
    <h2>Every key answers to a person.</h2>
    <p>Each access key belongs to one person and can only do less than they can: fewer spaces, less say, an end date.</p>
    <p>Lower someone's access and their keys follow right away.</p>
  </Row>
  <Row art="views" alt="One set of records seen through several windows, each showing different entries">
    <h2>Checked on every request.</h2>
    <p>The rules live at the core of Archive, not on a screen, so a bug in one place cannot show someone else's information.</p>
    <p>Every list, board, search and assistant sees exactly what you are allowed to see.</p>
  </Row>
  <Row flip art="types" alt="Typed fields on a form, one of them covered by a sealed envelope">
    <h2>Secret fields stay sealed.</h2>
    <p>Card numbers, Social Security numbers, national IDs and passports are encrypted before they are stored, and everyone sees only the last few digits.</p>
    <p>An admin decides who may see the real number.</p>
  </Row>
</Section>

<Section band title="Four levels of access." body="Each level includes everything below it. Access is set per space, so you can own one and only view another.">
  <Facts head={['Level', 'What they can do in the space']} rows={LEVELS} />
</Section>

<Section title="Sign in without a password." body="Use a passkey, or your Google or Microsoft account. However you sign in, you get the same access.">
  <Logos reveal items={['Passkeys', 'Google', 'Microsoft', 'Single sign-on']} />
</Section>

<Section band title="Safe to hand to an assistant." body="Assistants, connected tools and share links follow the same rules as people. Nothing gets a side door.">
  <Specs items={SIDE} />
</Section>

<Section title="Questions teams ask." body="The short answers. The docs have the rest.">
  <Faq items={FAQ} />
</Section>

<Cta title="Share the workspace, not the data." lead="Private by default, shared on purpose." />
