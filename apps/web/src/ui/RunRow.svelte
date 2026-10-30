<script>
  import Icon from '#kit/Icon.svelte';
  import Chip from './Chip.svelte';
  import Output from './Output.svelte';

  let { at, ms, label = '', error = '', result = null, logs = [] } = $props();
  let open = $state(false);
  const json = (v) => (typeof v === 'string' ? v : JSON.stringify(v ?? null, null, 2));
  const took = $derived(ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`);
</script>

<div class="run">
  <button class="row" type="button" aria-expanded={open} onclick={() => (open = !open)}>
    <span class="at">{at}</span>
    <span class="label">{label}</span>
    {#if error}<Chip>Failed</Chip>{/if}
    <span class="ms">{took}</span>
    <span class="chevron" class:open><Icon name="chevron-down" /></span>
  </button>
  <div class="body" class:open inert={!open}>
    <div class="inner">
      {#if logs?.length}<Output text={logs.join('\n')} tone="muted" />{/if}
      {#if error}<Output text={error} tone="danger" />{:else}<Output text={json(result)} />{/if}
    </div>
  </div>
</div>

<style>
  .run { display: flex; flex-direction: column; gap: 0; min-width: 0; transition: gap var(--duration-normal) var(--ease-out); }
  .run:has(.body.open) { gap: var(--space-2); }
  .row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--item-pad);
    border-radius: var(--radius-item);
    text-align: left;
    transition: background var(--duration-fast);
  }
  .row:hover, .row:focus-visible { outline: 0; background: var(--fill-faint); }
  .at { flex: none; font-weight: var(--weight-medium); }
  .label { flex: 1; min-width: 0; overflow: hidden; color: var(--fg-muted); text-overflow: ellipsis; white-space: nowrap; }
  .ms { flex: none; color: var(--fg-muted); font-size: var(--text-table); }
  .chevron { display: grid; place-items: center; color: var(--fg-muted); transition: transform var(--duration-normal) var(--ease-out); }
  .chevron.open { transform: rotate(180deg); }
  .body { display: grid; grid-template-rows: 0fr; opacity: 0; transition: grid-template-rows var(--duration-normal) var(--ease-out), opacity var(--duration-normal) var(--ease-out); }
  .body.open { grid-template-rows: 1fr; opacity: 1; }
  .inner { display: flex; flex-direction: column; gap: var(--space-2); min-width: 0; min-height: 0; overflow: hidden; }
</style>
