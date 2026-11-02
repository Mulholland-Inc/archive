<script>
  import { PartyPopper } from '@lucide/svelte';
  import TopBar from '../components/TopBar.svelte';
  import Page from '../components/Page.svelte';
  import StickyHeader from '../components/StickyHeader.svelte';
  import Section from '../components/Section.svelte';
  import Checklist from '../components/Checklist.svelte';
  import ChecklistItem from '../components/ChecklistItem.svelte';
  import Button from '../components/Button.svelte';
  import ProgressCount from '../components/ProgressCount.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import InviteDialog from '../components/InviteDialog.svelte';
  import NewWorkflowDialog from '../components/NewWorkflowDialog.svelte';
  import MobileMenu from '../components/MobileMenu.svelte';
  import { app, onboardingSteps, stepDone, completeStep, toggleStep, openSettings, openArchiveSettings, createWorkflow } from '../lib/app.svelte.js';
  import { go } from '../lib/router.svelte.js';

  let inviting = $state(false);
  let creating = $state(false);

  const done = $derived(onboardingSteps.filter((s) => app.onboarding.includes(s.id)).length);
  const next = $derived(onboardingSteps.find((s) => !app.onboarding.includes(s.id)));
  const groups = [...new Set(onboardingSteps.map((s) => s.group))].map((g) => [g, onboardingSteps.filter((s) => s.group === g)]);

  const actions = {
    msa: () => completeStep('msa'),
    dpa: () => completeStep('dpa'),
    verify: () => completeStep('verify'),
    invite: () => (inviting = true),
    connect: () => openSettings('connectors'),
    upload: () => go('/library'),
    workflow: () => (creating = true),
    region: () => {
      openArchiveSettings('server');
      completeStep('region');
    }
  };

  function create(data) {
    go(createWorkflow(data));
  }
</script>

<TopBar mobile>{#snippet leading()}<MobileMenu />{/snippet}</TopBar>
<Page>
  <StickyHeader title="Onboarding" art="bg-access">
    {#snippet search()}
      <ProgressCount {done} total={onboardingSteps.length} label="Onboarding progress" />
    {/snippet}
  </StickyHeader>

  {#if !next}
    <EmptyState icon={PartyPopper} title="You're all set" size="sm" />
  {/if}

  {#each groups as [group, steps] (group)}
    <Section title={group}>
      <Checklist>
        {#each steps as step (step.id)}
          <ChecklistItem title={step.title} body={step.body} checked={stepDone(step.id)} onchange={() => toggleStep(step.id)}>
            {#snippet action()}<Button size="sm" variant={step === next ? 'primary' : 'secondary'} onclick={actions[step.id]}>{step.action}</Button>{/snippet}
          </ChecklistItem>
        {/each}
      </Checklist>
    </Section>
  {/each}
</Page>

<InviteDialog bind:open={inviting} />
<NewWorkflowDialog bind:open={creating} oncreate={create} />
