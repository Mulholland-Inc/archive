<script>
  let { value = $bindable(''), action, ...rest } = $props();
</script>

<label class="field">
  <input bind:value {...rest} />
  {#if action}{@render action()}{/if}
</label>

<style>
  .field {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2);
    background: var(--panel);
    border: var(--stroke) solid var(--line-strong);
    border-radius: var(--radius-pill);
    transition: border-color var(--dur) var(--ease);
  }
  .field:focus-within { border-color: var(--line-focus); }
  input { flex: 1; min-width: 0; padding: var(--space-3); border: 0; background: none; font-size: var(--text-lg); line-height: 1; outline: 0; }
  input::placeholder { color: var(--text-2); }
  .field :global(button) { width: var(--size-md); height: var(--size-md); }
</style>
