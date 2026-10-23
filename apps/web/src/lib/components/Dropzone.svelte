<script lang="ts">
  import Icon from './Icon.svelte';
  let { ondrop, label = 'Drop files to upload' }: { ondrop: (files: File[]) => void; label?: string } = $props();
  let depth = $state(0);
  const active = $derived(depth > 0);

  function hasFiles(e: DragEvent) {
    return [...(e.dataTransfer?.types ?? [])].includes('Files');
  }
</script>

<svelte:window
  ondragenter={(e) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    depth++;
  }}
  ondragleave={(e) => {
    if (!hasFiles(e)) return;
    depth = Math.max(0, depth - 1);
  }}
  ondragover={(e) => {
    if (hasFiles(e)) e.preventDefault();
  }}
  ondrop={(e) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    depth = 0;
    const files = [...(e.dataTransfer?.files ?? [])];
    if (files.length) ondrop(files);
  }}
/>

<div class="zone" data-active={active || undefined} aria-hidden={!active}>
  <div class="card">
    <Icon name="upload" size="xl" />
    <span>{label}</span>
  </div>
</div>

<style>
  .zone {
    position: fixed;
    inset: 0;
    z-index: var(--z-dialog);
    display: grid;
    place-items: center;
    background: var(--overlay);
    opacity: 0;
    visibility: hidden;
    pointer-events: none;
    transition:
      opacity var(--duration-normal) var(--ease-out),
      visibility var(--duration-normal);
  }
  [data-active] {
    opacity: 1;
    visibility: visible;
  }
  .card {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-6) var(--space-8);
    border-radius: var(--radius-xl);
    background: var(--popover);
    box-shadow: var(--shadow-lift);
    font-size: var(--text-md);
    font-weight: var(--font-medium);
    transform: scale(var(--pop-scale));
    transition: transform var(--duration-slow) var(--ease-spring);
  }
  [data-active] .card {
    transform: none;
  }
</style>
