<script>
  let { value = $bindable(''), busy = false, invalid = false, done } = $props();

  const LENGTH = 6;
  let field = $state();

  $effect(() => field?.focus());

  function input(event) {
    value = event.currentTarget.value.replace(/\D/g, '').slice(0, LENGTH);
    event.currentTarget.value = value;
    if (value.length === LENGTH) done();
  }
</script>

<div class={['code', { busy, invalid }]}>
  <input bind:this={field} {value} readonly={busy} oninput={input} aria-label="Code" aria-invalid={invalid || undefined} inputmode="numeric" autocomplete="one-time-code" />
  {#each { length: LENGTH }, i}
    <span class={['box', { filled: value[i], active: i === value.length }]} aria-hidden="true">{value[i] ?? ''}</span>
  {/each}
</div>

<style>
  .code {
    position: relative;
    display: flex;
    gap: var(--space-1-5);
    width: 100%;
  }

  input {
    position: absolute;
    inset: 0;
    z-index: 1;
    width: 100%;
    font-size: var(--text-input-min);
    opacity: 0;
    cursor: text;
  }

  .box {
    display: grid;
    flex: 1;
    place-items: center;
    min-width: 0;
    aspect-ratio: 1;
    border-radius: var(--radius-md);
    background: var(--color-fill-subtle);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    color: var(--color-text);
    font-size: var(--text-xl);
    font-weight: var(--weight-medium);
    transition:
      box-shadow var(--duration-fast) var(--ease-out),
      background-color var(--duration-fast) var(--ease-out),
      opacity var(--duration-fast) var(--ease-out);
  }

  .box:nth-of-type(3) {
    margin-inline-end: var(--space-3);
  }

  .filled {
    background: var(--color-surface-raised);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .code:focus-within .active {
    background: var(--color-surface-raised);
    box-shadow:
      inset 0 0 0 var(--border-width) var(--color-accent),
      0 0 0 calc(var(--focus-width) + var(--border-width)) var(--color-accent-subtle);
  }

  .code:focus-within .active::after {
    content: '';
    width: var(--focus-width);
    height: var(--text-xl);
    background: var(--color-accent);
    animation: blink var(--duration-spin-status) step-end infinite;
  }

  .busy .box {
    opacity: var(--opacity-disabled);
  }

  .invalid {
    animation: shake var(--duration-slow) var(--ease-out);
  }

  .invalid .box {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-danger);
  }

  @keyframes blink {
    50% {
      opacity: 0;
    }
  }

  @keyframes shake {
    25% {
      transform: translateX(calc(var(--space-1) * -1));
    }

    75% {
      transform: translateX(var(--space-1));
    }
  }
</style>
