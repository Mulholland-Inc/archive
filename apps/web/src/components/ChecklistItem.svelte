<script>
  import Checkbox from './Checkbox.svelte';
  import Text from './Text.svelte';

  // One step of a Checklist: a checkbox, its title and what it means, and the button that does it until
  // it is checked. A checked step is struck through.
  let { title, body, checked = false, onchange, action } = $props();
</script>

<li class={['step', { checked }]}>
  <Checkbox {checked} label={title} {onchange} />
  <div class="copy">
    <Text weight="medium">{title}</Text>
    <Text size="sm" tone="secondary">{body}</Text>
  </div>
  {#if !checked && action}{@render action()}{/if}
</li>

<style>
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
</style>
