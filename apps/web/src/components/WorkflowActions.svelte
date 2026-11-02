<script>
  import { Play, LoaderCircle, Trash2 } from '@lucide/svelte';
  import IconButton from './IconButton.svelte';
  import Stack from './Stack.svelte';
  import { runWorkflow } from '../lib/app.svelte.js';

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
    <IconButton icon={LoaderCircle} label={`${workflow.name} is running`} size="sm" disabled />
  {:else}
    <IconButton icon={Play} label={`Run ${workflow.name}`} size="sm" onclick={act(() => runWorkflow(workflow.id))} />
  {/if}
  <IconButton icon={Trash2} label={`Delete ${workflow.name}`} size="sm" onclick={act(ondelete)} />
</Stack>
