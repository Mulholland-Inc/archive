<script>
  import { uid } from '../lib/ids.js';

  let { label, error, inline = false, children } = $props();
  const id = uid('field');
</script>

<div class={['field', { inline }]}>
  {#if label}<label for={id}>{label}</label>{/if}
  {@render children(id)}
  {#if error}<span class="error" role="alert">{error}</span>{/if}
</div>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-1-5);
    min-width: 0;
  }

  .inline {
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
  }

  label {
    color: var(--color-text);
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
  }

  .error {
    color: var(--color-danger);
    font-size: var(--text-sm);
  }
</style>
