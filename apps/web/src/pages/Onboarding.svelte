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
  import InviteDialog from '../components/InviteDialog.svelte';
  import NewWorkflowDialog from '../components/NewWorkflowDialog.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { app, onboardingSteps, stepDone, toggleStep, openSettings, createWorkflow } from '../lib/app.svelte.js';
  import { go } from '../lib/router.svelte.js';

  let inviting = $state(false);
  let creating = $state(false);

  const done = $derived(onboardingSteps.filter((s) => app.onboarding.includes(s.id)).length);
  const next = $derived(onboardingSteps.find((s) => !app.onboarding.includes(s.id)));
  const groups = [...new Set(onboardingSteps.map((s) => s.group))].map((g) => [g, onboardingSteps.filter((s) => s.group === g)]);

  const actions = {
    invite: () => (inviting = true),
    connect: () => openSettings('connectors'),
    upload: () => go('/library'),
    workflow: () => (creating = true)
  };

  const total = onboardingSteps.length;
  const progress = $derived(Math.round((done / total) * 20) / 20);

  function create(data) {
    go(createWorkflow(data));
  }
</script>

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
            <Checkbox {checked} label={step.title} onchange={() => toggleStep(step.id)} />
            <div class="copy">
              <Text weight="medium">{step.title}</Text>
              <Text size="sm" tone="secondary">{step.body}</Text>
            </div>
            {#if !checked}<Button size="sm" variant={step === next ? 'primary' : 'secondary'} onclick={actions[step.id]}>{step.action}</Button>{/if}
          </li>
        {/each}
      </ul>
    </Section>
  {/each}
</Page>

<InviteDialog bind:open={inviting} />
<NewWorkflowDialog bind:open={creating} oncreate={create} />

<style>
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
