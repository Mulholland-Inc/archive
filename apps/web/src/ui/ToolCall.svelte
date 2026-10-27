<script>
  import Icon from './Icon.svelte';

  let { code, result } = $props();
  let open = $state(false);
  const out = $derived.by(() => {
    if (result === undefined) return null;
    try {
      const r = JSON.parse(result.content);
      return { error: r.error, logs: r.logs ?? [], value: r.error ? '' : JSON.stringify(r.result, null, 2) };
    } catch {
      return { error: result.is_error ? result.content : '', logs: [], value: result.is_error ? '' : result.content };
    }
  });
  const state = $derived(!out ? 'Running' : out.error ? 'Failed' : 'Ran code');
</script>

<div class="tool" class:failed={out?.error}>
  <button class="head" aria-expanded={open} onclick={() => (open = !open)}>
    <Icon name={!out ? 'loader' : out.error ? 'circle-alert' : 'code'} />
    <span class="label" class:busy={!out}>{state}</span>
    <Icon name={open ? 'chevron-up' : 'chevron-down'} />
  </button>
  {#if open}
    <pre>{code}</pre>
    {#if out?.logs.length}<pre class="muted">{out.logs.join('\n')}</pre>{/if}
    {#if out?.error}<pre class="error">{out.error}</pre>{:else if out}<pre class="muted">{out.value}</pre>{/if}
  {/if}
</div>

<style>
  .tool { display: flex; flex-direction: column; gap: var(--space-2); align-self: flex-start; max-width: 100%; }
  .head {
    display: inline-flex;
    align-self: flex-start;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2);
    border: 0;
    border-radius: var(--radius-pill);
    background: var(--panel);
    color: var(--text-2);
    font-size: var(--text-sm);
    line-height: 1;
    cursor: pointer;
    transition: background var(--dur) var(--ease), color var(--dur) var(--ease);
  }
  .head:hover, .head:focus-visible { outline: 0; background: var(--raised); color: var(--text); }
  .head :global(svg) { width: var(--text-md); height: var(--text-md); }
  .failed .head { color: var(--alarm); }
  .busy { animation: pulse 1s infinite alternate; }
  @keyframes pulse { to { opacity: var(--dim); } }
  pre {
    margin: 0;
    padding: var(--space-4);
    max-height: var(--tile);
    overflow: auto;
    background: var(--panel);
    border-radius: var(--radius-lg);
    font: var(--text-xs)/var(--leading) var(--mono);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .muted { color: var(--text-2); }
  .error { color: var(--alarm); }
</style>
