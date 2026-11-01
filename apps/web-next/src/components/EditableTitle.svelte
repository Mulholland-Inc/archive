<script>
  import Button from './Button.svelte';
  import InlineEdit from './InlineEdit.svelte';

  let { value, label, onsave, variant = 'title' } = $props();

  let editing = $state(false);
</script>

{#if editing}
  <InlineEdit {value} {label} variant={variant === 'heading' ? 'heading' : 'title'} onsave={(next) => { editing = false; onsave(next); }} oncancel={() => (editing = false)} />
{:else if variant === 'heading'}
  <button type="button" class="heading" onclick={() => (editing = true)}>{value}</button>
{:else}
  <Button variant="ghost" onclick={() => (editing = true)}>{value}</Button>
{/if}

<style>
  .heading {
    display: block;
    margin: calc(var(--space-1) * -1) calc(var(--space-3) * -1);
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-md);
    font: inherit;
    text-align: start;
    overflow-wrap: anywhere;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .heading:hover {
    background: var(--color-fill);
  }
</style>
