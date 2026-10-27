<script>
  let { value = $bindable(''), action, ...rest } = $props();
</script>

<div class="field">
  <input bind:value {...rest} />
  {#if action}<span class="action">{@render action()}</span>{/if}
</div>

<style>
  .field { position: relative; width: 100%; }
  input {
    width: 100%;
    height: var(--size-xl);
    padding: 0 var(--size-xl) 0 var(--space-5);
    background: var(--panel);
    border: var(--stroke) solid var(--line-strong);
    border-radius: var(--radius-pill);
    font-size: var(--text-lg);
    outline: 0;
    transition: border-color var(--dur) var(--ease);
  }
  input:focus { border-color: var(--line-focus); }
  input::placeholder { color: var(--text-2); }
  .action { position: absolute; top: 50%; right: var(--space-1); transform: translateY(-50%); }
  .action :global(button) { width: var(--size-md); height: var(--size-md); }
</style>
