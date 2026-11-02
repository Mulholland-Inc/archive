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
  <span class="title"><Button variant="ghost" title={value} onclick={() => (editing = true)}>{value}</Button></span>
{/if}

<style>
  /* A long title ends in an ellipsis instead of pushing past the buttons beside it. */
  .title {
    display: flex;
    min-width: 0;
  }

  .title :global(.btn) {
    flex: 0 1 auto;
    min-width: 0;
    max-width: 100%;
  }

  .title :global(.label) {
    overflow: hidden;
    text-overflow: ellipsis;
  }

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
