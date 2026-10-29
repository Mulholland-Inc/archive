<script>
  import Cta from '#lib/Cta.svelte';
  import Facts from '#lib/Facts.svelte';
  import Faq from '#lib/Faq.svelte';
  import Hero from '#lib/Hero.svelte';
  import Meta from '#lib/Meta.svelte';
  import Plans from '#lib/Plans.svelte';
  import Section from '#lib/Section.svelte';
  import Specs from '#lib/Specs.svelte';
  import { APP } from '#lib/site.js';

  const PLANS = [
    {
      name: 'Free',
      price: '$0',
      per: 'forever',
      note: 'For you and your first AI assistants.',
      items: ['Up to 3 members', 'Your private space and 2 shared spaces', '2 GB of files', '2 connectors', 'Works with ChatGPT, Claude, Gemini and Grok', 'Community support'],
      cta: 'Start free',
      href: APP
    },
    {
      name: 'Pro',
      price: '$20',
      per: 'per member / month',
      note: 'For builders and small teams.',
      items: ['Everything in Free', 'Up to 20 members', 'Unlimited shared spaces', '100 GB of files per member', '10 connectors', 'A model trained to sort your records', 'Email support'],
      cta: 'Start with Pro',
      href: APP,
      pick: true
    },
    {
      name: 'Team',
      price: '$40',
      per: 'per member / month',
      note: 'For teams that run their work on Archive.',
      items: ['Everything in Pro', 'Unlimited members', '500 GB of files per member', 'Every connector', 'Models that sort and file your records', 'Priority support'],
      cta: 'Start with Team',
      href: APP
    },
    {
      name: 'Enterprise',
      price: 'Talk to us',
      note: 'For larger organizations.',
      items: ['Everything in Team', "Sign in through your company's own login", 'A database of your own', 'Custom storage and connector limits', 'Volume pricing for your AI models', 'Invoicing and a named engineer'],
      cta: 'Talk to us',
      href: 'mailto:hello@mulholland.ai',
      ghost: true
    }
  ];

  const COMPARE = [
    ['Members', '3', '20', 'Unlimited', 'Unlimited'],
    ['Shared spaces', '2', 'Unlimited', 'Unlimited', 'Unlimited'],
    ['File storage', '2 GB', '100 GB per member', '500 GB per member', 'Custom'],
    ['Connectors', '2', '10', 'All', 'All, custom limits'],
    ['Your own AI models', 'None', 'Sorting', 'Sorting and filing', 'Volume pricing'],
    ['AI assistants and developer tools', 'Yes', 'Yes', 'Yes', 'Yes'],
    ['Control over who sees and changes what', 'Yes', 'Yes', 'Yes', 'Yes'],
    ['Encrypted sensitive details', 'Yes', 'Yes', 'Yes', 'Yes'],
    ['Sign-in', 'Email link, passkeys, Google, Microsoft', 'Same', 'Same', "Plus your company's own login"],
    ['Database', 'Shared', 'Shared', 'Shared', 'Dedicated'],
    ['Support', 'Community', 'Email', 'Priority', 'Named engineer']
  ];

  const FAQ = [
    ['What counts as a member?', 'Anyone who has access to one of your spaces. AI assistants and their access keys are not members and cost nothing extra.'],
    ['Is the free plan a trial?', 'No. It has no time limit and needs no card. Upgrade when you need more members, storage or connectors.'],
    ['What happens when I reach a limit?', 'Nothing is deleted or locked. New uploads or connections wait until you upgrade or make room.'],
    ['Can I change plans at any time?', 'Yes. Upgrades apply at once and are prorated. Downgrades take effect at the end of the billing period.'],
    ['How is model training billed?', 'By the GPU-hour while a model trains, and by how much text it handles when your AI assistants use it.'],
    ['Can I take my data with me?', 'Yes, at any time. Export the structure of your data as plain files, and download every record and file.']
  ];

  const EVERY = [
    { art: 'spaces', title: 'Spaces and permissions', items: ['A private space for every member', 'Owner, admin, editor or viewer in each space', 'Only the right people see each record'] },
    { art: 'agents', title: 'Every AI assistant', items: ['ChatGPT, Claude, Gemini and Grok connect directly', 'A separate access key for each assistant', 'Changes are all-or-nothing, so nothing ends up half done'] },
    { art: 'training', title: 'Your records, your models', items: ['Turn your records into training examples', 'Train on paid plans, billed by use', 'Your model runs right next to your data'] }
  ];
</script>

<Meta title="Pricing — Archive" description="Start free with your first workspace. Pro and Team are priced per member, and training your own AI models is billed by use." />

<Hero
  title="Start free. Pay per member."
  lead="Your first workspace is free. Upgrade as your team grows; every plan works with your AI assistants and keeps your information with the right people."
  art="pricing"
  alt="A set of scales weighing a small stack of records against a few coins"
  links={[[APP, 'Start free'], ['#compare', 'Compare plans']]}
/>

<Section>
  <Plans plans={PLANS} />
</Section>

<Section band id="compare" title="Compare plans." body="Every plan gets the same protection and the same tools. The plans differ in size.">
  <Facts wide head={['', 'Free', 'Pro', 'Team', 'Enterprise']} rows={COMPARE} />
</Section>

<Section title="In every plan." body="The free plan is the real product. Nothing that keeps your data safe sits behind an upgrade.">
  <Specs items={EVERY} />
</Section>

<Section band title="Billing questions.">
  {#snippet body()}Anything else, write to <a class="link" href="mailto:hello@mulholland.ai">hello@mulholland.ai</a>.{/snippet}
  <Faq items={FAQ} />
</Section>

<Cta
  scene="city"
  alt="A city block whose buildings are spreadsheets, pages, files, a database and an antenna for AI assistants"
  title="Your first workspace is free."
  lead="No card, no sales call."
/>
