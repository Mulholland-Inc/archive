<script lang="ts">
  import Icon from './Icon.svelte';
  let {
    value = $bindable(''),
    type = 'text',
    placeholder,
    icon,
    invalid = false,
    disabled = false,
    autofocus = false,
    mono = false,
    id,
    name,
    autocomplete,
    label,
    onkeydown,
    oninput,
    onblur
  }: {
    value?: string;
    type?: 'text' | 'email' | 'password' | 'search' | 'number' | 'url' | 'date' | 'time';
    placeholder?: string;
    icon?: string;
    invalid?: boolean;
    disabled?: boolean;
    autofocus?: boolean;
    mono?: boolean;
    id?: string;
    name?: string;
    autocomplete?: HTMLInputElement['autocomplete'];
    label?: string;
    onkeydown?: (e: KeyboardEvent) => void;
    oninput?: (e: Event) => void;
    onblur?: (e: FocusEvent) => void;
  } = $props();
  let el: HTMLInputElement;
  $effect(() => {
    if (autofocus) el?.focus();
  });
</script>

<label class="input" data-invalid={invalid || undefined} data-disabled={disabled || undefined} data-mono={mono || undefined}>
  {#if icon}<span class="icon"><Icon name={icon} /></span>{/if}
  <input
    bind:this={el}
    bind:value
    {type}
    {placeholder}
    {disabled}
    {id}
    {name}
    {autocomplete}
    aria-label={label}
    aria-invalid={invalid || undefined}
    {onkeydown}
    {oninput}
    {onblur}
  />
</label>

<style>
  .input {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    min-width: 0;
    height: var(--field-height);
    padding: 0 var(--space-3);
    border-radius: var(--field-radius);
    background: var(--field-background);
    box-shadow: inset 0 0 0 var(--border-width) var(--field-border);
    cursor: text;
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }
  .input:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--field-border-hover);
  }
  .input:focus-within {
    box-shadow:
      inset 0 0 0 var(--border-width) var(--ring),
      0 0 0 calc(var(--focus-width) + var(--space-0-5)) var(--ring-soft);
  }
  [data-invalid],
  [data-invalid]:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--danger);
  }
  [data-disabled] {
    opacity: var(--opacity-disabled);
    cursor: default;
  }
  .icon {
    display: grid;
    color: var(--muted-foreground);
  }
  input {
    flex: 1;
    min-width: 0;
    height: 100%;
  }
  input::placeholder {
    color: var(--subtle-foreground);
  }
  [data-mono] input {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }
  input::-webkit-search-cancel-button {
    display: none;
  }
</style>
