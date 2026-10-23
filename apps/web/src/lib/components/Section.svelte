<script lang="ts">
  import type { Snippet } from 'svelte';
  let { title, description, children }: { title: string; description?: string; children: Snippet } = $props();
</script>

<section class="section" id={title.toLowerCase().replace(/\W+/g, '-')}>
  <header class="header">
    <h2 class="title">{title}</h2>
    {#if description}<p class="description">{description}</p>{/if}
  </header>
  {@render children()}
</section>

<style>
  .section {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    padding: var(--space-10) 0;
    border-top: var(--border-width) solid var(--border);
  }
  .header {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .title {
    font-size: var(--text-lg);
    font-weight: var(--font-semibold);
    letter-spacing: var(--tracking-snug);
  }
  .description {
    color: var(--muted-foreground);
    max-width: 40rem;
  }
</style>
