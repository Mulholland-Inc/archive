<script lang="ts">
  let { vertical = false, label }: { vertical?: boolean; label?: string } = $props();
</script>

{#if label}
  <div class="labelled" role="separator"><span>{label}</span></div>
{:else}
  <div class="divider" data-vertical={vertical || undefined} role="separator"></div>
{/if}

<style>
  .divider {
    flex: none;
    height: var(--border-width);
    background: var(--border);
  }
  [data-vertical] {
    width: var(--border-width);
    height: var(--icon-size);
    align-self: center;
  }
  .labelled {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    color: var(--muted-foreground);
    font-size: var(--text-sm);
  }
  .labelled::before,
  .labelled::after {
    content: '';
    flex: 1;
    height: var(--border-width);
    background: var(--border);
  }
</style>
