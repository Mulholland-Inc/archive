<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  let { icon, title, description, children }: { icon: string; title: string; description?: string; children?: Snippet } = $props();
</script>

<div class="empty">
  <span class="icon"><Icon name={icon} size="xl" /></span>
  <div class="text">
    <p class="title">{title}</p>
    {#if description}<p class="description">{description}</p>{/if}
  </div>
  {#if children}<div class="actions">{@render children()}</div>{/if}
</div>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-4);
    max-width: var(--form-width);
    padding: var(--space-6) 0;
    animation: empty-in var(--duration-slow) var(--ease-out);
  }
  .icon {
    display: grid;
    color: var(--subtle-foreground);
  }
  .text {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .title {
    font-size: var(--text-md);
    font-weight: var(--font-medium);
  }
  .description {
    color: var(--muted-foreground);
  }
  .actions {
    display: flex;
    gap: var(--space-2);
  }
  @keyframes empty-in {
    from {
      opacity: 0;
      transform: translateY(var(--nudge));
    }
  }
</style>
