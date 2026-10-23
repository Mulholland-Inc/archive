<script lang="ts">
  import type { Snippet } from 'svelte';
  let {
    label,
    hint,
    error,
    required = false,
    id,
    children
  }: { label: string; hint?: string; error?: string; required?: boolean; id?: string; children: Snippet } = $props();
</script>

<div class="field" data-invalid={error ? true : undefined}>
  <label class="label" for={id}>{label}{#if required}<span class="required" aria-hidden="true">Required</span>{/if}</label>
  {@render children()}
  {#if error}
    <p class="error" role="alert">{error}</p>
  {:else if hint}
    <p class="hint">{hint}</p>
  {/if}
</div>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-1-5);
    min-width: 0;
  }
  .label {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-2);
    font-weight: var(--font-medium);
    user-select: none;
  }
  .required {
    font-weight: var(--font-normal);
    font-size: var(--text-sm);
    color: var(--subtle-foreground);
  }
  .hint,
  .error {
    font-size: var(--text-sm);
    color: var(--muted-foreground);
  }
  .error {
    color: var(--danger);
    animation: error-in var(--duration-normal) var(--ease-out);
  }
  @keyframes error-in {
    from {
      opacity: 0;
      transform: translateY(calc(var(--space-1) * -1));
    }
  }
</style>
