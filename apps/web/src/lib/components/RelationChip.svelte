<script lang="ts">
  import Icon from './Icon.svelte';
  let {
    glyph,
    label,
    count,
    target,
    menu = false,
    onclick
  }: {
    glyph: string;
    label: string;
    count?: number;
    target?: { glyph: string; id: string }[];
    menu?: boolean;
    onclick?: () => void;
  } = $props();
</script>

<button class="chip" {onclick} data-menu={menu || undefined}>
  <Icon name={glyph} size="sm" />
  <span>{label}</span>
  {#if count !== undefined}<span class="count">{count}</span>{/if}
  {#if target}
    {#each target as t, i (t.id + i)}
      {#if i > 0}<span class="amp">&amp;</span>{/if}
      <span class="target"><Icon name={t.glyph} size="sm" />{t.id}</span>
    {/each}
  {/if}
  {#if menu}<Icon name="chevron-down" size="sm" />{/if}
</button>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    flex: none;
    max-width: 100%;
    height: var(--chip-height);
    padding: 0 var(--space-2) 0 var(--space-1-5);
    border-radius: var(--chip-radius);
    background: var(--chip-background);
    font-size: var(--text-sm);
    white-space: nowrap;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .chip:hover {
    background: var(--tint-3);
  }
  .chip:active {
    transform: scale(var(--press-scale));
  }
  .count {
    color: var(--muted-foreground);
  }
  .target {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
  }
  .amp {
    color: var(--foreground);
  }
</style>
