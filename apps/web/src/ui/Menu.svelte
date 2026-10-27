<script>
  import Icon from './Icon.svelte';

  let { title, items, index = 0, compact = false, onpick, onhover } = $props();
</script>

<div class="menu" class:compact role="listbox">
  {#if title}<div class="title">{title}</div>{/if}
  {#each items as item, i (item.label)}
    <button
      role="option"
      aria-selected={i === index}
      class:on={i === index}
      tabindex="-1"
      onmousedown={(e) => e.preventDefault()}
      onmouseenter={() => onhover?.(i)}
      onclick={() => onpick(item)}
    >
      {#if item.icon}<span class="icon"><Icon name={item.icon} /></span>{/if}
      <span class="label">{item.label}</span>
      {#if item.hint}<span class="hint">{item.hint}</span>{/if}
      {#if item.checked || item.after}<span class="check"><Icon name={item.after ?? 'check'} /></span>{/if}
    </button>
  {/each}
</div>

<style>
  .menu {
    width: var(--menu);
    max-height: calc(var(--size-xl) * 6);
    overflow-y: auto;
    padding: var(--space-2);
    background: var(--raised);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow);
  }
  .compact { width: auto; min-width: var(--field); }
  .title { padding: var(--space-2); font-size: var(--text-xs); color: var(--text-3); }
  button {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    padding: var(--space-2);
    border: 0;
    border-radius: var(--radius-sm);
    background: none;
    text-align: left;
    cursor: pointer;
  }
  .on { background: var(--hover); }
  .icon {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--size-sm);
    height: var(--size-sm);
    border-radius: var(--radius-sm);
    background: var(--panel);
    color: var(--text-2);
  }
  .label { flex: 1; font-size: var(--text-sm); }
  .check { display: flex; color: var(--text-2); }
  .check :global(svg) { width: var(--text-md); height: var(--text-md); }
  .hint { font: var(--text-xs) var(--mono); color: var(--text-3); }
</style>
