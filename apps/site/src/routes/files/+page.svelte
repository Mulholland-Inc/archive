<script>
  import Cta from '#lib/Cta.svelte';
  import Facts from '#lib/Facts.svelte';
  import Faq from '#lib/Faq.svelte';
  import Hero from '#lib/Hero.svelte';
  import Row from '#lib/Row.svelte';
  import Section from '#lib/Section.svelte';
  import Specs from '#lib/Specs.svelte';

  const KINDS = [
    ['Word', 'Paragraphs, styles and lists, including those in tables and text boxes, and the text in every table cell', 'Replace text, add or remove paragraphs, fill in table cells'],
    ['Excel', 'Sheets, cells, formulas, merged cells and Excel tables', 'Set values and formulas, clear cells, add a line to a table'],
    ['PowerPoint', 'Slides and the text on them', 'Replace text anywhere on a slide'],
    ['Pages', 'Blocks, each with a style and its text', "Replace text, add or remove blocks, change a block's style"]
  ];

  const FAQ = [
    ['Will my formatting survive an edit?', 'Yes. New text picks up the formatting of the text it replaces, and the rest of the file stays exactly as it was.'],
    ['Do formulas recalculate?', 'Yes. Every formula is worked out fresh the moment the workbook is opened.'],
    ['What can my AI not change yet?', 'Adding or removing lines in a sheet, whole slides or shapes, and formatting such as bold or number styles. It can always see exactly what it is able to edit.'],
    ['How large can a file be?', 'Up to 64 MB in a single upload. Bigger files go up in pieces and resume if the connection drops.'],
    ['What happens when two assistants edit the same file?', 'They take turns, and each edit goes on top of the latest version, so nothing gets lost.']
  ];

  const MORE = [
    { art: 'tokens', title: 'Share links', items: ['Private links for downloads and previews that expire on their own', 'A day by default, or any time you choose', 'No sign-in needed to open one'] },
    { art: 'files', title: 'Photos', items: ['Size, camera and time taken, saved automatically', 'Previews made on upload', 'Location removed on download when you ask'] },
    { art: 'search', title: 'Search', items: ['File names and the text inside, next to your records', 'Results show the matching words', 'Filter by kind of file, space and date'] }
  ];
</script>

<svelte:head>
  <title>Files — Archive</title>
  <meta name="description" content="Upload anything and it is ready to search and preview right away. Your AI assistants edit Word, Excel and PowerPoint files and leave everything else exactly as it was." />
  <meta name="theme-color" content="#0b0d12" />
</svelte:head>

<Hero
  title="Files your AI can edit for you."
  lead="Upload anything and it is ready to search and preview right away. Your AI assistant edits Word, Excel and PowerPoint files and leaves everything else exactly as it was."
  art="files"
  alt="A file cabinet with drawers of documents, photos and spreadsheets"
/>

<Section>
  <Row art="scale" alt="A conveyor moving parcels of different sizes into a single intake">
    <h2>Ready the moment it lands.</h2>
    <p>Drop in a small file or a huge one, and big uploads pick up where they left off if the connection drops.</p>
    <p>Its details, its text and a preview of every photo are ready as soon as the upload finishes, with no waiting.</p>
  </Row>
  <Row flip art="security" alt="A vault whose drawers are labeled by fingerprints">
    <h2>Kept once, seen by the right people.</h2>
    <p>Upload the same file twice and Archive keeps just one copy, along with its name, size and text.</p>
    <p>Every file belongs to a space, so only the right people see it, wherever they look.</p>
  </Row>
  <Row art="office" alt="A document, a spreadsheet and a slide on a workbench, one tile being swapped">
    <h2>Office files, edited in place.</h2>
    <p>When your AI assistant changes a Word, Excel or PowerPoint file, it touches only the part it was asked to.</p>
    <p>Styles, images, comments and charts all survive, because nothing else is rebuilt.</p>
  </Row>
  <Row flip art="transaction" alt="Robots lining up at a single turnstile in front of a spreadsheet">
    <h2>Assistants take turns.</h2>
    <p>When two assistants edit the same workbook, they line up and each change goes in after the last, so nothing gets lost.</p>
    <p>Your assistant can also start a fresh workbook, document or slide deck from scratch.</p>
  </Row>
</Section>

<Section band title="What your AI can change." body="What an assistant can read, and what it can change, in each kind of file.">
  <Facts head={['File', 'It can read', 'It can change']} rows={KINDS} />
</Section>

<Section title="Sharing, photos and search.">
  <Specs items={MORE} />
</Section>

<Section band title="Questions.">
  {#snippet body()}More in the <a class="link" href="/docs">files docs</a>.{/snippet}
  <Faq still items={FAQ} />
</Section>

<Cta
  scene="factory"
  alt="A factory turning envelopes, sheets and folders into neat stacks stored in a silo"
  title="Bring your files. Keep every detail."
  lead="Free for your first workspace. No card, no sales call."
/>
