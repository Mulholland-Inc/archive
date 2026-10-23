<script lang="ts">
  import Icon from './Icon.svelte';
  let { icon, title, description, selected = false, onclick }: { icon: string; title: string; description: string; selected?: boolean; onclick: () => void } = $props();
</script>

<button type="button" class="choice" data-selected={selected || undefined} aria-pressed={selected} {onclick}>
  <span class="icon"><Icon name={icon} /></span>
  <span class="text">
    <span class="title">{title}</span>
    <span class="description">{description}</span>
  </span>
</button>

<style>
  .choice {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    width: 100%;
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    background: var(--card);
    box-shadow: inset 0 0 0 var(--border-width) var(--border);
    text-align: start;
    transition:
      box-shadow var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .choice:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--tint-4);
  }
  .choice:active {
    transform: scale(var(--press-scale-subtle));
  }
  [data-selected],
  [data-selected]:hover {
    box-shadow:
      inset 0 0 0 var(--border-width-strong) var(--ring),
      0 0 0 calc(var(--focus-width) + var(--space-0-5)) var(--ring-soft);
  }
  .icon {
    display: grid;
    padding-top: var(--space-0-5);
    color: var(--muted-foreground);
  }
  [data-selected] .icon {
    color: var(--accent);
  }
  .text {
    display: flex;
    flex-direction: column;
    gap: var(--space-0-5);
  }
  .title {
    font-weight: var(--font-medium);
  }
  .description {
    font-size: var(--text-sm);
    color: var(--muted-foreground);
  }
</style>
