<script>
  import Cta from '#lib/Cta.svelte';
  import Facts from '#lib/Facts.svelte';
  import Faq from '#lib/Faq.svelte';
  import Hero from '#lib/Hero.svelte';
  import Letters from '#lib/Letters.svelte';
  import Meta from '#lib/Meta.svelte';
  import Row from '#lib/Row.svelte';
  import Scene from '#lib/Scene.svelte';
  import Section from '#lib/Section.svelte';
  import { START } from '#lib/site.js';

  const STEPS = [
    ['File', 'Filing is teaching. Where something lands says what it is, and what you write down says what it means.'],
    ['Train', 'Train a sorter that knows what something is, or a filer that fills in the details, on your examples alone, and check it on ones it has never seen.'],
    ['Use', 'Your assistants call on it whenever they need it, at full strength.']
  ];

  const KINDS = [
    ['Does', 'Knows what something is', 'Turns a document into filled-in records'],
    ['Reads', 'A file, a message or any text', 'Long documents, in one go'],
    ['Gives back', 'Where it belongs', 'Records with every field filled in, ready to save'],
    ['Learns from', 'What came in and where it was filed', 'What came in and what it became'],
    ['Tested on', 'Your own examples it never saw', 'Your own examples it never saw'],
    ['Quality', 'Full strength, never cut down', 'Full strength, never cut down'],
    ['Used by', 'Your assistants, whenever they need it', 'Your assistants, whenever they need it']
  ];

  const FAQ = [
    ['What data does a model learn from?', 'Only examples from your own records, and only ones you are allowed to see.'],
    ['Do I have to label anything by hand?', 'No. Filing something is the lesson, so your examples grow as people and assistants use Archive.'],
    ['How much can the filer read at once?', 'A long contract, a full mail thread or a whole report, in one go, without splitting it up first.'],
    ['Why full strength?', 'Cut-down models are cheaper to run but slip on the tricky cases, which is exactly where filing goes wrong. Every model runs at full strength.'],
    ['How do my assistants use a model?', 'They send it something new and get back what it is or what it says. Then they file the result as you, all or nothing.'],
    ['What if I change how my records are organized?', 'A filer fills in the fields it learned. After you add or change fields, gather fresh examples and train again.']
  ];
</script>

<Meta title="Models — Archive" description="Train AI on your own work. Everything you file is an example, so you can build a model that sorts and fills in records the way your business does, ready for your assistants." />

<Hero
  title="AI that knows your business."
  lead="Every document you file shows how your business works. Turn that into a model of your own, and let your assistants call on it anytime."
  art="models"
  alt="A model chip lifted out of a stack of filed records"
/>

<Section body="Your records already hold the answers. Archive turns them into a model your assistants can use.">
  {#snippet title()}From your work to a model, <em>in three steps</em>.{/snippet}
  <Scene name="lab" alt="A model foundry: silos of examples feed a training rig that stamps out model chips" />
</Section>

<Section band>
  <Row flip art="training" alt="A training rig with a progress gauge, fed by a conveyor of labeled cards">
    <h2>File, train, <em>use</em>.</h2>
    <Letters steps={STEPS} />
  </Row>
</Section>

<Section>
  <Row art="dataset" alt="Filed records stacked into a crate of examples">
    <h2>Your work is the training.</h2>
    <p>Each time a person or an assistant files something, that is an example the model can learn from.</p>
    <p>There is no separate labeling job to keep up with.</p>
  </Row>
  <Row flip art="classifier" alt="A sorting machine dropping documents into labeled bins">
    <h2>A sorter that knows what is what.</h2>
    <p>It reads something new and knows where it belongs: invoice, contract, resume, or anything else you keep.</p>
    <p>It learns your categories, not generic ones, so it sorts the way your team already does.</p>
  </Row>
  <Row art="schema" alt="A long scroll of text passing through a press and coming out as neat records">
    <h2>A filer that fills in the details.</h2>
    <p>It reads a long document and fills in your records field by field, just as your assistants would.</p>
    <p>A long contract, a full mail thread or a whole report goes in at once.</p>
  </Row>
  <Row flip art="endpoint" alt="A model answering requests from several assistants">
    <h2>Ready when your assistants are.</h2>
    <p>Your assistants send it something new, get the answer back and file it, all as you.</p>
    <p>It always runs at full strength, never cut down to save cost.</p>
  </Row>
</Section>

<Section band title="Two kinds of model." body="Both learn from your own records, and both are ready for your assistants to use.">
  <Facts head={['', 'Sorter', 'Filer']} rows={KINDS} />
</Section>

<Section title="Questions.">
  <Faq items={FAQ} />
</Section>

<Cta title="Train a model on your own work." lead="Start filing today, and your first examples start adding up." links={[START, ['/agents', 'How assistants work']]} />
