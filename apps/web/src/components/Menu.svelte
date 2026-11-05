<script>
  import { setContext } from 'svelte';
  import { ChevronDown } from '@lucide/svelte';
  import Icon from './Icon.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuPanel from './MenuPanel.svelte';
  import { uid } from '../lib/ids.js';
  import { show, hide, roving, focusables } from '../lib/floating.js';

  let {
    trigger,
    children,
    options,
    value = $bindable(),
    placeholder = 'Select',
    size = 'md',
    variant = 'field',
    full = false,
    disabled = false,
    id = uid('select'),
    end = false,
    width = 'md',
    label,
    onchange
  } = $props();

  const edge = 8;
  const panelId = uid('menu');
  const current = $derived(options?.find((o) => o.value === value));
  let panel = $state();
  let origin = $state();
  let open = $state(false);
  let up = false;

  const between = (n, lo, hi) => Math.max(lo, Math.min(n, hi));

  function place() {
    const s = panel.style;
    const el = origin.firstElementChild;
    const r = el.getBoundingClientRect();
    const items = panel.querySelector('.morph-items');
    s.transition = 'none';
    s.width = s.maxWidth = s.maxHeight = '';
    s.minWidth = options ? '0' : '';
    s.setProperty('--w', `${r.width}px`);
    s.setProperty('--head', '0px');
    delete panel.dataset.up;
    show(panel, el);

    let left, top, need;
    const w = () => Math.max(Math.ceil(panel.getBoundingClientRect().width), need);
    if (options) {
      const item = panel.querySelector('[aria-checked="true"]') ?? focusables(panel)[0];
      items.scrollTop = 0;
      s.maxHeight = `min(var(--menu-max-height), ${innerHeight - 2 * edge}px)`;
      const p = panel.getBoundingClientRect();
      const box = item.getBoundingClientRect();
      const text = el.querySelector('.value').getBoundingClientRect();
      left = text.left - (item.querySelector('.text').getBoundingClientRect().left - p.left);
      need = r.right - left;
      const middle = r.top + r.height / 2;
      const offset = box.top - p.top + box.height / 2;
      top = between(middle - offset, edge, innerHeight - edge - p.height);
      items.scrollTop = offset - (middle - top);
    } else {
      need = r.width;
      const below = innerHeight - edge - r.top;
      const above = r.bottom - edge;
      up = panel.offsetHeight > below && above > below;
      if (up) panel.dataset.up = '';
      s.maxHeight = `min(var(--menu-max-height), ${up ? above : below}px)`;
      left = end ? r.right - w() : r.left;
      top = up ? r.bottom - panel.offsetHeight : r.top;
    }

    const width = w();
    left = between(left, edge, Math.max(edge, innerWidth - edge - width));
    const height = panel.offsetHeight;
    s.width = `${width}px`;
    s.maxWidth = 'none';
    s.left = `${left}px`;
    s.top = `${top}px`;
    s.setProperty('--head', `${r.left - left}px`);
    s.setProperty('--clip', `inset(${r.top - top}px ${left + width - r.right}px ${top + height - r.bottom}px ${r.left - left}px round var(--radius-lg))`);
    s.clipPath = 'var(--clip)';
    panel.offsetHeight;
    s.transition = s.clipPath = '';
  }

  function openCard(last) {
    place();
    requestAnimationFrame(() => {
      const list = focusables(panel);
      ((last ?? up) && !options ? list.at(-1) : list.find((el) => el.getAttribute('aria-checked') === 'true' && !el.dataset.toggle) ?? list[0])?.focus();
    });
  }

  function close() {
    hide(panel);
  }

  function pick(v) {
    panel.style.transition = 'none';
    value = v;
    onchange?.(v);
  }

  function triggerKey(event) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      openCard(event.key === 'ArrowUp');
    }
  }

  function ontoggle(event) {
    open = event.newState === 'open';
    if (!open) panel.style.transition = '';
  }

  function onkeydown(event) {
    roving(event, panel, { onclose: close });
  }

  $effect(() => {
    if (!open) return;
    const moved = (event) => {
      if (!panel.contains(event.target)) close();
    };
    addEventListener('scroll', moved, true);
    addEventListener('resize', close);
    return () => {
      removeEventListener('scroll', moved, true);
      removeEventListener('resize', close);
    };
  });

  setContext('menu', { close });

  const props = $derived({ onclick: () => (open ? close() : openCard()), onkeydown: triggerKey, 'aria-haspopup': 'menu', 'aria-expanded': open, 'aria-controls': panelId });
</script>

<div bind:this={origin} class="morph-origin" data-open={open || undefined}>
  {#if options}
    <button {id} type="button" class={['trigger', size, variant, { full }]} aria-label={label} {disabled} {...props}>
      {#if current?.icon}<Icon icon={current.icon} size="sm" />{/if}
      <span class={['value', { placeholder: !current }]}>{current?.label ?? placeholder}</span>
      <Icon icon={ChevronDown} size="sm" />
    </button>
  {:else}
    {@render trigger?.(props)}
  {/if}
</div>

<MenuPanel bind:panel id={panelId} {label} {width} data-morph {ontoggle} {onkeydown}>
  {#if !options}<div class="morph-head">{@render trigger?.(props)}</div>{/if}
  <div class="morph-items">
    {#if options}
      {#each options as option (option.value)}
        <MenuItem icon={option.icon} checked={option.value === value} onselect={() => pick(option.value)}>{option.label}</MenuItem>
      {/each}
    {:else}
      {@render children()}
    {/if}
  </div>
</MenuPanel>

<style>
  .trigger {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    flex: none;
    width: fit-content;
    height: var(--control-xl);
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    color: var(--color-text-tertiary);
    text-align: start;
    transition: box-shadow var(--duration-fast) var(--ease-out), background-color var(--duration-fast) var(--ease-out);
  }

  .trigger:hover:not(:disabled) {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .trigger:disabled {
    opacity: var(--opacity-disabled-field);
  }

  .sm {
    height: var(--control-md);
    padding: var(--space-2);
    border-radius: var(--radius-md);
  }

  .ghost {
    height: var(--control-md);
    padding: var(--space-2);
    border-radius: var(--radius-md);
    background: none;
    box-shadow: none;
    color: var(--color-text-secondary);
  }

  .ghost:hover:not(:disabled) {
    background: var(--color-fill);
    box-shadow: none;
    color: var(--color-text);
  }

  .full {
    display: flex;
    width: 100%;
  }

  .value {
    flex: 1;
    min-width: 0;
    color: var(--color-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .ghost .value {
    color: inherit;
    font-weight: var(--weight-medium);
  }

  .placeholder {
    color: var(--color-text-tertiary);
  }
</style>
