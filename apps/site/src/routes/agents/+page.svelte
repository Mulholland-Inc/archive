<script>
  import Cta from '#lib/Cta.svelte';
  import Facts from '#lib/Facts.svelte';
  import Faq from '#lib/Faq.svelte';
  import Hero from '#lib/Hero.svelte';
  import Logos from '#lib/Logos.svelte';
  import Row from '#lib/Row.svelte';
  import Section from '#lib/Section.svelte';
  import Specs from '#lib/Specs.svelte';

  const ACTIONS = [
    ['Ask', 'Records', 'Answer a precise question about your records, from only what you can see'],
    ['Browse', 'Records', 'A list of records or a saved board, narrowed to exact values'],
    ['Open', 'Records', 'One record'],
    ['Add', 'Records', 'Add a record, checked and tidied first, or start a blank page, Word document, spreadsheet or slide deck'],
    ['Change', 'Records', 'Change some fields of a record, or clear one'],
    ['Remove', 'Records', 'Remove a record'],
    ['Search', 'Records', 'Search all the records and files you can see, best match first'],
    ['Describe', 'Structure', 'Write out how your information is organized, one part at a time'],
    ['Reshape', 'Structure', 'Plan a change to how your information is organized, then make it'],
    ['Overview', 'Structure', 'Every kind of record and saved board, with its fields, notes and links'],
    ['Files', 'Files', 'The files you can see, with the text read out of them'],
    ['File', 'Files', 'One file, with the text read out of it'],
    ['Read', 'Documents', 'The layout of a Word document, spreadsheet, slide deck or page'],
    ['Edit', 'Documents', 'Make a list of edits to a document, all of them or none'],
    ['Spaces', 'Access', 'The spaces you can see, and what you may do in each']
  ];

  const FAQ = [
    ['Which AI apps can use Archive?', 'ChatGPT, Claude, Gemini and Grok, right from their connector lists. Coding tools like Claude Code and Cursor connect with an access key.'],
    ["Can my assistant see things it shouldn't?", 'No. It sees exactly what the person who connected it can see, and its key can only narrow that, never widen it.'],
    ['What if a job fails halfway?', 'Everything it changed is undone, so nothing ends up half done. The one exception is a change to how your information is organized, which sticks as soon as it succeeds.'],
    ['Can it reach other websites or apps?', 'Not while it works in Archive. Your assistant can look up what it needs first, then bring it along.'],
    ['How does my assistant know how my business is organized?', 'Archive tells it up front: every kind of record, its fields and the notes you wrote about them. When you change something, the next conversation sees it.']
  ];

  const ANYWHERE = [
    { art: 'cli', title: 'Command line', items: ['Run a saved set of steps from your terminal', 'Print every action, with your own records in it', 'Make a new access key for an assistant'] },
    { art: 'api', title: 'Your own apps', items: ['Send the same steps from anything you build', 'Every action is also available on its own', 'Each request carries a sign-in or an access key'] },
    { art: 'security', title: 'Guardrails', items: ['Sees only what its person can see', 'No reaching out to the internet or other files', 'Time and size limits on every job'] }
  ];
</script>

<svelte:head>
  <title>Agents — Archive</title>
  <meta name="description" content="Add Archive to ChatGPT, Claude, Gemini or Grok, and your AI assistant can find, update and file your company's information for you, as you, with nothing left half done." />
</svelte:head>

<Hero
  title="Every AI app. One connection."
  lead="Add Archive to ChatGPT, Claude, Gemini or Grok. Your assistant can then find, update and file your company's information for you."
  art="agents"
  alt="Chat windows from several AI apps plugged into one Archive antenna"
>
  <Logos items={['ChatGPT', 'Claude', 'Gemini', 'Grok', 'Claude Code', 'Cursor']} />
</Hero>

<Section>
  <Row art="code" alt="A short list of steps fed into a small engine">
    <h2>Big jobs in one go.</h2>
    <p>Your assistant writes out the steps and Archive carries them out right next to your information.</p>
    <p>Sorting a hundred files or updating a whole list takes one request, not a hundred back-and-forths.</p>
  </Row>
  <Row flip art="transaction" alt="A set of steps sealed together in one envelope">
    <h2>As you, all or nothing.</h2>
    <p>Your assistant sees and changes only what you could, nothing more.</p>
    <p>If anything goes wrong partway, every change is undone, so nothing ends up half done.</p>
  </Row>
  <Row art="schema" alt="A blueprint of your records turning into a guide for the assistant">
    <h2>It already knows your business.</h2>
    <p>Before it starts, your assistant sees how your information is organized: every field, what is required and how things link together.</p>
    <p>Add a field and the next conversation already knows about it.</p>
  </Row>
  <Row flip art="tokens" alt="A ring of keys, each cut to open fewer doors">
    <h2>A key for each assistant.</h2>
    <p>Every assistant gets its own access key, and it can only do less than you: fewer spaces, less say, an end date.</p>
    <p>Turn off one key and only that assistant stops.</p>
  </Row>
</Section>

<Section band body="What your assistant does in a chat works the same from your own apps and from the command line.">
  {#snippet title()}Works the same <em>everywhere</em>.{/snippet}
  <Specs items={ANYWHERE} />
</Section>

<Section title="Fifteen things it can do." body="Everything your assistant can do in Archive, in fifteen plain actions. If something is off, it hears exactly which field to fix.">
  <Facts head={['Action', 'Area', 'What it does']} rows={ACTIONS} />
</Section>

<Section band title="Questions.">
  <Faq items={FAQ} />
</Section>

<Cta title="Give your AI something real to work with." lead="Connect ChatGPT, Claude, Gemini or Grok to your first workspace for free." />
