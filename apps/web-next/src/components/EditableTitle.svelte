<script>
  import Button from './Button.svelte';
  import InlineEdit from './InlineEdit.svelte';

  let { value, label, onsave, variant = 'title' } = $props();

  let editing = $state(false);
</script>

{#if editing}
  <InlineEdit {value} {label} variant={variant === 'heading' ? 'plain' : 'title'} onsave={(next) => { editing = false; onsave(next); }} oncancel={() => (editing = false)} />
{:else if variant === 'heading'}
  <button type="button" class="heading" onclick={() => (editing = true)}>{value}</button>
{:else}
  <Button variant="ghost" onclick={() => (editing = true)}>{value}</Button>
{/if}

<style>
  .heading {
    display: block;
    max-width: 100%;
    overflow: hidden;
    border-radius: var(--radius-xs);
    font: inherit;
    text-align: start;
    text-overflow: ellipsis;
    white-space: nowrap;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  .heading:hover {
    background: var(--color-fill);
  }
</style>
