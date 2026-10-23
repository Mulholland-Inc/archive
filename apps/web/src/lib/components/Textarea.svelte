<script lang="ts">
  let {
    value = $bindable(''),
    placeholder,
    rows = 4,
    invalid = false,
    mono = false,
    id,
    label
  }: { value?: string; placeholder?: string; rows?: number; invalid?: boolean; mono?: boolean; id?: string; label?: string } = $props();
</script>

<textarea class="textarea" bind:value {placeholder} {rows} {id} aria-label={label} data-invalid={invalid || undefined} data-mono={mono || undefined}></textarea>

<style>
  .textarea {
    display: block;
    width: 100%;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--field-radius);
    background: var(--field-background);
    box-shadow: inset 0 0 0 var(--border-width) var(--field-border);
    line-height: var(--leading-normal);
    resize: vertical;
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }
  .textarea:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--field-border-hover);
  }
  .textarea:focus {
    box-shadow:
      inset 0 0 0 var(--border-width) var(--ring),
      0 0 0 calc(var(--focus-width) + var(--space-0-5)) var(--ring-soft);
  }
  [data-invalid] {
    box-shadow: inset 0 0 0 var(--border-width) var(--danger);
  }
  [data-mono] {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }
  .textarea::placeholder {
    color: var(--subtle-foreground);
  }
</style>
