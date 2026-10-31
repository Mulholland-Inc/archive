<script>
  import Cta from '#lib/Cta.svelte';
  import Hero from '#lib/Hero.svelte';
  import Meta from '#lib/Meta.svelte';
  import Section from '#lib/Section.svelte';
  import Specs from '#lib/Specs.svelte';
  import { reveal } from '#lib/reveal.js';
  import { GITHUB } from '#lib/site.js';

  const FAMILY = 'https://archive.family';

  const SCREENS = [
    {
      shot: 'tree',
      alt: 'A family tree four generations deep, centered on one person, with photos on some of the people',
      title: 'A family tree, from a file or a chat.',
      text: ['Bring in the tree you already have, from any genealogy app, and every person, couple, event and place becomes a record that points back to the line it came from.', 'Or just tell it who is who. Either way, it is the same tree.']
    },
    {
      shot: 'desk',
      alt: 'Suggested records from a story, each ticked or waiting, each quoting the words it came from',
      title: 'A story becomes records.',
      text: ['Write “my grandmother kept bees behind the house” and it suggests what that says: a person, a place, a habit, each quoting the words it came from.', 'Nothing goes in until someone ticks it. Where it isn’t sure who you mean, it asks.']
    },
    {
      shot: 'photos',
      alt: 'A grid of old family photographs, newest first',
      title: 'Photos, kept and found.',
      text: ['Every photo is kept as it was, and found by who is in it, where and when.', 'Each one starts private to whoever added it, until they share it with the family.']
    },
    {
      shot: 'chat-tree',
      alt: 'A chat where a family tree file has been brought in, with a line saying how many people, families and events it held',
      title: 'Ask it about anyone.',
      text: ['The family’s chat answers from the archive itself, and every answer shows where it came from.', 'It only ever sees what the person asking is allowed to.']
    }
  ];

  const KEEPS = [
    { img: 'family-tree', caption: 'A family tree drawn by hand, Connecticut, around 1800', credit: 'National Archives, NAID 12059080' },
    { img: 'marriage-certificate', caption: 'A marriage certificate filled in by hand, New Jersey, 1855', credit: 'Library of Congress, LC-DIG-pga-05615' },
    { img: 'steamship-ticket', caption: 'A steamship ticket, Rotterdam to New York, 1911', credit: 'National Archives, NAID 5753067' },
    { img: 'radio-1920s', caption: 'A family around the radio, 1920s', credit: 'Library of Congress, LC-DIG-hec-45134' },
    { img: 'land-deed', caption: 'A land deed and its wax seal, Greene County, Georgia, 1797', credit: 'Library of Congress, LC-USF34-044309-D' },
    { img: 'pupil-letter', caption: 'A letter of thanks from schoolchildren, Montenegro, 1921', credit: 'Library of Congress, LC-DIG-anrc-14705' }
  ];

  const PARTS = [
    { art: 'types', title: 'Its own kinds of record', items: ['People, couples, parents, events and places', 'Stories and the sources behind each claim', 'Twelve kinds of document, from a bill to a keepsake'] },
    { art: 'security', title: 'Private when it matters', items: ['Keep a file to the parents, or any group', 'Family records stay closed until an approver opens them', 'Sharing is always on purpose'] },
    { art: 'calendar', title: 'Dates that matter', items: ['Everything due, expiring or renewing in one calendar', 'A reminder by email or text, if you want one', 'A private calendar address for each member'] }
  ];
</script>

<Meta title="Archive Family — Built on Archive" description="Archive Family keeps a family's papers, photos, stories and family tree in one private place. It is a module on the Archive engine: its own kinds of record, the same files, search and assistants." />

<Hero
  title="Archive Family"
  lead="A private archive for a family's papers, photos, stories and family tree, and its bills, receipts and warranties too. Built as a module on Archive."
  art="family"
  alt="A house on a base, with a mailbox holding a letter, a photo album and a tree out front"
  links={[[FAMILY, 'Visit archive.family'], [GITHUB, 'See the engine']]}
/>

<Section>
  {#each SCREENS as s, i}
    <div class={['split', i % 2 && 'flip']}>
      <div class="split-copy reveal" use:reveal>
        <h2>{s.title}</h2>
        {#each s.text as p}<p>{p}</p>{/each}
      </div>
      <div class="split-art reveal" use:reveal><img class="shot" src="/family/screen-{s.shot}.webp" alt={s.alt} loading="lazy" /></div>
    </div>
  {/each}
  <p class="shot-note">Screens from Archive Family, with a made-up family.</p>
</Section>

<Section band title="What a family keeps." body="Trees drawn by hand, certificates, tickets, deeds, letters and photographs. Archive Family keeps the original, reads what it says, and files it with the people and places in it.">
  <div class="keeps">
    {#each KEEPS as k}
      <figure class="reveal" use:reveal>
        <img src="/family/{k.img}.webp" alt={k.caption} loading="lazy" />
        <figcaption>{k.caption}<small>{k.credit}</small></figcaption>
      </figure>
    {/each}
  </div>
</Section>

<Section title="One module, the whole engine." body="Archive Family adds its own kinds of record and a few screens. Files, search, groups, sharing and assistants all come from Archive, in one program.">
  <Specs items={PARTS} />
</Section>

<Cta
  scene="city"
  alt="A city block whose buildings are records, papers, files, a vault and an antenna for AI assistants"
  title="Build your own on Archive."
  lead="Start with your own kinds of record. Archive does the rest."
  links={[[FAMILY, 'Visit archive.family'], ['/developers', 'Build on Archive']]}
/>
