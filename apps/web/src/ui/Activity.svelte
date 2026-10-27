<script module>
  const thinking = ['Rummaging…', 'Pondering…', 'Cross-referencing…', 'Untangling…', 'Consulting the archive…', 'Doing sums…', 'Mulling it over…', 'Dusting off folders…', 'Connecting the dots…', 'Checking the shelves…', 'Sorting the mail…', 'Thinking it through…', 'Reading the fine print…', 'Leafing through notes…', 'Piecing it together…', 'Tidying up thoughts…', 'Looking under the sofa…', 'Asking the filing cabinet…', 'Weighing it up…', 'Making a cup of tea…'];
  const running = ['Running the numbers…', 'Opening drawers…', 'Flipping through receipts…', 'Pulling the files…', 'Counting the coins…', 'Checking the ledger…', 'Querying the archive…', 'Opening the shoebox…', 'Scanning the shelves…', 'Fetching the folders…', 'Adding it all up…', 'Digging through boxes…', 'Reading the meter…', 'Checking the calendar…', 'Sifting through papers…', 'Unfolding the paperwork…', 'Tallying the bills…', 'Searching the attic…', 'Looking it up…', 'Filing the answer…'];
  const pick = (list, not) => {
    const rest = list.filter((w) => w !== not);
    return rest[Math.floor(Math.random() * rest.length)];
  };
</script>

<script>
  import { untrack } from 'svelte';
  import Icon from './Icon.svelte';
  import ToolCall from './ToolCall.svelte';

  let { calls = [], phase = 'done' } = $props();
  let open = $state(false);
  let word = $state('');
  const failed = $derived(calls.filter((c) => c.result?.is_error).length);
  const summary = $derived((calls.length === 1 ? 'Ran code' : `Ran ${calls.length} queries`) + (failed ? `, ${failed} failed` : ''));

  $effect(() => {
    if (phase === 'done') return;
    const list = phase === 'running' ? running : thinking;
    untrack(() => (word = pick(list, word)));
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const id = setInterval(() => (word = pick(list, word)), 2200);
    return () => clearInterval(id);
  });
</script>

<div class="activity">
  {#if phase === 'done'}
    <button class="line" aria-expanded={open} onclick={() => (open = !open)}>
      <span class="summary">{summary}</span>
      <span class="chevron" class:open><Icon name="chevron-down" /></span>
    </button>
    {#if open}
      <div class="calls">
        {#each calls as c, i (i)}<ToolCall code={c.code} result={c.result} />{/each}
      </div>
    {/if}
  {:else}
    <div class="line busy" role="status">
      <span class="spinner"></span>
      <span class="shimmer">{word}</span>
    </div>
  {/if}
</div>

<style>
  .activity { display: flex; flex-direction: column; gap: var(--space-5); min-width: 0; }
  .line {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: calc(-1 * var(--space-2));
    padding: var(--space-2);
    border: 0;
    border-radius: var(--radius-md);
    background: none;
    color: var(--text-3);
    font-size: var(--text-sm);
    line-height: 1;
    text-align: left;
  }
  button.line { cursor: pointer; transition: background var(--dur) var(--ease), color var(--dur) var(--ease); }
  button.line:hover, button.line:focus-visible { outline: 0; background: var(--hover); color: var(--text-2); }
  .summary { flex: 1; }
  .chevron { display: flex; transition: transform var(--dur) var(--ease); }
  .chevron.open { transform: rotate(180deg); }
  .chevron :global(svg) { width: var(--text-md); height: var(--text-md); }
  .spinner {
    flex: none;
    width: var(--text-sm);
    height: var(--text-sm);
    border: var(--stroke-bold) solid var(--line-strong);
    border-top-color: var(--text);
    border-radius: 50%;
    animation: spin var(--spin) linear infinite;
  }
  .shimmer {
    background: linear-gradient(90deg, var(--text-3) 35%, var(--text) 50%, var(--text-3) 65%) 0 0 / 300% 100%;
    background-clip: text;
    -webkit-background-clip: text;
    color: transparent;
    animation: shimmer var(--shimmer) linear infinite;
  }
  .calls { display: flex; flex-direction: column; gap: var(--space-4); }
  @keyframes spin { to { transform: rotate(360deg); } }
  @keyframes shimmer { from { background-position: 100% 0; } to { background-position: 0 0; } }
  @media (prefers-reduced-motion: reduce) {
    .spinner { display: none; }
    .shimmer { background: none; color: var(--text-2); }
  }
</style>
