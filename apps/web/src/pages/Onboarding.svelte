<script>
  import { PartyPopper } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
  import Section from '../components/Section.svelte';
  import Checkbox from '../components/Checkbox.svelte';
  import Text from '../components/Text.svelte';
  import Button from '../components/Button.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import Dialog from '../components/Dialog.svelte';
  import Field from '../components/Field.svelte';
  import Input from '../components/Input.svelte';
  import Prose from '../components/Prose.svelte';
  import { app, onboardingSteps, stepDone, signed, atLeast, openPanel, closeFile, saveBusiness, signAgreement, startCard, cardLinked } from '../lib/app.svelte.js';
  import { notify } from '../lib/dialogs.svelte.js';
  import { markdown } from '../lib/live.js';

  let error = $state('');
  let busy = $state(false);

  let filing = $state(false);
  let form = $state({});

  let signing = $state(null);
  let company = $state('');
  let signer = $state('');
  let title = $state('');
  let binding = $state(false);

  let reading = $state(null);

  $effect(() => () => {
    if (app.canvas?.panel) closeFile();
  });

  let linking = $state(null);
  let stripe;
  let elements;

  const done = $derived(onboardingSteps.filter((s) => stepDone(s.id)).length);
  const next = $derived(onboardingSteps.find((s) => !stepDone(s.id)));
  const groups = [...new Set(onboardingSteps.map((s) => s.group))].map((g) => [g, onboardingSteps.filter((s) => s.group === g)]);
  const business = $derived(app.signup.business[0]);
  const owner = $derived(atLeast('owner'));

  const actions = {
    business: openBusiness,
    msa: () => openSigning('msa'),
    dpa: () => openSigning('dpa'),
    card: openCard
  };

  const ready = (id) => {
    if (!owner) return false;
    if (id === 'msa' || id === 'dpa') return Boolean(app.signup.terms[id]?.trim());
    return id === 'business' || Boolean(business);
  };

  const total = onboardingSteps.length;
  const progress = $derived(Math.round((done / total) * 20) / 20);

  function openBusiness() {
    const { name = '', ein = '', street = '', city = '', state = '', zip = '' } = business ?? {};
    form = { name, ein, street, city, state, zip };
    error = '';
    filing = true;
  }

  function openReading(kind) {
    reading = signed(kind);
    signing = null;
    openPanel(`read:${kind}`, reading.name, agreement);
  }

  function openSigning(kind) {
    reading = null;
    signing = kind;
    openPanel(`sign:${kind}`, onboardingSteps.find((s) => s.id === kind).title, agreement);
    company = business?.name ?? '';
    signer = '';
    title = '';
    binding = false;
    error = '';
  }

  async function attempt(work) {
    busy = true;
    error = '';
    try {
      await work();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  const fileBusiness = (close) => attempt(async () => {
    await saveBusiness(form);
    close();
  });

  const sign = () => attempt(async () => {
    await signAgreement({ kind: signing, company, signer, title });
    closeFile();
  });

  function stripeJs() {
    if (window.Stripe) return Promise.resolve();
    return new Promise((resolve, reject) => {
      const script = document.createElement('script');
      script.src = 'https://js.stripe.com/v3/';
      script.onload = resolve;
      script.onerror = () => reject(new Error('Stripe didn’t load'));
      document.head.append(script);
    });
  }

  async function openCard() {
    try {
      const started = await startCard();
      await stripeJs();
      error = '';
      linking = started;
    } catch (e) {
      await notify({ title: 'That didn’t work', body: e.message });
    }
  }

  function mountCard(node) {
    stripe = window.Stripe(linking.key);
    elements = stripe.elements({ clientSecret: linking.secret });
    const element = elements.create('payment');
    element.mount(node);
    return { destroy: () => element.destroy() };
  }

  const link = (close) => attempt(async () => {
    const { error: refused, setupIntent } = await stripe.confirmSetup({ elements, redirect: 'if_required', confirmParams: { return_url: location.href } });
    if (refused) throw new Error(refused.message);
    await cardLinked(setupIntent.id);
    close();
  });

  const when = (at) => new Date(at).toLocaleDateString(undefined, { dateStyle: 'long' });

  function detail(step) {
    if (step.id === 'business' && business) return business.name;
    if (step.id === 'card' && stepDone('card')) return app.signup.billing[0].name;
    const row = (step.id === 'msa' || step.id === 'dpa') && signed(step.id);
    return row ? `Signed by ${row.signer} on ${when(row.created_at)}` : step.body;
  }
</script>

{#snippet agreement()}
  {#if reading}
    <Prose blocks={markdown(reading.body)} />
    <div class="sign">
      <Text size="sm" tone="secondary">Signed by {reading.signer}, {reading.title}, for {reading.company} on {when(reading.created_at)}</Text>
    </div>
  {:else if signing}
    <Prose blocks={markdown(app.signup.terms[signing])} />
    <div class="sign">
      <Field label="Company">{#snippet children(id)}<Input {id} bind:value={company} />{/snippet}</Field>
      <Field label="Full name">{#snippet children(id)}<Input {id} bind:value={signer} />{/snippet}</Field>
      <Field label="Title">{#snippet children(id)}<Input {id} bind:value={title} />{/snippet}</Field>
      <label class="bind">
        <Checkbox bind:checked={binding} label="I can sign for this company" />
        <Text>I can sign for this company</Text>
      </label>
      {#if error}<Text size="sm" tone="danger" role="alert">{error}</Text>{/if}
      <div><Button variant="primary" disabled={busy || !binding || !company.trim() || !signer.trim() || !title.trim()} onclick={sign}>Sign</Button></div>
    </div>
  {/if}
{/snippet}

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page>
  <StickyHeader title="Onboarding" art="bg-access">
    {#snippet search()}
      <div class="count">
        <Text tone="secondary" tabular>{done} of {total} done</Text>
        <span class="bar" role="progressbar" aria-label="Onboarding progress" aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round((done / total) * 100)}>
          <span class="fill" style:--progress={progress}></span>
        </span>
      </div>
    {/snippet}
  </StickyHeader>

  {#if !next}
    <EmptyState icon={PartyPopper} title="You're all set" size="sm" />
  {/if}

  {#each groups as [group, steps] (group)}
    <Section title={group}>
      <ul class="checklist">
        {#each steps as step (step.id)}
          {@const checked = stepDone(step.id)}
          <li class={['step', { checked }]}>
            <Checkbox {checked} label={step.title} disabled />
            <div class="copy">
              <Text weight="medium">{step.title}</Text>
              <Text size="sm" tone="secondary">{detail(step)}</Text>
            </div>
            {#if !checked}
              <Button size="sm" variant={step === next ? 'primary' : 'secondary'} disabled={!ready(step.id)} onclick={actions[step.id]}>{step.action}</Button>
            {:else if step.id === 'msa' || step.id === 'dpa'}
              <Button size="sm" variant="secondary" onclick={() => openReading(step.id)}>View</Button>
            {:else}
              <Button size="sm" variant="secondary" disabled={!ready(step.id)} onclick={actions[step.id]}>{step.id === 'card' ? 'Change' : 'Edit'}</Button>
            {/if}
          </li>
        {/each}
      </ul>
    </Section>
  {/each}
</Page>

<Dialog bind:open={filing} title="Business details">
  <Field label="Legal name">{#snippet children(id)}<Input {id} bind:value={form.name} />{/snippet}</Field>
  <Field label="EIN">{#snippet children(id)}<Input {id} bind:value={form.ein} placeholder="12-3456789" />{/snippet}</Field>
  <Field label="Street">{#snippet children(id)}<Input {id} bind:value={form.street} />{/snippet}</Field>
  <Field label="City">{#snippet children(id)}<Input {id} bind:value={form.city} />{/snippet}</Field>
  <div class="pair">
    <Field label="State">{#snippet children(id)}<Input {id} bind:value={form.state} placeholder="NY" />{/snippet}</Field>
    <Field label="ZIP">{#snippet children(id)}<Input {id} bind:value={form.zip} placeholder="10001" />{/snippet}</Field>
  </div>
  {#if error}<Text size="sm" tone="danger" role="alert">{error}</Text>{/if}
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={busy} onclick={() => fileBusiness(close)}>Save</Button>
  {/snippet}
</Dialog>

<Dialog open={Boolean(linking)} onclose={() => (linking = null)} title="Link a card">
  {#if linking}
    <div use:mountCard></div>
    <Text size="sm" tone="secondary">You authorize us to charge this card under your signed agreement.</Text>
    {#if error}<Text size="sm" tone="danger" role="alert">{error}</Text>{/if}
  {/if}
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={busy} onclick={() => link(close)}>Link</Button>
  {/snippet}
</Dialog>

<style>
  .sign {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    margin-block-start: var(--space-6);
    padding-block-start: var(--space-6);
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }

  .pair {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-3);
  }

  .bind {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .checklist {
    display: flex;
    flex-direction: column;
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: var(--ring);
  }

  .checklist > :global(li + li) {
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }

  .step {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    min-height: var(--space-16);
    padding: var(--space-3) var(--space-4);
  }

  .copy {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: var(--space-0-5);
    min-width: 0;
  }

  .checked .copy > :global(:first-child) {
    color: var(--color-text-tertiary);
    text-decoration: line-through;
  }

  .checked .copy > :global(:last-child) {
    color: var(--color-text-tertiary);
  }

  @media (max-width: 40rem) {
    .step {
      flex-wrap: wrap;
      align-items: flex-start;
      gap: var(--space-3);
    }

    .step > :global(:first-child) {
      margin-block-start: var(--space-0-5);
    }

    .copy {
      flex-basis: calc(100% - var(--check-size) - var(--space-3));
    }

    .step > :global(.btn) {
      margin-inline-start: calc(var(--check-size) + var(--space-3));
    }
  }

  .count {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    max-width: var(--col-lg);
    white-space: nowrap;
  }

  .count > .bar {
    flex: 1;
  }

  .bar {
    display: block;
    width: 100%;
    height: var(--space-1);
    border-radius: var(--radius-full);
    background: var(--color-fill);
    overflow: hidden;
  }

  .fill {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--color-accent);
    transform: scaleX(var(--progress));
    transform-origin: left;
    transition: transform var(--duration-slower) var(--ease-out);
  }
</style>
