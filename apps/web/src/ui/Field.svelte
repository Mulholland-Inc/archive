<script>
  let { value = $bindable(''), action, ...rest } = $props();
</script>

<div class="field">
  <input bind:value {...rest} />
  {@render action?.()}
</div>

<style>
  .field {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-1);
    border-radius: var(--radius-full);
    background: var(--fill);
    transition: background var(--duration-fast);
  }
  .field:focus-within { background: var(--fill-strong); }
  input { flex: 1; min-width: 0; padding: var(--space-3); line-height: var(--leading-none); outline: 0; }
  input::placeholder { color: var(--fg-muted); }
</style>
