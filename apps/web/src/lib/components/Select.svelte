<script lang="ts">
  import Popover from './Popover.svelte';
  import Menu, { type MenuItem } from './Menu.svelte';
  import Button from './Button.svelte';
  import Icon from './Icon.svelte';
  let {
    icon,
    value = $bindable(),
    options,
    variant = 'tint',
    placeholder = 'Choose…',
    searchable,
    invalid = false,
    disabled = false,
    label,
    onchange
  }: {
    icon?: string;
    value: string;
    options: MenuItem[];
    variant?: 'tint' | 'secondary' | 'ghost' | 'field';
    placeholder?: string;
    searchable?: boolean;
    invalid?: boolean;
    disabled?: boolean;
    label?: string;
    onchange?: (value: string) => void;
  } = $props();
  let open = $state(false);
  const current = $derived(options.find((o) => o.id === value));
</script>

<Popover bind:open block={variant === 'field'}>
  {#snippet trigger({ toggle, open })}
    {#if variant === 'field'}
      <button
        type="button"
        class="field"
        data-invalid={invalid || undefined}
        data-open={open || undefined}
        data-empty={!current || undefined}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={label}
        {disabled}
        onclick={toggle}
      >
        {#if current?.icon ?? icon}<Icon name={(current?.icon ?? icon)!} />{/if}
        <span class="value">{current?.label ?? placeholder}</span>
        <span class="chev"><Icon name="chevron-down" size="sm" /></span>
      </button>
    {:else}
      <Button {variant} icon={icon ?? current?.icon} trailing="chevron-down" pressed={open} {disabled} onclick={toggle}>{current?.label ?? placeholder}</Button>
    {/if}
  {/snippet}
  {#snippet children({ close })}
    <Menu
      items={options}
      searchable={searchable ?? options.length > 8}
      selected={value ? [value] : []}
      onselect={(item) => {
        value = item.id;
        onchange?.(item.id);
        close();
      }}
    />
  {/snippet}
</Popover>

<style>
  .field {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    min-width: 0;
    height: var(--field-height);
    padding: 0 var(--space-2-5) 0 var(--space-3);
    border-radius: var(--field-radius);
    background: var(--field-background);
    box-shadow: inset 0 0 0 var(--border-width) var(--field-border);
    text-align: start;
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }
  .field:hover:not(:disabled) {
    box-shadow: inset 0 0 0 var(--border-width) var(--field-border-hover);
  }
  .field[data-open] {
    box-shadow:
      inset 0 0 0 var(--border-width) var(--ring),
      0 0 0 calc(var(--focus-width) + var(--space-0-5)) var(--ring-soft);
  }
  .field:disabled {
    opacity: var(--opacity-disabled);
    cursor: default;
  }
  [data-invalid] {
    box-shadow: inset 0 0 0 var(--border-width) var(--danger);
  }
  .value {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  [data-empty] .value {
    color: var(--subtle-foreground);
  }
  .chev {
    display: grid;
    color: var(--muted-foreground);
    transition: transform var(--duration-normal) var(--ease-spring);
  }
  [data-open] .chev {
    transform: rotate(180deg);
  }
</style>
