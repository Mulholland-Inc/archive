<script>
  import { Play, Pause, Square, Trash2 } from '@lucide/svelte';
  import IconButton from './IconButton.svelte';
  import Stack from './Stack.svelte';
  import { runWorkflow, pauseRun, stopRun } from '../lib/app.svelte.js';

  let { workflow, ondelete } = $props();

  const run = $derived(workflow.runs[0]);
  const live = $derived(run && ['running', 'waiting'].includes(run.status));

  const act = (fn) => (event) => {
    event.stopPropagation();
    fn();
  };
</script>

<Stack direction="row" gap="1" align="center" justify="end">
  {#if live}
    <IconButton icon={run.paused ? Play : Pause} label={`${run.paused ? 'Resume' : 'Pause'} ${workflow.name}`} size="sm" onclick={act(() => pauseRun(run))} />
    <IconButton icon={Square} label={`Stop ${workflow.name}`} size="sm" onclick={act(() => stopRun(run))} />
  {:else}
    <IconButton icon={Play} label={`Run ${workflow.name}`} size="sm" onclick={act(() => runWorkflow(workflow.id))} />
  {/if}
  <IconButton icon={Trash2} label={`Delete ${workflow.name}`} size="sm" onclick={act(ondelete)} />
</Stack>
