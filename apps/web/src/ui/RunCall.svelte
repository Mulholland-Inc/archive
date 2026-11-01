<script>
  import Icon from '#kit/Icon.svelte';
  import CodeBlock from './CodeBlock.svelte';
  import Output from './Output.svelte';

  let { calls } = $props();
  const json = (v) => (typeof v === 'string' ? v : JSON.stringify(v ?? null, null, 2));
  let open = $state(false);

  const busy = $derived(calls.some((c) => c.state !== 'done'));
  const failures = $derived(calls.filter((c) => c.state === 'done' && c.error).length);
  const failed = $derived(!busy && failures === calls.length);
  const browsing = $derived(calls.find((c) => c.state === 'running' && c.frame));
  const label = $derived.by(() => {
    if (browsing) return 'Browsing ' + (browsing.url || 'the web');
    if (calls.some((c) => c.state === 'running')) return 'Running code';
    if (busy) return 'Writing code';
    if (calls.length === 1) return failures ? 'Code failed' : 'Ran code';
    return `Ran code ${calls.length} times` + (failures ? `, ${failures} failed` : '');
  });
</script>

<div class="run" class:busy class:failed>
  <button class="row" type="button" aria-expanded={open} onclick={() => (open = !open)}>
    <span class="glyph"><Icon name={failed ? 'circle-alert' : 'code-xml'} /></span>
    <span class="label" role={busy ? 'status' : undefined}>{label}</span>
    <span class="chevron" class:open><Icon name="chevron-down" /></span>
  </button>
  {#if browsing}<img class="screen" src="data:image/jpeg;base64,{browsing.frame}" alt={label} />{/if}
  <div class="body" class:open inert={!open}>
    <div class="inner">
      {#each calls as call (call.key)}
        <div class="call">
          <CodeBlock code={call.code} live={call.state === 'writing'} />
          {#if call.logs?.length}<Output text={call.logs.join('\n')} tone="muted" />{/if}
          {#if call.error}<Output text={call.error} tone="danger" />{:else if call.state === 'done'}<Output text={json(call.result)} />{/if}
        </div>
      {/each}
    </div>
  </div>
</div>

<style>
  .run { display: flex; flex-direction: column; gap: 0; min-width: 0; transition: gap var(--duration-normal) var(--ease-out); animation: rise var(--duration-normal) var(--ease-out) both; }
  .row {
    display: flex;
    align-items: center;
    align-self: flex-start;
    gap: var(--space-2);
    color: var(--fg-muted);
    font-size: var(--text-chat);
    line-height: var(--leading-ui);
    transition: color var(--duration-fast);
  }
  .row:hover, .row:focus-visible { outline: 0; color: var(--fg); }
  .run:has(.body.open) { gap: var(--space-2); }
  .glyph { display: grid; place-items: center; }
  .busy .glyph { animation: pulse var(--duration-loop) var(--ease-in-out) infinite; }
  .busy .label {
    background: linear-gradient(90deg, var(--fg-muted) 30%, var(--fg) 50%, var(--fg-muted) 70%) 0 0 / 300% 100%;
    background-clip: text;
    -webkit-background-clip: text;
    color: transparent;
    animation: shimmer var(--duration-loop) linear infinite;
  }
  .failed .row { color: var(--danger); }
  .screen { width: 100%; max-width: 640px; aspect-ratio: 16 / 10; margin-top: var(--space-2); border-radius: var(--radius-panel); background: var(--fill); object-fit: cover; object-position: top; }
  .chevron { display: grid; place-items: center; transition: transform var(--duration-normal) var(--ease-out); }
  .chevron.open { transform: rotate(180deg); }
  .body { display: grid; grid-template-rows: 0fr; opacity: 0; transition: grid-template-rows var(--duration-normal) var(--ease-out), opacity var(--duration-normal) var(--ease-out); }
  .body.open { grid-template-rows: 1fr; opacity: 1; }
  .inner { display: flex; flex-direction: column; gap: var(--space-4); min-width: 0; min-height: 0; overflow: hidden; }
  .call { display: flex; flex-direction: column; gap: var(--space-2); min-width: 0; }
  @keyframes rise { from { opacity: 0; transform: translateY(var(--rise)); } }
  @keyframes pulse { 50% { opacity: var(--opacity-dim); } }
  @keyframes shimmer { from { background-position: 100% 0; } to { background-position: 0 0; } }
  @media (prefers-reduced-motion: reduce) {
    .run, .busy .glyph { animation: none; }
    .busy .label { animation: none; background: none; color: var(--fg-soft); }
  }
</style>
