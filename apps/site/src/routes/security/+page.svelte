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

  const ROLES = [
    ['viewer', 'Browse, open and search records and files, read documents, and open boards, calendars and lists.'],
    ['editor', 'Everything a viewer can, plus add, change and remove records and files, and edit Office documents.'],
    ['owner', 'Everything an editor can, plus add, change and remove people, change how information is organized, see everything whatever its group, and see the real secret numbers. There is always at least one owner.']
  ];

  const FAQ = [
    ['Can a technical user get around the rules?', 'No. The rules live at the core of Archive, so even someone working directly with the data only sees what their role and groups allow.'],
    ['What can an assistant see?', 'Exactly what the person who connected it can see. Its access key acts as them, and can be made read only or given an end date.'],
    ['Can an assistant leave a half-finished change behind?', 'No. Every job is all or nothing, so if anything goes wrong, every change is undone.'],
    ['Can an assistant reach our servers or network?', 'No. The scripts it runs can call public web services, but never private or internal addresses, so they cannot reach the servers Archive runs on.'],
    ['Who can see a card number or a Social Security number?', 'Only owners, and only when they ask for it. Everyone else sees just the last few digits.'],
    ['How do I share a file outside Archive?', 'Make a share link. It lasts 24 hours unless you choose another length, and photos shared through it have their location data removed.']
  ];

  const SIDE = [
    { art: 'transaction', title: 'Assistants act as you', items: ['They see and change only what you could', 'If a job fails partway, nothing is saved', 'Only public web addresses, and a time limit on every job'] },
    { art: 'api', title: 'Access keys act as you', items: ['A key never does more than the person who made it', 'Make it read only if it only needs to look', 'Give it an end date, or revoke it anytime'] },
    { art: 'files', title: 'Files leave carefully', items: ['Share links last 24 hours unless you choose otherwise', 'Photos shared by link lose their location data', 'Files follow the same rules as records'] }
  ];
</script>

<Meta title="Security — Archive" description="Only the right people see your information. Roles and groups, sealed secret fields, and AI assistants that follow the same rules as everyone else." />

<Hero
  title="Only the right people see it."
  lead="Everyone gets a role, and groups keep private things to the people who need them. The same rules hold for the app, your own tools and every AI assistant."
  art="security"
  alt="A vault door with records filing past a guard"
/>

<Section>
  <Row art="spaces" alt="Separate rooms, each holding its own records and files">
    <h2>Groups keep things close.</h2>
    <p>Put people in groups, like HR or the parents in a family, and keep a record or a file to one group.</p>
    <p>Everything else is shared with everyone in your workspace.</p>
  </Row>
  <Row flip art="tokens" alt="A ring of keys, each cut for a different door">
    <h2>Every key answers to a person.</h2>
    <p>Each access key belongs to one person and never does more than they can. Make it read only, or give it an end date.</p>
    <p>Change someone's role or groups and their keys follow right away.</p>
  </Row>
  <Row art="views" alt="One set of records seen through several windows, each showing different entries">
    <h2>Checked on every request.</h2>
    <p>The rules live at the core of Archive, not on a screen, so a bug in one place cannot show someone else's information.</p>
    <p>Every list, board, search and assistant sees exactly what you are allowed to see.</p>
  </Row>
  <Row flip art="types" alt="Typed fields on a form, one of them covered by a sealed envelope">
    <h2>Secret fields stay sealed.</h2>
    <p>Card numbers, Social Security numbers, national IDs and passports are encrypted before they are stored, and everyone sees only the last few digits.</p>
    <p>Only owners can see the real number.</p>
  </Row>
</Section>

<Section band title="Three roles." body="Each role includes everything below it. Everyone in a workspace has one.">
  <Facts head={['Role', 'What they can do']} rows={ROLES} />
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
