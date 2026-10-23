<script lang="ts">
  import Icon from './Icon.svelte';
  let { checked = false, label, onchange }: { checked?: boolean; label?: string; onchange?: (v: boolean) => void } = $props();
</script>

<span
  class="checkbox"
  role="checkbox"
  aria-checked={checked}
  aria-label={label}
  tabindex={onchange ? 0 : -1}
  data-checked={checked || undefined}
  onclick={(e) => {
    if (!onchange) return;
    e.stopPropagation();
    onchange(!checked);
  }}
  onkeydown={(e) => {
    if (onchange && (e.key === ' ' || e.key === 'Enter')) {
      e.preventDefault();
      onchange(!checked);
    }
  }}
>
  <Icon name="check" size="sm" />
</span>

<style>
  .checkbox {
    flex: none;
    display: inline-grid;
    place-items: center;
    width: var(--checkbox-size);
    height: var(--checkbox-size);
    border-radius: var(--radius-xs);
    box-shadow: inset 0 0 0 var(--border-width-strong) var(--tint-4);
    background: var(--card);
    cursor: pointer;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      box-shadow var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .checkbox:active {
    transform: scale(var(--press-scale-strong));
  }
  [data-checked] {
    background: var(--accent);
    box-shadow: inset 0 0 0 var(--border-width-strong) var(--accent);
  }
  .checkbox :global(path) {
    stroke: var(--accent-foreground);
    stroke-width: 3;
    stroke-dasharray: 24;
    stroke-dashoffset: 24;
    transition: stroke-dashoffset var(--duration-normal) var(--ease-out);
  }
  [data-checked] :global(path) {
    stroke-dashoffset: 0;
    transition-delay: var(--duration-instant);
  }
</style>
