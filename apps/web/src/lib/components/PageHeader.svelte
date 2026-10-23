<script lang="ts">
  import type { Snippet } from 'svelte';
  let { title, description, meta, actions }: { title: string; description?: string; meta?: Snippet; actions?: Snippet } = $props();
</script>

<header class="header">
  <div class="titles">
    <h1 class="title">{title}</h1>
    {#if description}<p class="description">{description}</p>{/if}
    {#if meta}<div class="meta">{@render meta()}</div>{/if}
  </div>
  {#if actions}<div class="actions">{@render actions()}</div>{/if}
</header>

<style>
  .header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-4);
    flex-wrap: wrap;
  }
  .titles {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    min-width: 0;
  }
  .title {
    font-family: var(--font-display);
    font-size: var(--text-xl);
    font-weight: var(--font-semibold);
    letter-spacing: var(--tracking-tight);
    line-height: var(--leading-tight);
    overflow-wrap: anywhere;
  }
  .description {
    color: var(--muted-foreground);
    max-width: var(--form-width);
  }
  .meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-3);
    color: var(--muted-foreground);
    font-size: var(--text-sm);
  }
  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  @media (max-width: 40rem) {
    .title {
      font-size: var(--text-lg);
    }
    .actions {
      flex-wrap: wrap;
    }
  }
</style>
