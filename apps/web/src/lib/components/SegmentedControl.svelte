<script lang="ts">
  let {
    value = $bindable(),
    options,
    variant = 'toolbar',
    label
  }: {
    value: string;
    options: { value: string; label: string; icon?: string }[];
    variant?: 'toolbar' | 'plain';
    label: string;
  } = $props();
  import Icon from './Icon.svelte';

  let root: HTMLElement;
  let x = $state(0);
  let w = $state(0);
  let ready = $state(false);

  function measure() {
    const el = root?.querySelector<HTMLElement>(`[data-value="${CSS.escape(value)}"]`);
    if (!el) return;
    x = el.offsetLeft;
    w = el.offsetWidth;
  }

  $effect(() => {
    value;
    measure();
  });

  $effect(() => {
    const ro = new ResizeObserver(measure);
    ro.observe(root);
    const id = requestAnimationFrame(() => requestAnimationFrame(() => (ready = true)));
    return () => {
      ro.disconnect();
      cancelAnimationFrame(id);
    };
  });

  function onkey(e: KeyboardEvent) {
    const i = options.findIndex((o) => o.value === value);
    if (e.key === 'ArrowRight') value = options[(i + 1) % options.length].value;
    else if (e.key === 'ArrowLeft') value = options[(i - 1 + options.length) % options.length].value;
    else return;
    e.preventDefault();
    root.querySelector<HTMLElement>(`[data-value="${CSS.escape(value)}"]`)?.focus();
  }
</script>

<div class="segmented" data-variant={variant} role="tablist" aria-label={label} bind:this={root} onkeydown={onkey} tabindex="-1">
  <span class="thumb" data-ready={ready || undefined} style:--x={`${x}px`} style:--w={`${w}px`}></span>
  {#each options as option (option.value)}
    <button
      class="segment"
      role="tab"
      data-value={option.value}
      aria-selected={value === option.value}
      aria-label={option.icon && variant === 'toolbar' ? option.label : undefined}
      title={option.icon && variant === 'toolbar' ? option.label : undefined}
      tabindex={value === option.value ? 0 : -1}
      onclick={() => (value = option.value)}
    >
      {#if option.icon}<Icon name={option.icon} size={variant === 'toolbar' ? 'lg' : 'md'} />{/if}
      {#if !option.icon || variant !== 'toolbar'}<span>{option.label}</span>{/if}
    </button>
  {/each}
</div>

<style>
  .segmented {
    position: relative;
    display: inline-flex;
    flex: none;
    padding: var(--space-0-5);
    border-radius: var(--radius);
    background: var(--tint-2);
    outline: none;
    isolation: isolate;
  }
  .thumb {
    position: absolute;
    top: var(--space-0-5);
    bottom: var(--space-0-5);
    left: 0;
    width: var(--w);
    translate: var(--x) 0;
    border-radius: calc(var(--radius) - var(--space-0-5));
    background: var(--selected);
    box-shadow: 0 0 0 var(--border-width) var(--tint-1);
    z-index: -1;
  }
  .thumb[data-ready] {
    transition:
      translate var(--duration-slow) var(--ease-spring),
      width var(--duration-slow) var(--ease-spring);
  }
  .segment {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-1-5);
    height: calc(var(--control-height) - var(--space-1));
    min-width: calc(var(--control-height) + var(--space-3));
    padding: 0 var(--space-2);
    border-radius: calc(var(--radius) - var(--space-0-5));
    color: var(--muted-foreground);
    white-space: nowrap;
    transition: color var(--duration-normal) var(--ease-out);
  }
  .segment[aria-selected='true'] {
    color: var(--foreground);
  }
  .segment:active {
    scale: var(--press-scale);
  }
  [data-variant='toolbar'] {
    background: var(--tint-2);
  }
  :global([data-theme='dark']) [data-variant='toolbar'] .thumb {
    background: var(--gray-950);
  }
  :global([data-theme='dark']) [data-variant='toolbar'] .segment[aria-selected='true'] {
    color: var(--gray-100);
  }
  [data-variant='plain'] .segment {
    padding: 0 var(--space-3);
  }
</style>
