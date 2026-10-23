<script lang="ts">
  import type { Snippet } from 'svelte';
  let {
    title,
    description,
    actions,
    flush = false,
    children
  }: { title?: string; description?: string; actions?: Snippet; flush?: boolean; children: Snippet } = $props();
</script>

<section class="card" data-flush={flush || undefined}>
  {#if title || actions}
    <header class="header">
      <div class="titles">
        {#if title}<h2 class="title">{title}</h2>{/if}
        {#if description}<p class="description">{description}</p>{/if}
      </div>
      {#if actions}<div class="actions">{@render actions()}</div>{/if}
    </header>
  {/if}
  <div class="body">{@render children()}</div>
</section>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    min-width: 0;
    padding: var(--space-5);
    border-radius: var(--radius-xl);
    background: var(--card);
    color: var(--card-foreground);
    box-shadow: inset 0 0 0 var(--border-width) var(--border);
  }
  .header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-3);
  }
  .titles {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    min-width: 0;
  }
  .title {
    font-size: var(--text-md);
    font-weight: var(--font-semibold);
    letter-spacing: var(--tracking-snug);
  }
  .description {
    color: var(--muted-foreground);
  }
  .actions {
    display: flex;
    gap: var(--space-2);
    margin: calc(var(--space-1) * -1) 0;
  }
  .body {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-width: 0;
  }
  [data-flush] .body {
    margin: 0 calc(var(--space-5) * -1) calc(var(--space-5) * -1);
  }
  @media (max-width: 40rem) {
    .card {
      padding: var(--space-4);
    }
    .header {
      flex-wrap: wrap;
    }
    [data-flush] .body {
      margin: 0 calc(var(--space-4) * -1) calc(var(--space-4) * -1);
    }
  }
</style>
